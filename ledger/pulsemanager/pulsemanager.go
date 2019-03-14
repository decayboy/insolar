/*
 *    Copyright 2019 Insolar Technologies
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package pulsemanager

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.opencensus.io/stats"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"

	"github.com/insolar/insolar"
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/core"
	"github.com/insolar/insolar/core/message"
	"github.com/insolar/insolar/core/reply"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/instrumentation/instracer"
	"github.com/insolar/insolar/ledger/artifactmanager"
	"github.com/insolar/insolar/ledger/heavyclient"
	"github.com/insolar/insolar/ledger/internal/jet"
	"github.com/insolar/insolar/ledger/recentstorage"
	"github.com/insolar/insolar/ledger/storage"
	"github.com/insolar/insolar/ledger/storage/drop"
	"github.com/insolar/insolar/ledger/storage/node"
	"github.com/insolar/insolar/ledger/storage/object"
)

//go:generate minimock -i github.com/insolar/insolar/ledger/pulsemanager.ActiveListSwapper -o ../../testutils -s _mock.go
type ActiveListSwapper interface {
	MoveSyncToActive(ctx context.Context) error
}

// PulseManager implements core.PulseManager.
type PulseManager struct {
	LR                         core.LogicRunner                `inject:""`
	Bus                        core.MessageBus                 `inject:""`
	NodeNet                    core.NodeNetwork                `inject:""`
	JetCoordinator             core.JetCoordinator             `inject:""`
	GIL                        core.GlobalInsolarLock          `inject:""`
	CryptographyService        core.CryptographyService        `inject:""`
	PlatformCryptographyScheme core.PlatformCryptographyScheme `inject:""`
	RecentStorageProvider      recentstorage.Provider          `inject:""`
	ActiveListSwapper          ActiveListSwapper               `inject:""`
	PulseStorage               pulseStoragePm                  `inject:""`
	HotDataWaiter              artifactmanager.HotDataWaiter   `inject:""`
	JetAccessor                jet.Accessor                    `inject:""`
	JetModifier                jet.Modifier                    `inject:""`

	ObjectStorage  storage.ObjectStorage  `inject:""`
	NodeSetter     node.Modifier          `inject:""`
	Nodes          node.Accessor          `inject:""`
	PulseTracker   storage.PulseTracker   `inject:""`
	ReplicaStorage storage.ReplicaStorage `inject:""`
	DBContext      storage.DBContext      `inject:""`
	StorageCleaner storage.Cleaner        `inject:""`

	DropModifier drop.Modifier `inject:""`
	DropAccessor drop.Accessor `inject:""`

	syncClientsPool *heavyclient.Pool

	currentPulse core.Pulse

	// setLock locks Set method call.
	setLock sync.RWMutex
	// saves PM stopping mode
	stopped bool

	// stores pulse manager options
	options pmOptions
}

type jetInfo struct {
	id       core.JetID
	mineNext bool
	left     *jetInfo
	right    *jetInfo
}

// TODO: @andreyromancev. 15.01.19. Just store ledger configuration in PM. This is not required.
type pmOptions struct {
	enableSync            bool
	splitThreshold        uint64
	dropHistorySize       int
	storeLightPulses      int
	heavySyncMessageLimit int
	lightChainLimit       int
}

// NewPulseManager creates PulseManager instance.
func NewPulseManager(conf configuration.Ledger) *PulseManager {
	pmconf := conf.PulseManager

	pm := &PulseManager{
		currentPulse: *core.GenesisPulse,
		options: pmOptions{
			enableSync:            pmconf.HeavySyncEnabled,
			splitThreshold:        pmconf.SplitThreshold,
			storeLightPulses:      conf.LightChainLimit,
			heavySyncMessageLimit: pmconf.HeavySyncMessageLimit,
			lightChainLimit:       conf.LightChainLimit,
		},
	}
	return pm
}

func (m *PulseManager) processEndPulse(
	ctx context.Context,
	jets []jetInfo,
	prevPulseNumber core.PulseNumber,
	currentPulse, newPulse core.Pulse,
) error {
	var g errgroup.Group
	ctx, span := instracer.StartSpan(ctx, "pulse.process_end")
	defer span.End()

	logger := inslogger.FromContext(ctx)
	for _, i := range jets {
		info := i

		g.Go(func() error {
			drop, dropSerialized, _, err := m.createDrop(ctx, core.RecordID(info.id), prevPulseNumber, currentPulse.PulseNumber)
			if err != nil {
				return errors.Wrapf(err, "create drop on pulse %v failed", currentPulse.PulseNumber)
			}

			sender := func(msg message.HotData, jetID core.JetID) {
				ctx, span := instracer.StartSpan(ctx, "pulse.send_hot")
				defer span.End()
				msg.Jet = *core.NewRecordRef(core.DomainID, core.RecordID(jetID))
				genericRep, err := m.Bus.Send(ctx, &msg, nil)
				if err != nil {
					logger.WithField("err", err).Error("failed to send hot data")
					return
				}
				if _, ok := genericRep.(*reply.OK); !ok {
					logger.WithField(
						"err",
						fmt.Sprintf("unexpected reply: %T", genericRep),
					).Error("failed to send hot data")
					return
				}
			}

			if info.left == nil && info.right == nil {
				msg, err := m.getExecutorHotData(
					ctx, core.RecordID(info.id), newPulse.PulseNumber, drop, dropSerialized,
				)
				if err != nil {
					return errors.Wrapf(err, "getExecutorData failed for jet id %v", info.id)
				}
				// No split happened.
				if !info.mineNext {
					go sender(*msg, info.id)
				}
			} else {
				msg, err := m.getExecutorHotData(
					ctx, core.RecordID(info.id), newPulse.PulseNumber, drop, dropSerialized,
				)
				if err != nil {
					return errors.Wrapf(err, "getExecutorData failed for jet id %v", info.id)
				}
				// Split happened.
				if !info.left.mineNext {
					go sender(*msg, info.left.id)
				}
				if !info.right.mineNext {
					go sender(*msg, info.right.id)
				}
			}

			m.RecentStorageProvider.RemovePendingStorage(ctx, core.RecordID(info.id))

			// FIXME: @andreyromancev. 09.01.2019. Temporary disabled validation. Uncomment when jet split works properly.
			// dropErr := m.processDrop(ctx, jetID, currentPulse, dropSerialized, messages)
			// if dropErr != nil {
			// 	return errors.Wrap(dropErr, "processDrop failed")
			// }

			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return errors.Wrap(err, "got error on jets sync")
	}

	return nil
}

func (m *PulseManager) createDrop(
	ctx context.Context,
	jetID core.RecordID,
	prevPulse, currentPulse core.PulseNumber,
) (
	block *drop.Drop,
	dropSerialized []byte,
	messages [][]byte,
	err error,
) {
	// TODO: 1.03.19 need to be replaced with smth. @egorikas
	// var prevDrop jet.Drop
	// prevDrop, err = m.DropAccessor.ForPulse(ctx, core.JetID(jetID), prevPulse)
	// if err == core.ErrNotFound {
	// 	prevDrop, err = m.DropAccessor.ForPulse(ctx, jet.JetParent(core.JetID(jetID)), prevPulse)
	// 	if err == core.ErrNotFound {
	// 		inslogger.FromContext(ctx).WithFields(map[string]interface{}{
	// 			"pulse": prevPulse,
	// 			"jet":   jetID.DebugString(),
	// 		}).Error("failed to find drop")
	// 		prevDrop = jet.Drop{Pulse: prevPulse}
	// 		err = m.DropModifier.Set(ctx, core.JetID(jetID), prevDrop)
	// 		if err != nil {
	// 			return nil, nil, nil, errors.Wrap(err, "failed to create empty drop")
	// 		}
	// 	} else if err != nil {
	// 		return nil, nil, nil, errors.Wrap(err, "[ createDrop ] failed to find parent")
	// 	}
	// } else if err != nil {
	// 	return nil, nil, nil, errors.Wrap(err, "[ createDrop ] Can't GetDrop")
	// }

	block = &drop.Drop{
		Pulse: currentPulse,
	}

	err = m.DropModifier.Set(ctx, core.JetID(jetID), *block)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "[ createDrop ] Can't SetDrop")
	}

	dropSerialized, err = drop.Encode(block)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "[ createDrop ] Can't Encode")
	}

	return
}

func (m *PulseManager) getExecutorHotData(
	ctx context.Context,
	jetID core.RecordID,
	pulse core.PulseNumber,
	drop *drop.Drop,
	dropSerialized []byte,
) (*message.HotData, error) {
	ctx, span := instracer.StartSpan(ctx, "pulse.prepare_hot_data")
	defer span.End()

	logger := inslogger.FromContext(ctx)
	indexStorage := m.RecentStorageProvider.GetIndexStorage(ctx, jetID)
	pendingStorage := m.RecentStorageProvider.GetPendingStorage(ctx, jetID)
	recentObjectsIds := indexStorage.GetObjects()

	recentObjects := map[core.RecordID]message.HotIndex{}
	pendingRequests := map[core.RecordID]recentstorage.PendingObjectContext{}

	for id, ttl := range recentObjectsIds {
		lifeline, err := m.ObjectStorage.GetObjectIndex(ctx, jetID, &id, false)
		if err != nil {
			logger.Error(err)
			continue
		}
		encoded := object.Encode(*lifeline)
		recentObjects[id] = message.HotIndex{
			TTL:   ttl,
			Index: encoded,
		}
	}

	requestCount := 0
	for objID, objContext := range pendingStorage.GetRequests() {
		if len(objContext.Requests) > 0 {
			pendingRequests[objID] = objContext
			requestCount += len(objContext.Requests)
		}
	}

	stats.Record(
		ctx,
		statHotObjectsSent.M(int64(len(recentObjects))),
		statPendingSent.M(int64(requestCount)),
	)

	msg := &message.HotData{
		Drop:            *drop,
		DropJet:         jetID,
		PulseNumber:     pulse,
		RecentObjects:   recentObjects,
		PendingRequests: pendingRequests,
	}
	return msg, nil
}

// TODO: @andreyromancev. 12.01.19. Remove when dynamic split is working.
var splitCount = 5

func (m *PulseManager) processJets(ctx context.Context, currentPulse, newPulse core.PulseNumber) ([]jetInfo, error) {
	ctx, span := instracer.StartSpan(ctx, "jets.process")
	defer span.End()

	m.JetModifier.Clone(ctx, currentPulse, newPulse)

	if m.NodeNet.GetOrigin().Role() != core.StaticRoleLightMaterial {
		return nil, nil
	}

	var results []jetInfo
	jetIDs := m.JetAccessor.All(ctx, newPulse)
	me := m.JetCoordinator.Me()
	logger := inslogger.FromContext(ctx).WithFields(map[string]interface{}{
		"current_pulse": currentPulse,
		"new_pulse":     newPulse,
	})
	indexToSplit := rand.Intn(len(jetIDs))
	for i, jetID := range jetIDs {
		wasExecutor := false
		executor, err := m.JetCoordinator.LightExecutorForJet(ctx, core.RecordID(jetID), currentPulse)
		if err != nil && err != node.ErrNoNodes {
			return nil, err
		}
		if err == nil {
			wasExecutor = *executor == me
		}

		logger = logger.WithField("jetid", jetID.DebugString())
		inslogger.SetLogger(ctx, logger)
		logger.WithField("i_was_executor", wasExecutor).Debug("process jet")
		if !wasExecutor {
			continue
		}

		info := jetInfo{id: jetID}
		if indexToSplit == i && splitCount > 0 {
			splitCount--

			leftJetID, rightJetID, err := m.JetModifier.Split(
				ctx,
				newPulse,
				jetID,
			)
			if err != nil {
				return nil, errors.Wrap(err, "failed to split jet tree")
			}

			// Set actual because we are the last executor for jet.
			m.JetModifier.Update(ctx, newPulse, true, leftJetID, rightJetID)

			info.left = &jetInfo{id: leftJetID}
			info.right = &jetInfo{id: rightJetID}
			nextLeftExecutor, err := m.JetCoordinator.LightExecutorForJet(ctx, core.RecordID(leftJetID), newPulse)
			if err != nil {
				return nil, err
			}
			if *nextLeftExecutor == me {
				info.left.mineNext = true
				err := m.rewriteHotData(ctx, core.RecordID(jetID), core.RecordID(leftJetID))
				if err != nil {
					return nil, err
				}
			}
			nextRightExecutor, err := m.JetCoordinator.LightExecutorForJet(ctx, core.RecordID(rightJetID), newPulse)
			if err != nil {
				return nil, err
			}
			if *nextRightExecutor == me {
				info.right.mineNext = true
				err := m.rewriteHotData(ctx, core.RecordID(jetID), core.RecordID(rightJetID))
				if err != nil {
					return nil, err
				}
			}

			logger.WithFields(map[string]interface{}{
				"left_child":  leftJetID.DebugString(),
				"right_child": rightJetID.DebugString(),
			}).Info("jet split performed")
		} else {
			// Set actual because we are the last executor for jet.
			m.JetModifier.Update(ctx, newPulse, true, jetID)
			nextExecutor, err := m.JetCoordinator.LightExecutorForJet(ctx, core.RecordID(jetID), newPulse)
			if err != nil {
				return nil, err
			}
			if *nextExecutor == me {
				info.mineNext = true
			}
		}
		results = append(results, info)
	}

	return results, nil
}

func (m *PulseManager) rewriteHotData(ctx context.Context, fromJetID, toJetID core.RecordID) error {
	indexStorage := m.RecentStorageProvider.GetIndexStorage(ctx, fromJetID)

	logger := inslogger.FromContext(ctx).WithFields(map[string]interface{}{
		"from_jet": fromJetID.DebugString(),
		"to_jet":   toJetID.DebugString(),
	})
	for id := range indexStorage.GetObjects() {
		idx, err := m.ObjectStorage.GetObjectIndex(ctx, fromJetID, &id, false)
		if err != nil {
			if err == core.ErrNotFound {
				logger.WithField("id", id.DebugString()).Error("rewrite index not found")
				continue
			}
			return errors.Wrap(err, "failed to rewrite index")
		}
		err = m.ObjectStorage.SetObjectIndex(ctx, toJetID, &id, idx)
		if err != nil {
			return errors.Wrap(err, "failed to rewrite index")
		}
	}

	m.RecentStorageProvider.CloneIndexStorage(ctx, fromJetID, toJetID)
	m.RecentStorageProvider.ClonePendingStorage(ctx, fromJetID, toJetID)

	return nil
}

// Set set's new pulse and closes current jet drop.
func (m *PulseManager) Set(ctx context.Context, newPulse core.Pulse, persist bool) error {
	m.setLock.Lock()
	defer m.setLock.Unlock()
	if m.stopped {
		return errors.New("can't call Set method on PulseManager after stop")
	}

	ctx, span := instracer.StartSpan(
		ctx, "pulse.process", trace.WithSampler(trace.AlwaysSample()),
	)
	span.AddAttributes(
		trace.Int64Attribute("pulse.PulseNumber", int64(newPulse.PulseNumber)),
	)
	defer span.End()

	jets, jetIndexesRemoved, oldPulse, prevPN, err := m.setUnderGilSection(ctx, newPulse, persist)
	if err != nil {
		return err
	}

	if !persist {
		return nil
	}

	// Run only on material executor.
	// execute only on material executor
	// TODO: do as much as possible async.
	if m.NodeNet.GetOrigin().Role() == core.StaticRoleLightMaterial && oldPulse != nil && prevPN != nil {
		err = m.processEndPulse(ctx, jets, *prevPN, *oldPulse, newPulse)
		if err != nil {
			return err
		}
		m.postProcessJets(ctx, newPulse, jets)
		m.addSync(ctx, jets, oldPulse.PulseNumber)
		go m.cleanLightData(ctx, newPulse, jetIndexesRemoved)
	}

	err = m.Bus.OnPulse(ctx, newPulse)
	if err != nil {
		inslogger.FromContext(ctx).Error(errors.Wrap(err, "MessageBus OnPulse() returns error"))
	}

	if m.NodeNet.GetOrigin().Role() == core.StaticRoleVirtual {
		err = m.LR.OnPulse(ctx, newPulse)
	}
	if err != nil {
		return err
	}

	return nil
}

func (m *PulseManager) setUnderGilSection(
	ctx context.Context, newPulse core.Pulse, persist bool,
) (
	[]jetInfo, map[core.RecordID][]core.RecordID, *core.Pulse, *core.PulseNumber, error,
) {
	var (
		oldPulse *core.Pulse
		prevPN   *core.PulseNumber
	)

	m.GIL.Acquire(ctx)
	ctx, span := instracer.StartSpan(ctx, "pulse.gil_locked")
	defer span.End()
	defer m.GIL.Release(ctx)

	m.PulseStorage.Lock()
	// FIXME: @andreyromancev. 17.12.18. return core.Pulse here.
	storagePulse, err := m.PulseTracker.GetLatestPulse(ctx)
	if err != nil && err != core.ErrNotFound {
		m.PulseStorage.Unlock()
		return nil, nil, nil, nil, errors.Wrap(err, "call of GetLatestPulseNumber failed")
	}

	if err != core.ErrNotFound {
		oldPulse = &storagePulse.Pulse
		prevPN = storagePulse.Prev
		ctx, _ = inslogger.WithField(ctx, "current_pulse", fmt.Sprintf("%d", oldPulse.PulseNumber))
	}

	logger := inslogger.FromContext(ctx)
	logger.WithFields(map[string]interface{}{
		"new_pulse": newPulse.PulseNumber,
		"persist":   persist,
	}).Debugf("received pulse")

	// swap pulse
	m.currentPulse = newPulse

	// swap active nodes
	err = m.ActiveListSwapper.MoveSyncToActive(ctx)
	if err != nil {
		return nil, nil, nil, nil, errors.Wrap(err, "failed to apply new active node list")
	}
	if persist {
		if err := m.PulseTracker.AddPulse(ctx, newPulse); err != nil {
			m.PulseStorage.Unlock()
			return nil, nil, nil, nil, errors.Wrap(err, "call of AddPulse failed")
		}
		fromNetwork := m.NodeNet.GetWorkingNodes()
		toSet := make([]insolar.Node, 0, len(fromNetwork))
		for _, node := range fromNetwork {
			toSet = append(toSet, insolar.Node{ID: node.ID(), Role: node.Role()})
		}
		err = m.NodeSetter.Set(newPulse.PulseNumber, toSet)
		if err != nil {
			m.PulseStorage.Unlock()
			return nil, nil, nil, nil, errors.Wrap(err, "call of SetActiveNodes failed")
		}
	}

	m.PulseStorage.Set(&newPulse)
	m.PulseStorage.Unlock()

	if m.NodeNet.GetOrigin().Role() == core.StaticRoleHeavyMaterial {
		return nil, nil, nil, nil, nil
	}

	var jets []jetInfo
	if persist && oldPulse != nil {
		jets, err = m.processJets(ctx, oldPulse.PulseNumber, newPulse.PulseNumber)
		// We just joined to network
		if err == node.ErrNoNodes {
			return jets, map[core.RecordID][]core.RecordID{}, oldPulse, prevPN, nil
		}
		if err != nil {
			return nil, nil, nil, nil, errors.Wrap(err, "failed to process jets")
		}
	}

	removed := map[core.RecordID][]core.RecordID{}
	if oldPulse != nil && prevPN != nil {
		removed = m.RecentStorageProvider.DecreaseIndexesTTL(ctx)
		if m.NodeNet.GetOrigin().Role() == core.StaticRoleLightMaterial {
			m.prepareArtifactManagerMessageHandlerForNextPulse(ctx, newPulse, jets)
		}
	}

	if persist && oldPulse != nil {
		nodes, err := m.Nodes.All(oldPulse.PulseNumber)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		// No active nodes for pulse. It means there was no processing (network start).
		if len(nodes) == 0 {
			// Activate zero jet for jet tree and unlock jet waiter.
			zeroJet := core.NewJetID(0, nil)
			m.JetModifier.Update(ctx, newPulse.PulseNumber, true, *zeroJet)
			err := m.HotDataWaiter.Unlock(ctx, core.RecordID(*zeroJet))
			if err != nil {
				if err == artifactmanager.ErrWaiterNotLocked {
					inslogger.FromContext(ctx).Error(err)
				} else {
					return nil, nil, nil, nil, errors.Wrap(err, "failed to unlock zero jet")
				}
			}
		}
	}

	return jets, removed, oldPulse, prevPN, nil
}

func (m *PulseManager) addSync(ctx context.Context, jets []jetInfo, pulse core.PulseNumber) {
	ctx, span := instracer.StartSpan(ctx, "pulse.add_sync")
	defer span.End()

	if !m.options.enableSync || m.NodeNet.GetOrigin().Role() != core.StaticRoleLightMaterial {
		return
	}

	for _, jInfo := range jets {
		m.syncClientsPool.AddPulsesToSyncClient(ctx, core.RecordID(jInfo.id), true, pulse)
	}
}

func (m *PulseManager) postProcessJets(ctx context.Context, newPulse core.Pulse, jets []jetInfo) {
	ctx, span := instracer.StartSpan(ctx, "jets.post_process")
	defer span.End()

	for _, jetInfo := range jets {
		if !jetInfo.mineNext {
			m.RecentStorageProvider.RemovePendingStorage(ctx, core.RecordID(jetInfo.id))
		}
	}
}

func (m *PulseManager) cleanLightData(ctx context.Context, newPulse core.Pulse, jetIndexesRemoved map[core.RecordID][]core.RecordID) {
	startSync := time.Now()
	inslog := inslogger.FromContext(ctx)
	ctx, span := instracer.StartSpan(ctx, "pulse.clean")
	defer func() {
		latency := time.Since(startSync)
		stats.Record(ctx, statCleanLatencyTotal.M(latency.Nanoseconds()/1e6))
		span.End()
		inslog.Infof("cleanLightData all time spend=%v", latency)
	}()

	delta := m.options.storeLightPulses

	p, err := m.PulseTracker.GetNthPrevPulse(ctx, uint(delta), newPulse.PulseNumber)
	if err != nil {
		inslogger.FromContext(ctx).Errorf("Can't get %dth previous pulse: %s", delta, err)
		return
	}

	pn := p.Pulse.PulseNumber
	err = m.syncClientsPool.LightCleanup(ctx, pn, m.RecentStorageProvider, jetIndexesRemoved)
	if err != nil {
		inslogger.FromContext(ctx).Errorf(
			"Error on light cleanup, until pulse = %v, singlefligt err = %v", pn, err)
	}

	p, err = m.PulseTracker.GetPreviousPulse(ctx, pn)
	if err != nil {
		inslogger.FromContext(ctx).Errorf("Can't get previous pulse: %s", err)
		return
	}
	m.JetModifier.Delete(ctx, p.Pulse.PulseNumber)
	m.NodeSetter.Delete(p.Pulse.PulseNumber)
	err = m.PulseTracker.DeletePulse(ctx, p.Pulse.PulseNumber)
	if err != nil {
		inslogger.FromContext(ctx).Errorf("Can't clean pulse-tracker from pulse: %s", err)
	}
}

func (m *PulseManager) prepareArtifactManagerMessageHandlerForNextPulse(ctx context.Context, newPulse core.Pulse, jets []jetInfo) {
	ctx, span := instracer.StartSpan(ctx, "early.close")
	defer span.End()

	m.HotDataWaiter.ThrowTimeout(ctx)

	logger := inslogger.FromContext(ctx)
	for _, jetInfo := range jets {
		if jetInfo.left == nil && jetInfo.right == nil {
			// No split happened.
			if jetInfo.mineNext {
				err := m.HotDataWaiter.Unlock(ctx, core.RecordID(jetInfo.id))
				if err != nil {
					logger.Error(err)
				}
			}
		} else {
			// Split happened.
			if jetInfo.left.mineNext {
				err := m.HotDataWaiter.Unlock(ctx, core.RecordID(jetInfo.left.id))
				if err != nil {
					logger.Error(err)
				}
			}
			if jetInfo.right.mineNext {
				err := m.HotDataWaiter.Unlock(ctx, core.RecordID(jetInfo.right.id))
				if err != nil {
					logger.Error(err)
				}
			}
		}
	}
}

// Start starts pulse manager, spawns replication goroutine under a hood.
func (m *PulseManager) Start(ctx context.Context) error {
	err := m.restoreLatestPulse(ctx)
	if err != nil {
		return err
	}

	origin := m.NodeNet.GetOrigin()
	err = m.NodeSetter.Set(core.FirstPulseNumber, []insolar.Node{{ID: origin.ID(), Role: origin.Role()}})
	if err != nil && err != storage.ErrOverride {
		return err
	}

	if m.options.enableSync && m.NodeNet.GetOrigin().Role() == core.StaticRoleLightMaterial {
		heavySyncPool := heavyclient.NewPool(
			m.Bus,
			m.PulseStorage,
			m.PulseTracker,
			m.ReplicaStorage,
			m.StorageCleaner,
			m.DBContext,
			heavyclient.Options{
				SyncMessageLimit: m.options.heavySyncMessageLimit,
				PulsesDeltaLimit: m.options.lightChainLimit,
			},
		)
		m.syncClientsPool = heavySyncPool

		err := m.initJetSyncState(ctx)
		if err != nil {
			return err
		}
	}

	return m.restoreGenesisRecentObjects(ctx)
}

func (m *PulseManager) restoreLatestPulse(ctx context.Context) error {
	if m.NodeNet.GetOrigin().Role() != core.StaticRoleHeavyMaterial {
		return nil
	}
	pulse, err := m.PulseTracker.GetLatestPulse(ctx)
	if err != nil {
		return err
	}
	m.PulseStorage.Lock()
	m.PulseStorage.Set(&pulse.Pulse)
	m.PulseStorage.Unlock()

	return nil
}

func (m *PulseManager) restoreGenesisRecentObjects(ctx context.Context) error {
	if m.NodeNet.GetOrigin().Role() == core.StaticRoleHeavyMaterial {
		return nil
	}

	jetID := core.RecordID(*core.NewJetID(0, nil))
	recent := m.RecentStorageProvider.GetIndexStorage(ctx, core.RecordID(jetID))

	return m.ObjectStorage.IterateIndexIDs(ctx, core.RecordID(jetID), func(id core.RecordID) error {
		if id.Pulse() == core.FirstPulseNumber {
			recent.AddObject(ctx, id)
		}
		return nil
	})
}

// Stop stops PulseManager. Waits replication goroutine is done.
func (m *PulseManager) Stop(ctx context.Context) error {
	// There should not to be any Set call after Stop call
	m.setLock.Lock()
	m.stopped = true
	m.setLock.Unlock()

	if m.options.enableSync && m.NodeNet.GetOrigin().Role() == core.StaticRoleLightMaterial {
		inslogger.FromContext(ctx).Info("waiting finish of heavy replication client...")
		m.syncClientsPool.Stop(ctx)
	}
	return nil
}

//
// Copyright 2019 Insolar Technologies GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package object

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/insolar/insolar/insolar"
	buswm "github.com/insolar/insolar/insolar/bus"
	"github.com/insolar/insolar/insolar/jet"
	"github.com/insolar/insolar/insolar/message"
	"github.com/insolar/insolar/insolar/payload"
	"github.com/insolar/insolar/insolar/pulse"
	"github.com/insolar/insolar/insolar/record"
	"github.com/insolar/insolar/insolar/reply"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/instrumentation/instracer"
	"github.com/insolar/insolar/network/storage"
	"github.com/pkg/errors"
	"go.opencensus.io/stats"
)

//go:generate minimock -i github.com/insolar/insolar/ledger/object.PendingModifier -o ./ -s _mock.go

// PendingModifier provides methods for modifying pending requests
type PendingModifier interface {
	// SetRequest sets a request for a specific object
	SetRequest(ctx context.Context, pn insolar.PulseNumber, objID insolar.ID, jetID insolar.JetID, reqID insolar.ID) error
	// SetResult sets a result for a specific object. Also, if there is a not closed request for a provided result,
	// the request will be closed
	SetResult(ctx context.Context, pn insolar.PulseNumber, objID insolar.ID, jetID insolar.JetID, resID insolar.ID, res record.Result) error
}

//go:generate minimock -i github.com/insolar/insolar/ledger/object.PendingAccessor -o ./ -s _mock.go

// PendingAccessor provides methods for fetching pending requests.
type PendingAccessor interface {
	// OpenRequestsForObjID returns a specific number of open requests for a specific object
	OpenRequestsForObjID(ctx context.Context, currentPN insolar.PulseNumber, objID insolar.ID, count int) ([]record.Request, error)
	// Records returns all the records for a provided object
	Records(ctx context.Context, currentPN insolar.PulseNumber, objID insolar.ID) ([]record.CompositeFilamentRecord, error)
	FirstPending(ctx context.Context, currentPN insolar.PulseNumber, objID insolar.ID) (*record.PendingFilament, error)
}

type HeavyPendingAccessor interface {
	Records(ctx context.Context, readFrom insolar.PulseNumber, readUntil insolar.PulseNumber, objID insolar.ID) ([]record.CompositeFilamentRecord, error)
}

type FilamentCacheManager interface {
	Gather(ctx context.Context, pn insolar.PulseNumber, objID insolar.ID) error
	SendAbandonedNotification(ctx context.Context, currentPN insolar.PulseNumber, objID insolar.ID) error
}

type FilamentCacheCleaner interface {
	// DeleteForPN method removes indexes from a storage for a provided
	DeleteForPN(ctx context.Context, pn insolar.PulseNumber)
}

// FilamentCacheStorage is a in-memory storage, that stores a collection of IndexBuckets
type FilamentCacheStorage struct {
	idxAccessor     IndexAccessor
	recordStorage   RecordStorage
	coordinator     jet.Coordinator
	pcs             insolar.PlatformCryptographyScheme
	pulseCalculator storage.PulseCalculator
	bus             insolar.MessageBus
	busWM           buswm.Sender

	bucketsLock sync.RWMutex
	buckets     map[insolar.PulseNumber]map[insolar.ID]*pendingMeta
}

func NewFilamentCacheStorage(
	idxAccessor IndexAccessor,
	recordStorage RecordStorage,
	coordinator jet.Coordinator,
	pcs insolar.PlatformCryptographyScheme,
	pulseCalculator storage.PulseCalculator,
	bus insolar.MessageBus,
	busWM buswm.Sender,
) *FilamentCacheStorage {
	return &FilamentCacheStorage{
		idxAccessor:     idxAccessor,
		recordStorage:   recordStorage,
		coordinator:     coordinator,
		pcs:             pcs,
		pulseCalculator: pulseCalculator,
		bus:             bus,
		busWM:           busWM,
		buckets:         map[insolar.PulseNumber]map[insolar.ID]*pendingMeta{},
	}
}

// pendingMeta contains info for calculating pending requests states
// The structure contains full chain of pendings (they are grouped by pulse).
// Groups are sorted by pulse, from a lowest to a highest
// There are a few maps inside, that help not to search through full fillament every SetRequest/SetResult
type pendingMeta struct {
	sync.RWMutex

	isStateCalculated bool

	fullFilament []chainLink

	notClosedRequestsIds      []insolar.ID
	notClosedRequestsIdsIndex map[insolar.PulseNumber]map[insolar.ID]struct{}

	resultsForOutOfLimitRequests map[insolar.ID]struct{}
}

type chainLink struct {
	PN             insolar.PulseNumber
	MetaRecordsIDs []insolar.ID
}

func (i *FilamentCacheStorage) createPendingBucket(ctx context.Context, pn insolar.PulseNumber, objID insolar.ID) *pendingMeta {
	i.bucketsLock.Lock()
	defer i.bucketsLock.Unlock()

	bucket := &pendingMeta{
		fullFilament:              []chainLink{},
		notClosedRequestsIds:      []insolar.ID{},
		notClosedRequestsIdsIndex: map[insolar.PulseNumber]map[insolar.ID]struct{}{},
		isStateCalculated:         false,
	}

	objsByPn, ok := i.buckets[pn]
	if !ok {
		objsByPn = map[insolar.ID]*pendingMeta{}
		i.buckets[pn] = objsByPn
	}

	_, ok = objsByPn[objID]
	if !ok {
		objsByPn[objID] = bucket
	}

	inslogger.FromContext(ctx).Debugf("[createPendingBucket] create bucket for obj - %v was created successfully", objID.DebugString())
	return bucket
}

func (i *FilamentCacheStorage) pendingBucket(pn insolar.PulseNumber, objID insolar.ID) *pendingMeta {
	i.bucketsLock.RLock()
	defer i.bucketsLock.RUnlock()

	objsByPn, ok := i.buckets[pn]
	if !ok {
		return nil
	}

	return objsByPn[objID]
}

// DeleteForPN deletes all buckets for a provided pulse number
func (i *FilamentCacheStorage) DeleteForPN(ctx context.Context, pn insolar.PulseNumber) {
	i.bucketsLock.Lock()
	defer i.bucketsLock.Unlock()

	delete(i.buckets, pn)
}

// SetRequest sets a request for a specific object
func (i *FilamentCacheStorage) SetRequest(ctx context.Context, pn insolar.PulseNumber, objID insolar.ID, jetID insolar.JetID, reqID insolar.ID) error {
	idx := i.idxAccessor.Index(pn, objID)
	if idx == nil {
		return ErrLifelineNotFound
	}
	idx.Lock()
	defer idx.Unlock()

	pb := i.pendingBucket(pn, objID)
	if pb == nil {
		pb = i.createPendingBucket(ctx, pn, objID)
	}

	pb.Lock()
	defer pb.Unlock()

	lfl := idx.objectMeta.Lifeline

	if lfl.PendingPointer != nil && reqID.Pulse() < lfl.PendingPointer.Pulse() {
		return errors.New("request from the past")
	}

	pf := record.PendingFilament{
		RecordID:       reqID,
		PreviousRecord: idx.objectMeta.Lifeline.PendingPointer,
	}

	if lfl.EarliestOpenRequest == nil {
		lfl.EarliestOpenRequest = &pn
	}

	pfv := record.Wrap(pf)
	hash := record.HashVirtual(i.pcs.ReferenceHasher(), pfv)
	metaID := *insolar.NewID(pn, hash)

	err := i.recordStorage.Set(ctx, metaID, record.Material{Virtual: &pfv, JetID: jetID})
	if err != nil {
		return errors.Wrap(err, "failed to create a meta-record about pending request")
	}

	idx.objectMeta.PendingRecords = append(idx.objectMeta.PendingRecords, metaID)
	lfl.PendingPointer = &metaID
	idx.objectMeta.Lifeline = lfl

	pb.addMetaIDToFilament(pn, metaID)

	_, ok := pb.notClosedRequestsIdsIndex[pn]
	if !ok {
		pb.notClosedRequestsIdsIndex[pn] = map[insolar.ID]struct{}{}
	}
	pb.notClosedRequestsIdsIndex[pn][reqID] = struct{}{}
	pb.notClosedRequestsIds = append(pb.notClosedRequestsIds, reqID)

	stats.Record(ctx,
		statObjectPendingRequestsInMemoryAddedCount.M(int64(1)),
	)

	inslogger.FromContext(ctx).Debugf("open requests - %v for - %v", len(pb.notClosedRequestsIds), objID.DebugString())

	return nil

}

func (b *pendingMeta) addMetaIDToFilament(pn insolar.PulseNumber, metaID insolar.ID) {
	isInserted := false
	for i, chainPart := range b.fullFilament {
		if chainPart.PN == pn {
			b.fullFilament[i].MetaRecordsIDs = append(b.fullFilament[i].MetaRecordsIDs, metaID)
			isInserted = true
		}
	}

	if !isInserted {
		b.fullFilament = append(b.fullFilament, chainLink{MetaRecordsIDs: []insolar.ID{metaID}, PN: pn})
	}
}

// SetResult sets a result for a specific object. Also, if there is a not closed request for a provided result,
// the request will be closed
func (i *FilamentCacheStorage) SetResult(ctx context.Context, pn insolar.PulseNumber, objID insolar.ID, jetID insolar.JetID, resID insolar.ID, res record.Result) error {
	logger := inslogger.FromContext(ctx)
	idx := i.idxAccessor.Index(pn, objID)
	if idx == nil {
		return ErrLifelineNotFound
	}
	idx.Lock()
	defer idx.Unlock()

	pb := i.pendingBucket(pn, objID)
	if pb == nil {
		pb = i.createPendingBucket(ctx, pn, objID)
	}

	pb.Lock()
	defer pb.Unlock()

	reqsIDs, ok := pb.notClosedRequestsIdsIndex[res.Request.Record().Pulse()]
	if !ok {
		return errors.New("not known result")
	}

	lfl := idx.objectMeta.Lifeline

	pf := record.PendingFilament{
		RecordID:       resID,
		PreviousRecord: lfl.PendingPointer,
	}

	pfv := record.Wrap(pf)
	hash := record.HashVirtual(i.pcs.ReferenceHasher(), pfv)
	metaID := *insolar.NewID(pn, hash)

	err := i.recordStorage.Set(ctx, metaID, record.Material{Virtual: &pfv, JetID: jetID})
	if err != nil {
		panic(errors.Wrapf(err, "obj id - %v", metaID.DebugString()))
		return errors.Wrap(err, "failed to create a meta-record about pending request")
	}

	pb.addMetaIDToFilament(pn, metaID)

	reqsIDs, ok = pb.notClosedRequestsIdsIndex[res.Request.Record().Pulse()]
	if ok {
		delete(reqsIDs, *res.Request.Record())
		for i := 0; i < len(pb.notClosedRequestsIds); i++ {
			if pb.notClosedRequestsIds[i] == *res.Request.Record() {
				pb.notClosedRequestsIds = append(pb.notClosedRequestsIds[:i], pb.notClosedRequestsIds[i+1:]...)
				break
			}
		}
	}

	if len(pb.notClosedRequestsIds) == 0 {
		logger.Debugf("no open requests for - %v", objID.DebugString())
		logger.Debugf("RefreshPendingFilament set EarliestOpenRequest - %v, val - %v", objID.DebugString(), lfl.EarliestOpenRequest)
		lfl.EarliestOpenRequest = nil
	}

	idx.objectMeta.Lifeline = lfl

	stats.Record(ctx,
		statObjectPendingResultsInMemoryAddedCount.M(int64(1)),
	)

	return nil
}

// SetFilament adds a slice of records to an object with provided id and pulse. It's assumed, that the method is
// called for setting records from another light, during the process of filling full chaing of pendings
func (i *FilamentCacheStorage) setFilament(ctx context.Context, pm *pendingMeta, filPN insolar.PulseNumber, recs []record.CompositeFilamentRecord) error {
	recsIds := make([]insolar.ID, len(recs))
	for idx, rec := range recs {
		recsIds[idx] = rec.MetaID

		err := i.recordStorage.Set(ctx, rec.MetaID, rec.Meta)
		if err != nil && err != ErrOverride {
			panic(errors.Wrapf(err, "obj id - %v", rec.MetaID.DebugString()))
			return errors.Wrap(err, "filament update failed")
		}
		err = i.recordStorage.Set(ctx, rec.RecordID, rec.Record)
		if err != nil && err != ErrOverride {
			panic(errors.Wrapf(err, "obj id - %v", rec.MetaID.DebugString()))
			return errors.Wrap(err, "filament update failed")
		}
	}

	pm.fullFilament = append(pm.fullFilament, chainLink{MetaRecordsIDs: recsIds, PN: filPN})
	sort.Slice(pm.fullFilament, func(i, j int) bool {
		return pm.fullFilament[i].PN < pm.fullFilament[j].PN
	})

	return nil
}

func (i *FilamentCacheStorage) Gather(ctx context.Context, pn insolar.PulseNumber, objID insolar.ID) error {
	idx := i.idxAccessor.Index(pn, objID)
	if idx == nil {
		return ErrLifelineNotFound
	}

	idx.Lock()
	defer idx.Unlock()

	pb := i.pendingBucket(pn, objID)
	if pb == nil {
		pb = i.createPendingBucket(ctx, pn, objID)
	}

	pb.Lock()
	defer pb.Unlock()

	logger := inslogger.FromContext(ctx)
	lfl := idx.objectMeta.Lifeline

	// state already calculated
	if pb.isStateCalculated {
		logger.Debugf("Gather filament. objID - %v, pn - %v. State is already calculated", objID, pn)
		return nil
	}

	// No pendings
	if lfl.PendingPointer == nil {
		logger.Debugf("Gather filament. objID - %v, pn - %v. No pendings", objID, pn)
		return nil
	}
	// No open pendings
	if lfl.EarliestOpenRequest == nil {
		logger.Debugf("Gather filament. objID - %v, pn - %v. No open pendings", objID, pn)
		return nil
	}
	// If an earliest pending created during a current pulse
	if lfl.EarliestOpenRequest != nil && *lfl.EarliestOpenRequest == pn {
		logger.Debugf("Gather filament. objID - %v, pn - %v. If an earliest pending created during a current pulse", objID, pn)
		return nil
	}

	fp, err := i.firstPending(ctx, pb)
	if err != nil {
		panic(err)
		return err
	}

	if fp == nil || fp.PreviousRecord == nil {
		err = i.fillPendingFilament(ctx, pn, objID, lfl.PendingPointer.Pulse(), *lfl.EarliestOpenRequest, idx.objectMeta.PendingRecords, pb)
		if err != nil {
			panic(err)
			return err
		}
	} else {
		err = i.fillPendingFilament(ctx, pn, objID, fp.PreviousRecord.Pulse(), *lfl.EarliestOpenRequest, idx.objectMeta.PendingRecords, pb)
		if err != nil {
			panic(err)
			return err
		}
	}

	err = i.refresh(ctx, pn, objID)
	if err != nil {
		panic(err)
		return err
	}

	return nil
}

func (i *FilamentCacheStorage) SendAbandonedNotification(ctx context.Context, currentPN insolar.PulseNumber, objID insolar.ID) error {
	logger := inslogger.FromContext(ctx)
	idx := i.idxAccessor.Index(currentPN, objID)
	if idx == nil {
		return ErrLifelineNotFound
	}

	idx.RLock()
	defer idx.RUnlock()

	if idx.objectMeta.Lifeline.EarliestOpenRequest == nil {
		return nil
	}

	notifyPoint, err := i.pulseCalculator.Backwards(ctx, currentPN, 2)
	if err == pulse.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if notifyPoint.PulseNumber < *idx.objectMeta.Lifeline.EarliestOpenRequest {
		return nil
	}

	rep, err := i.bus.Send(ctx, &message.AbandonedRequestsNotification{
		Object: objID,
	}, nil)
	if err != nil {
		logger.Error("failed to notify about pending requests")
		return err
	}
	if _, ok := rep.(*reply.OK); !ok {
		logger.Error("received unexpected reply on pending notification")
		return errors.New("received unexpected reply on pending notification")
	}
	return nil
}

func (i *FilamentCacheStorage) firstPending(ctx context.Context, pb *pendingMeta) (*record.PendingFilament, error) {
	if len(pb.fullFilament) == 0 {
		return nil, nil
	}
	if len(pb.fullFilament[0].MetaRecordsIDs) == 0 {
		return nil, nil
	}

	metaID := pb.fullFilament[0].MetaRecordsIDs[0]
	rec, err := i.recordStorage.ForID(ctx, metaID)
	if err != nil {
		return nil, err
	}

	return record.Unwrap(rec.Virtual).(*record.PendingFilament), nil
}

func (i *FilamentCacheStorage) fillPendingFilament(
	ctx context.Context,
	currentPN insolar.PulseNumber,
	objID insolar.ID,
	destPN insolar.PulseNumber,
	earlistOpenRequest insolar.PulseNumber,
	pendingRecords []insolar.ID,
	pm *pendingMeta,
) error {
	ctx, span := instracer.StartSpan(ctx, fmt.Sprintf("RefreshPendingFilament.fillPendingFilament"))
	defer span.End()

	continueFilling := true

	for continueFilling {
		node, err := i.coordinator.NodeForObject(ctx, objID, currentPN, destPN)
		if err != nil {
			panic(err)
			return err
		}

		var pl payload.Payload
		// TODO: temp hack waiting for INS-2597 INS-2598 @egorikas
		// Because a current node can be a previous LME for a object
		if *node == i.coordinator.Me() {
			res := make([]record.CompositeFilamentRecord, len(pendingRecords))
			for idx, id := range pendingRecords {
				metaRec, err := i.recordStorage.ForID(ctx, id)
				if err != nil {
					panic(err)
				}

				concreteMeta := record.Unwrap(metaRec.Virtual).(*record.PendingFilament)
				rec, err := i.recordStorage.ForID(ctx, concreteMeta.RecordID)
				if err != nil {
					panic(err)
				}

				res[idx] = record.CompositeFilamentRecord{
					Record:   rec,
					RecordID: concreteMeta.RecordID,
					Meta:     metaRec,
					MetaID:   id,
				}
			}
			inslogger.FromContext(ctx).Debugf("RefreshPendingFilament objID == %v, records - %v", objID.DebugString(), len(res))
			pl = &payload.PendingFilament{
				ObjectID: objID,
				Records:  res,
			}
		} else {
			msg, err := payload.NewMessage(&payload.GetPendingFilament{
				ObjectID:  objID,
				StartFrom: destPN,
				ReadUntil: earlistOpenRequest,
			})
			if err != nil {
				panic(err)
				return errors.Wrap(err, "failed to create a GetPendingFilament message")
			}

			rep, done := i.busWM.SendTarget(ctx, msg, *node)
			defer done()

			var ok bool
			res, ok := <-rep
			if !ok {
				panic(err)
				return errors.New("no reply")
			}

			pl, err = payload.UnmarshalFromMeta(res.Payload)
			if err != nil {
				panic(err)
				return errors.Wrap(err, "failed to unmarshal reply")
			}

		}
		switch r := pl.(type) {
		case *payload.PendingFilament:
			err := i.setFilament(ctx, pm, destPN, r.Records)
			if err != nil {
				panic(err)
				return err
			}

			if len(r.Records) == 0 {
				panic(fmt.Sprintf("unexpected behaviour - %v", earlistOpenRequest))
			}

			firstRec := record.Unwrap(r.Records[0].Meta.Virtual).(*record.PendingFilament)
			if firstRec.PreviousRecord == nil {
				continueFilling = false
				return nil
			}

			// If know border read to the start of the chain
			// In other words, we read until limit
			if firstRec.PreviousRecord.Pulse() > earlistOpenRequest {
				destPN = firstRec.PreviousRecord.Pulse()
			} else {
				continueFilling = false
			}
		case *payload.Error:
			panic(err)
			return errors.New(r.Text)
		default:
			panic(fmt.Errorf("RefreshPendingFilament: unexpected reply: %#v", r))
			return fmt.Errorf("RefreshPendingFilament: unexpected reply: %#v", r)
		}
	}

	return nil
}

// RefreshState recalculates state of the chain, marks requests as closed and opened.
func (i *FilamentCacheStorage) refresh(ctx context.Context, pn insolar.PulseNumber, objID insolar.ID) error {
	println("RefreshState")
	logger := inslogger.FromContext(ctx)
	logger.Debugf("RefreshState for objID: %v pn: %v", objID.DebugString(), pn)

	idx := i.idxAccessor.Index(pn, objID)
	if idx == nil {
		return ErrLifelineNotFound
	}
	pb := i.pendingBucket(pn, objID)
	if pb == nil {
		return ErrLifelineNotFound
	}

	pb.Lock()
	defer pb.Unlock()

	if pb.isStateCalculated {
		return nil
	}

	for _, chainLink := range pb.fullFilament {
		for _, metaID := range chainLink.MetaRecordsIDs {
			metaRec, err := i.recordStorage.ForID(ctx, metaID)
			if err != nil {
				panic(errors.Wrapf(err, "obj id - %v", metaID.DebugString()))
				return errors.Wrap(err, "failed to refresh an index state")
			}

			concreteMeta := record.Unwrap(metaRec.Virtual).(*record.PendingFilament)
			rec, err := i.recordStorage.ForID(ctx, concreteMeta.RecordID)
			if err != nil {
				panic(errors.Wrapf(err, "obj id - %v", concreteMeta.RecordID.DebugString()))
				return errors.Wrap(err, "failed to refresh an index state")
			}

			switch r := record.Unwrap(rec.Virtual).(type) {
			case *record.Request:
				_, ok := pb.notClosedRequestsIdsIndex[chainLink.PN]
				if !ok {
					pb.notClosedRequestsIdsIndex[chainLink.PN] = map[insolar.ID]struct{}{}
				}
				pb.notClosedRequestsIdsIndex[chainLink.PN][*r.Object.Record()] = struct{}{}
			case *record.Result:
				println(r.Request.Record().Pulse())
				openReqs, ok := pb.notClosedRequestsIdsIndex[r.Request.Record().Pulse()]
				if ok {
					delete(openReqs, *r.Request.Record())
				}
			default:
				panic(fmt.Sprintf("unknow type - %v", r))
			}
		}
	}

	isEarliestFound := false

	for i, chainLink := range pb.fullFilament {
		if len(pb.notClosedRequestsIdsIndex[chainLink.PN]) != 0 {
			if !isEarliestFound {
				idx.objectMeta.Lifeline.EarliestOpenRequest = &pb.fullFilament[i].PN
				logger.Debugf("RefreshPendingFilament set EarliestOpenRequest - %v, val - %v", objID.DebugString(), idx.objectMeta.Lifeline.EarliestOpenRequest)
				isEarliestFound = true
			}

			for openReqID := range pb.notClosedRequestsIdsIndex[chainLink.PN] {
				pb.notClosedRequestsIds = append(pb.notClosedRequestsIds, openReqID)
			}
		}
	}

	logger.Debugf("RefreshState. Close channel for objID: %v pn: %v", objID.DebugString(), pn)
	pb.isStateCalculated = true

	if len(pb.notClosedRequestsIds) == 0 {
		idx.objectMeta.Lifeline.EarliestOpenRequest = nil
		logger.Debugf("RefreshPendingFilament set EarliestOpenRequest - %v, val - %v", objID.DebugString(), idx.objectMeta.Lifeline.EarliestOpenRequest)
		logger.Debugf("no open requests for - %v", objID.DebugString())
	}

	return nil
}

// OpenRequestsForObjID returns open requests for a specific object
func (i *FilamentCacheStorage) OpenRequestsForObjID(ctx context.Context, currentPN insolar.PulseNumber, objID insolar.ID, count int) ([]record.Request, error) {
	pb := i.pendingBucket(currentPN, objID)
	if pb == nil {
		return nil, ErrLifelineNotFound
	}

	pb.RLock()
	defer pb.RUnlock()

	if len(pb.notClosedRequestsIds) < count {
		count = len(pb.notClosedRequestsIds)
	}

	res := make([]record.Request, count)

	for idx := 0; idx < count; idx++ {
		rec, err := i.recordStorage.ForID(ctx, pb.notClosedRequestsIds[idx])
		if err != nil {
			return nil, err
		}

		switch r := record.Unwrap(rec.Virtual).(type) {
		case *record.Request:
			res[idx] = *r
		default:
			panic("filament is totally broken")
		}
	}

	return res, nil
}

// Records returns all the records for a provided object
func (i *FilamentCacheStorage) Records(ctx context.Context, currentPN insolar.PulseNumber, objID insolar.ID) ([]record.CompositeFilamentRecord, error) {
	idx := i.idxAccessor.Index(currentPN, objID)
	if idx == nil {
		return nil, ErrLifelineNotFound
	}
	b := i.pendingBucket(currentPN, objID)
	if b == nil {
		return nil, ErrLifelineNotFound
	}

	idx.RLock()
	defer idx.RUnlock()
	b.RLock()
	defer b.RLock()

	res := make([]record.CompositeFilamentRecord, len(idx.objectMeta.PendingRecords))
	for idx, id := range idx.objectMeta.PendingRecords {
		metaRec, err := i.recordStorage.ForID(ctx, id)
		if err != nil {
			return nil, err
		}

		concreteMeta := record.Unwrap(metaRec.Virtual).(*record.PendingFilament)
		rec, err := i.recordStorage.ForID(ctx, concreteMeta.RecordID)
		if err != nil {
			return nil, err
		}

		res[idx] = record.CompositeFilamentRecord{
			Record:   rec,
			RecordID: concreteMeta.RecordID,
			Meta:     metaRec,
			MetaID:   id,
		}
	}

	return res, nil
}

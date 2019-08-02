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

package light

import (
	"context"

	"github.com/insolar/insolar/network/rules"

	"github.com/ThreeDotsLabs/watermill"
	watermillMsg "github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/infrastructure/gochannel"
	"github.com/insolar/insolar/api"
	"github.com/insolar/insolar/certificate"
	"github.com/insolar/insolar/component"
	"github.com/insolar/insolar/configuration"
	"github.com/insolar/insolar/contractrequester"
	"github.com/insolar/insolar/cryptography"
	"github.com/insolar/insolar/genesisdataprovider"
	"github.com/insolar/insolar/insolar"
	"github.com/insolar/insolar/insolar/bus"
	"github.com/insolar/insolar/insolar/delegationtoken"
	"github.com/insolar/insolar/insolar/jet"
	"github.com/insolar/insolar/insolar/jetcoordinator"
	"github.com/insolar/insolar/insolar/message"
	"github.com/insolar/insolar/insolar/node"
	"github.com/insolar/insolar/insolar/pulse"
	"github.com/insolar/insolar/instrumentation/inslogger"
	"github.com/insolar/insolar/keystore"
	"github.com/insolar/insolar/ledger/drop"
	"github.com/insolar/insolar/ledger/light/artifactmanager"
	"github.com/insolar/insolar/ledger/light/executor"
	"github.com/insolar/insolar/ledger/light/hot"
	"github.com/insolar/insolar/ledger/light/pulsemanager"
	"github.com/insolar/insolar/ledger/light/replication"
	"github.com/insolar/insolar/ledger/object"
	"github.com/insolar/insolar/log"
	"github.com/insolar/insolar/logicrunner/artifacts"
	"github.com/insolar/insolar/messagebus"
	"github.com/insolar/insolar/metrics"
	"github.com/insolar/insolar/network/nodenetwork"
	"github.com/insolar/insolar/network/servicenetwork"
	"github.com/insolar/insolar/network/termination"
	"github.com/insolar/insolar/platformpolicy"
	"github.com/insolar/insolar/server/internal"
	"github.com/pkg/errors"
)

type components struct {
	cmp               component.Manager
	NodeRef, NodeRole string
	replicator        replication.LightReplicator
	cleaner           replication.Cleaner
}

func newComponents(ctx context.Context, cfg configuration.Configuration) (*components, error) {
	// Cryptography.
	var (
		KeyProcessor  insolar.KeyProcessor
		CryptoScheme  insolar.PlatformCryptographyScheme
		CryptoService insolar.CryptographyService
		CertManager   insolar.CertificateManager
	)
	{
		var err error
		// Private key storage.
		ks, err := keystore.NewKeyStore(cfg.KeysPath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load KeyStore")
		}
		// Public key manipulations.
		KeyProcessor = platformpolicy.NewKeyProcessor()
		// Platform cryptography.
		CryptoScheme = platformpolicy.NewPlatformCryptographyScheme()
		// Sign, verify, etc.
		CryptoService = cryptography.NewCryptographyService()

		c := component.Manager{}
		c.Inject(CryptoService, CryptoScheme, KeyProcessor, ks)

		publicKey, err := CryptoService.GetPublicKey()
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve node public key")
		}

		// Node certificate.
		CertManager, err = certificate.NewManagerReadCertificate(publicKey, KeyProcessor, cfg.CertificatePath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start Certificate")
		}
	}

	comps := &components{}
	comps.cmp = component.Manager{}
	comps.NodeRef = CertManager.GetCertificate().GetNodeRef().String()
	comps.NodeRole = CertManager.GetCertificate().GetRole().String()

	logger := log.NewWatermillLogAdapter(inslogger.FromContext(ctx))
	pubSub := gochannel.NewGoChannel(gochannel.Config{}, logger)
	pubSub = internal.PubSubWrapper(ctx, &comps.cmp, cfg.Introspection, pubSub)

	// Network.
	var (
		NetworkService *servicenetwork.ServiceNetwork
		NodeNetwork    insolar.NodeNetwork
		Termination    insolar.TerminationHandler
	)
	{
		var err error
		// External communication.
		NetworkService, err = servicenetwork.NewServiceNetwork(cfg, &comps.cmp)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start Network")
		}

		Termination = termination.NewHandler(NetworkService)

		// Node info.
		NodeNetwork, err = nodenetwork.NewNodeNetwork(cfg.Host.Transport, CertManager.GetCertificate())
		if err != nil {
			return nil, errors.Wrap(err, "failed to start NodeNetwork")
		}

	}

	// API.
	var (
		Requester insolar.ContractRequester
		Genesis   insolar.GenesisDataProvider
		API       insolar.APIRunner
	)
	{
		var err error
		Requester, err = contractrequester.New(nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start ContractRequester")
		}

		Genesis, err = genesisdataprovider.New()
		if err != nil {
			return nil, errors.Wrap(err, "failed to start GenesisDataProvider")
		}

		API, err = api.NewRunner(&cfg.APIRunner)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start ApiRunner")
		}
	}

	// Role calculations.
	var (
		Coordinator jet.Coordinator
		Pulses      *pulse.StorageMem
		Jets        jet.Storage
		Nodes       *node.Storage
	)
	{
		Nodes = node.NewStorage()
		Pulses = pulse.NewStorageMem()
		Jets = jet.NewStore()

		c := jetcoordinator.NewJetCoordinator(cfg.Ledger.LightChainLimit)
		c.PulseCalculator = Pulses
		c.PulseAccessor = Pulses
		c.JetAccessor = Jets
		c.NodeNet = NodeNetwork
		c.PlatformCryptographyScheme = CryptoScheme
		c.Nodes = Nodes

		Coordinator = c
	}

	// Communication.
	var (
		Tokens  insolar.DelegationTokenFactory
		Parcels message.ParcelFactory
		Bus     insolar.MessageBus
		WmBus   *bus.Bus
	)
	{
		var err error
		Tokens = delegationtoken.NewDelegationTokenFactory()
		Parcels = messagebus.NewParcelFactory()
		Bus, err = messagebus.NewMessageBus(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "failed to start MessageBus")
		}
		WmBus = bus.NewBus(cfg.Bus, pubSub, Pulses, Coordinator, CryptoScheme)
	}

	metricsHandler, err := metrics.NewMetrics(
		ctx,
		cfg.Metrics,
		metrics.GetInsolarRegistry(comps.NodeRole),
		comps.NodeRole,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Metrics")
	}

	// Light components.
	var (
		PulseManager insolar.PulseManager
		Handler      *artifactmanager.MessageHandler
	)
	{
		conf := cfg.Ledger
		idLocker := object.NewIndexLocker()
		drops := drop.NewStorageMemory()
		records := object.NewRecordMemory()
		indexes := object.NewIndexStorageMemory()
		writeController := hot.NewWriteController()
		waiter := hot.NewChannelWaiter()

		c := component.Manager{}
		c.Inject(CryptoScheme)

		handler := artifactmanager.NewMessageHandler(&conf)
		handler.PulseCalculator = Pulses
		handler.FlowDispatcher.PulseAccessor = Pulses

		handler.PCS = CryptoScheme
		handler.JetCoordinator = Coordinator
		handler.JetStorage = Jets
		handler.DropModifier = drops
		handler.IndexLocker = idLocker
		handler.Records = records
		handler.HotDataWaiter = waiter
		handler.JetReleaser = waiter
		handler.Sender = WmBus
		handler.WriteAccessor = writeController
		handler.Sender = WmBus
		handler.IndexStorage = indexes

		jetTreeUpdater := executor.NewFetcher(Nodes, Jets, WmBus, Coordinator)
		filamentCalculator := executor.NewFilamentCalculator(
			indexes,
			records,
			Coordinator,
			jetTreeUpdater,
			WmBus,
			Pulses,
		)
		requestChecker := executor.NewRequestChecker(filamentCalculator, Coordinator, jetTreeUpdater, WmBus)

		handler.JetTreeUpdater = jetTreeUpdater
		handler.FilamentCalculator = filamentCalculator
		handler.RequestChecker = requestChecker

		jetCalculator := executor.NewJetCalculator(Coordinator, Jets)
		lightCleaner := replication.NewCleaner(
			Jets.(jet.Cleaner),
			Nodes,
			drops,
			records,
			indexes,
			Pulses,
			Pulses,
			indexes,
			filamentCalculator,
			conf.LightChainLimit,
			conf.CleanerDelay,
		)
		comps.cleaner = lightCleaner

		lthSyncer := replication.NewReplicatorDefault(
			jetCalculator,
			lightCleaner,
			WmBus,
			Pulses,
			drops,
			records,
			indexes,
			Jets,
		)
		comps.replicator = lthSyncer

		jetSplitter := executor.NewJetSplitter(
			conf.JetSplit, jetCalculator, Jets, Jets, drops, drops, Pulses, records,
		)

		hotSender := executor.NewHotSender(
			drops,
			indexes,
			Pulses,
			Jets,
			conf.LightChainLimit,
			WmBus,
		)

		stateIniter := executor.NewStateIniter(Jets, waiter, drops, Nodes, WmBus)

		pm := pulsemanager.NewPulseManager(
			jetSplitter,
			lthSyncer,
			writeController,
			hotSender,
			stateIniter,
		)
		pm.MessageHandler = handler
		pm.Bus = Bus
		pm.NodeNet = NodeNetwork
		pm.JetReleaser = waiter
		pm.JetModifier = Jets
		pm.NodeSetter = Nodes
		pm.Nodes = Nodes
		pm.PulseAccessor = Pulses
		pm.PulseCalculator = Pulses
		pm.PulseAppender = Pulses

		PulseManager = pm
		Handler = handler
	}

	comps.cmp.Inject(
		WmBus,
		Handler,
		Jets,
		Pulses,
		Coordinator,
		PulseManager,
		metricsHandler,
		Bus,
		Requester,
		Tokens,
		Parcels,
		artifacts.NewClient(WmBus),
		Genesis,
		API,
		KeyProcessor,
		Termination,
		CryptoScheme,
		CryptoService,
		CertManager,
		NodeNetwork,
		NetworkService,
		pubSub,
		rules.NewRules(),
	)

	err = comps.cmp.Init(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init components")
	}

	comps.startWatermill(ctx, logger, pubSub, WmBus, NetworkService.SendMessageHandler, Handler.FlowDispatcher.Process)

	return comps, nil
}

func (c *components) Start(ctx context.Context) error {
	return c.cmp.Start(ctx)
}

func (c *components) Stop(ctx context.Context) error {
	c.replicator.Stop()
	c.cleaner.Stop()
	return c.cmp.Stop(ctx)
}

func (c *components) startWatermill(
	ctx context.Context,
	logger watermill.LoggerAdapter,
	pubSub watermillMsg.PubSub,
	b *bus.Bus,
	outHandler, inHandler watermillMsg.HandlerFunc,
) {
	inRouter, err := watermillMsg.NewRouter(watermillMsg.RouterConfig{}, logger)
	if err != nil {
		panic(err)
	}
	outRouter, err := watermillMsg.NewRouter(watermillMsg.RouterConfig{}, logger)
	if err != nil {
		panic(err)
	}

	outRouter.AddNoPublisherHandler(
		"OutgoingHandler",
		bus.TopicOutgoing,
		pubSub,
		outHandler,
	)

	inRouter.AddMiddleware(
		b.IncomingMessageRouter,
	)

	inRouter.AddNoPublisherHandler(
		"IncomingHandler",
		bus.TopicIncoming,
		pubSub,
		inHandler,
	)

	startRouter(ctx, inRouter)
	startRouter(ctx, outRouter)
}

func startRouter(ctx context.Context, router *watermillMsg.Router) {
	go func() {
		if err := router.Run(); err != nil {
			inslogger.FromContext(ctx).Error("Error while running router", err)
		}
	}()
	<-router.Running()
}

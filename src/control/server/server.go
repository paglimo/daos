//
// (C) Copyright 2018-2021 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package server

import (
	"context"
	"net"
	"os"
	"os/signal"
	"os/user"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/daos-stack/daos/src/control/build"
	ctlpb "github.com/daos-stack/daos/src/control/common/proto/ctl"
	mgmtpb "github.com/daos-stack/daos/src/control/common/proto/mgmt"
	"github.com/daos-stack/daos/src/control/events"
	"github.com/daos-stack/daos/src/control/lib/control"
	"github.com/daos-stack/daos/src/control/logging"
	"github.com/daos-stack/daos/src/control/security"
	"github.com/daos-stack/daos/src/control/server/config"
	"github.com/daos-stack/daos/src/control/server/engine"
	"github.com/daos-stack/daos/src/control/server/storage/bdev"
	"github.com/daos-stack/daos/src/control/server/storage/scm"
	"github.com/daos-stack/daos/src/control/system"
)

func processConfig(log *logging.LeveledLogger, cfg *config.Server) (*system.FaultDomain, error) {
	err := cfg.Validate(log)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: validation failed", cfg.Path)
	}

	cfg.SaveActiveConfig(log)

	if err := setDaosHelperEnvs(cfg, os.Setenv); err != nil {
		return nil, err
	}

	faultDomain, err := getFaultDomain(cfg)
	if err != nil {
		return nil, err
	}
	log.Debugf("fault domain: %s", faultDomain.String())

	return faultDomain, nil
}

// server struct contains state and components of DAOS Server.
type server struct {
	log         *logging.LeveledLogger
	cfg         *config.Server
	faultDomain *system.FaultDomain
	ctlAddr     *net.TCPAddr
	netDevClass uint32
	listener    net.Listener

	harness      *EngineHarness
	membership   *system.Membership
	sysdb        *system.Database
	pubSub       *events.PubSub
	evtForwarder *control.EventForwarder
	evtLogger    *control.EventLogger
	ctlSvc       *ControlService
	mgmtSvc      *mgmtSvc
	scmProvider  *scm.Provider
	bdevProvider *bdev.Provider
	grpcServer   *grpc.Server

	enginesStartedCbs []func(context.Context)
}

func newServer(ctx context.Context, log *logging.LeveledLogger, cfg *config.Server, faultDomain *system.FaultDomain) (*server, error) {
	harness := NewEngineHarness(log).WithFaultDomain(faultDomain)

	// Create storage subsystem providers.
	scmProvider := scm.DefaultProvider(log)
	bdevProvider := bdev.DefaultProvider(log)

	return &server{
		log:          log,
		cfg:          cfg,
		faultDomain:  faultDomain,
		harness:      harness,
		scmProvider:  scmProvider,
		bdevProvider: bdevProvider,
	}, nil
}

// createServices builds scaffolding for rpc and event services.
func (srv *server) createServices(ctx context.Context) error {
	dbReplicas, err := cfgGetReplicas(srv.cfg, net.ResolveTCPAddr)
	if err != nil {
		return errors.Wrap(err, "retrieve replicas from config")
	}

	// If this daos_server instance ends up being the MS leader,
	// this will record the DAOS system membership.
	sysdb, err := system.NewDatabase(srv.log, &system.DatabaseConfig{
		Replicas:   dbReplicas,
		RaftDir:    cfgGetRaftDir(srv.cfg),
		SystemName: srv.cfg.SystemName,
	})
	if err != nil {
		return errors.Wrap(err, "create system database")
	}
	srv.sysdb = sysdb
	srv.membership = system.NewMembership(srv.log, sysdb)

	// Create rpcClient for inter-server communication.
	cliCfg := control.DefaultConfig()
	cliCfg.TransportConfig = srv.cfg.TransportConfig
	rpcClient := control.NewClient(
		control.WithConfig(cliCfg),
		control.WithClientLogger(srv.log))

	// Create event distribution primitives.
	srv.pubSub = events.NewPubSub(ctx, srv.log)
	srv.evtForwarder = control.NewEventForwarder(rpcClient, srv.cfg.AccessPoints)
	srv.evtLogger = control.NewEventLogger(srv.log)

	srv.ctlSvc = NewControlService(srv.log, srv.harness, srv.bdevProvider, srv.scmProvider,
		srv.cfg, srv.pubSub)

	srv.mgmtSvc = newMgmtSvc(srv.harness, srv.membership, sysdb, rpcClient, srv.pubSub)

	return nil
}

func (srv *server) onEnginesStarted(fn func(context.Context)) {
	srv.enginesStartedCbs = append(srv.enginesStartedCbs, fn)
}

func (srv *server) shutdown() {
	srv.pubSub.Close()
}

// initNetwork resolves local address and starts TCP listener then calls
// netInit to process network configuration.
func (srv *server) initNetwork(ctx context.Context) error {
	ctlAddr, listener, err := connectServer(srv.cfg.ControlPort, net.ResolveTCPAddr, net.Listen)
	if err != nil {
		return err
	}
	srv.ctlAddr = ctlAddr
	srv.listener = listener

	ndc, err := netInit(ctx, srv.log, srv.cfg)
	if err != nil {
		return err
	}
	srv.netDevClass = ndc

	return nil
}

func (srv *server) initStorage() error {
	runningUser, err := user.Current()
	if err != nil {
		return errors.Wrap(err, "unable to lookup current user")
	}

	if err := prepBdevStorage(srv, runningUser, iommuDetected(), getHugePageInfo); err != nil {
		return err
	}

	return srv.ctlSvc.Setup()
}

func (srv *server) createEngine(ei int, ec *engine.Config) (*EngineInstance, error) {
	// Closure to join an engine instance to a system using control API.
	joinFn := func(ctxIn context.Context, req *control.SystemJoinReq) (*control.SystemJoinResp, error) {
		req.SetHostList(srv.cfg.AccessPoints)
		req.SetSystem(srv.cfg.SystemName)
		req.ControlAddr = srv.ctlAddr

		return control.SystemJoin(ctxIn, srv.mgmtSvc.rpcClient, req)
	}

	// Indicate whether VMD devices have been detected and can be used.
	ec.Storage.Bdev.VmdDisabled = srv.bdevProvider.IsVMDDisabled()

	// TODO: ClassProvider should be encapsulated within bdevProvider
	bcp, err := bdev.NewClassProvider(srv.log, ec.Storage.SCM.MountPoint, &ec.Storage.Bdev)
	if err != nil {
		return nil, err
	}

	engine := NewEngineInstance(srv.log, bcp, srv.scmProvider, joinFn,
		engine.NewRunner(srv.log, ec)).WithHostFaultDomain(srv.harness.faultDomain)

	if ei == 0 {
		return engine, nil
	}

	configureFirstEngine(engine, srv.sysdb, joinFn)
	return engine, nil
}

// addEngines creates and adds engine instances to harness then starts
// goroutine to execute callbacks when all engines are started.
func (srv *server) addEngines(ctx context.Context) error {
	var allStarted sync.WaitGroup
	registerTelemetryCallback(ctx, srv)

	for ei, ec := range srv.cfg.Engines {
		engine, err := srv.createEngine(ei, ec)
		if err != nil {
			return err
		}

		registerEngineCallbacks(engine, srv.pubSub, &allStarted)

		if err := srv.harness.AddInstance(engine); err != nil {
			return err
		}
		// increment count of engines waiting to start
		allStarted.Add(1)
	}

	go func(ctxIn context.Context) {
		allStarted.Wait()
		for _, cb := range srv.enginesStartedCbs {
			cb(ctxIn)
		}
	}(ctx)

	return nil
}

// setupGrpc creates a new grpc server and registers services.
func (srv *server) setupGrpc() error {
	srvOpts, err := getGrpcOpts(srv.cfg.TransportConfig)
	if err != nil {
		return err
	}

	srv.grpcServer = grpc.NewServer(srvOpts...)
	ctlpb.RegisterCtlSvcServer(srv.grpcServer, srv.ctlSvc)

	srv.mgmtSvc.clientNetworkCfg = &config.ClientNetworkCfg{
		Provider:        srv.cfg.Fabric.Provider,
		CrtCtxShareAddr: srv.cfg.Fabric.CrtCtxShareAddr,
		CrtTimeout:      srv.cfg.Fabric.CrtTimeout,
		NetDevClass:     srv.netDevClass,
	}
	mgmtpb.RegisterMgmtSvcServer(srv.grpcServer, srv.mgmtSvc)

	tSec, err := security.DialOptionForTransportConfig(srv.cfg.TransportConfig)
	if err != nil {
		return err
	}
	srv.sysdb.ConfigureTransport(srv.grpcServer, tSec)

	return nil
}

func (srv *server) registerEvents() {
	registerInitialSubscriptions(srv)

	srv.sysdb.OnLeadershipGained(func(ctx context.Context) error {
		srv.log.Infof("MS leader running on %s", hostname())
		srv.mgmtSvc.startJoinLoop(ctx)
		registerLeaderSubscriptions(srv)
		return nil
	})
	srv.sysdb.OnLeadershipLost(func() error {
		srv.log.Infof("MS leader no longer running on %s", hostname())
		registerFollowerSubscriptions(srv)
		return nil
	})
}

func (srv *server) start(ctx context.Context, shutdown context.CancelFunc) error {
	go func() {
		_ = srv.grpcServer.Serve(srv.listener)
	}()
	defer srv.grpcServer.Stop()

	srv.log.Infof("%s v%s (pid %d) listening on %s", build.ControlPlaneName,
		build.DaosVersion, os.Getpid(), srv.ctlAddr)

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		srv.log.Debugf("Caught signal: %s", sig)

		shutdown()
	}()

	return errors.Wrapf(srv.harness.Start(ctx, srv.sysdb, srv.pubSub, srv.cfg),
		"%s harness exited", build.ControlPlaneName)
}

// Start is the entry point for a daos_server instance, perform the following
// tasks in order to bring-up:
//
// - create server struct with component references
// - create service wrappers for rpc and event services
// - initialize network components
// - initialize storage components
// - create and add configured engine instances to server
// - create and configure main grpc server to handle requests
// - register actions to be taken on receipt of events
// - start listening server and processing loops
//
func Start(log *logging.LeveledLogger, cfg *config.Server) error {
	faultDomain, err := processConfig(log, cfg)
	if err != nil {
		return err
	}

	// Create the root context here. All contexts should inherit from this one so
	// that they can be shut down from one place.
	ctx, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	srv, err := newServer(ctx, log, cfg, faultDomain)
	if err != nil {
		return err
	}
	defer srv.shutdown()

	if err := srv.createServices(ctx); err != nil {
		return err
	}

	if err := srv.initNetwork(ctx); err != nil {
		return err
	}

	if err := srv.initStorage(); err != nil {
		return err
	}

	if err := srv.addEngines(ctx); err != nil {
		return err
	}

	if err := srv.setupGrpc(); err != nil {
		return err
	}

	srv.registerEvents()

	return srv.start(ctx, shutdown)
}

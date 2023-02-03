package main

import (
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/nightlyone/lockfile"
  "context"
  "flag"
  "fmt"
  "log"
  "time"
  "os"
)

var (
  // Network flags
	addrFlag     = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to host the service")
	rendezvousFlag     = flag.String("rendezvous", "peerprint-registry-v0.1", "String to use for discovering registry peers")
	localFlag          = flag.Bool("local", true, "Use local MDNS (instead of global DHT) for discovery")

  // Data flags
  dbPathFlag = flag.String("db", ":memory:", "Path to database (use :memory: for ephemeral, inmemory DB")
  lockPathFlag = flag.String("lockfile", "/tmp/peerprint_registry.lock", "Path to lockfile for process exclusion")

  // Timing flags
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
  syncPeriodFlag = flag.Duration("syncPeriod", 10*time.Minute, "Time between syncing with peers to correct missed data")

  logger = log.New(os.Stderr, "", 0)
)

func main() {
  flag.Parse()
	if *rendezvousFlag == "" {
		panic("-rendezvous must be specified!")
	}
  var lf lockfile.Lockfile
  var err error
  if *lockPathFlag != "" {
    lf, err = lockfile.New(*lockPathFlag)
    if err != nil {
      panic(err)
    } else if err = lf.TryLock(); err != nil {
      panic(err)
    }
    defer lf.Unlock()
  }

  var st storage.Registry
  st, err = storage.NewRegistry(*dbPathFlag)
  if err != nil {
    panic(fmt.Errorf("Error initializing DB: %w", err))
  }
  kpriv, kpub, err := crypto.GenKeyPair()

  ctx := context.Background()
  t, err := transport.New(&transport.Opts{
    Addr: *addrFlag,
    OnlyRPC: true,
    Rendezvous: *rendezvousFlag,
    Local: *localFlag,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: nil,
    ConnectTimeout: *connectTimeoutFlag,
  }, ctx, logger)
  if err != nil {
    panic(fmt.Errorf("Error initializing transport layer: %w", err))
  }
  s := server.NewRegistry(t, st, *syncPeriodFlag, pplog.New("server", logger))
  s.Run(ctx)
}


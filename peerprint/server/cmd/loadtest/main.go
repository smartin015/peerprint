package main

import (
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
	"github.com/libp2p/go-libp2p/core/peer"
  "context"
  "flag"
  "fmt"
  "log"
  "time"
  "os"
  "path/filepath"
)

var (
	fakeFlag = flag.Bool("fake", false, "Use a fake server that always returns success; for testing the loadtester")
	numServersFlag = flag.Int("servers", 2, "Number of servers to run")
  numRecordsFlag = flag.Int("records", 10, "Number of records to fill the database with")
  qpsFlag = flag.Float64("qps", 4, "Number of record-mutating operations per second, in aggregate")
  durationFlag = flag.Duration("duration", 1*time.Minute, "length of time to run the test, including initial connection time")
  logger = log.New(os.Stderr, "", 0)
)

func dlog(fmt string, args ...any) {
  // Magenta, then fmt string, then reset colors
  logger.Printf("\u001b[35m" + fmt + "\u001b[0m", args...)
}

func main() {
  flag.Parse()
  servers := []server.Interface{}

  dataDir, err := os.MkdirTemp("", "loadtest-*")
  if err != nil {
    panic(fmt.Errorf("MkdirTemp: %w", err))
  } else {
    dlog("Using temporary directory for sqlite databases: %s", dataDir)
  }
  for i := 0; i < *numServersFlag; i++ {
    kpriv, kpub, err := crypto.GenKeyPair()
    if err != nil {
      panic(fmt.Errorf("Error generating keys: %w", err))
    }
    id, err := peer.IDFromPublicKey(kpub)
    if err != nil {
      panic(fmt.Errorf("IDFromPublicKey: %w", err))
    }
    name := id.Pretty()
    name = name[len(name)-4:]

    if *fakeFlag {
      servers = append(servers, &fakeServer{Id: name})
      continue
    }

    st, err := storage.NewSqlite3(filepath.Join(dataDir, name))
    if err != nil {
      panic(fmt.Errorf("Error initializing DB: %w", err))
    }

    t, err := transport.New(&transport.Opts{
      PubsubAddr: "/ip4/127.0.0.1/tcp/0",
      Rendezvous: "testing",
      Local: true,
      PrivKey: kpriv,
      PubKey: kpub,
      ConnectTimeout: 5 * time.Second,
      Topics: []string{server.DefaultTopic, server.StatusTopic},
    }, context.Background(), logger)
    if err != nil {
      panic(fmt.Errorf("Error initializing transport layer: %w", err))
    }

    s := server.New(t, st, &server.Opts{
      StatusPeriod: 1*time.Minute,
      AccessionDelay: 5*time.Second,
    }, pplog.New(name, logger))
    if err := t.Register(server.PeerPrintProtocol, s.GetService()); err != nil {
      panic(fmt.Errorf("Failed to register RPC server: %w", err))
    }
    servers = append(servers, s)
  }

  dlog("Created %d servers; starting load test driver", *numServersFlag)
  d := NewDriver(servers, *numRecordsFlag)
  d.Run(*durationFlag, *qpsFlag)

  dlog("Verifying consistent state")
  d.Verify()
}

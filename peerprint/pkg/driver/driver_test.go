package driver

import (
  "testing"
	"github.com/libp2p/go-libp2p/core/peer"
  "google.golang.org/protobuf/proto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "path/filepath"
  "sync"
  "log"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/crawl"
  "github.com/smartin015/peerprint/p2pgit/pkg/cmd"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "time"
  "context"
  "os"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
)

var logger = log.New(os.Stderr, "", 0)

func makePeerID() string {
  kpriv, _, err := crypto.GenKeyPair()
  if err != nil {
    panic(err)
  }
  id, err := peer.IDFromPrivateKey(kpriv)
  if err != nil {
    panic(err)
  }
  return id.String()
}

func newTestDriver(t *testing.T) (*Driver, context.Context, context.CancelFunc) {
  ctx, done := context.WithTimeout(context.Background(), 60*time.Second)
  kpriv, kpub, err := crypto.GenKeyPair()
  if err != nil {
    t.Fatalf(err)
  }
  st, err := storage.NewSqlite3(":memory:")
  if err != nil {
    t.Fatalf(err)
  }
  t, err := transport.New(&transport.Opts{
    PubsubAddr: "/ip4/127.0.0.1/tcp/0",
    Rendezvous: "drivertest",
    Local: true,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: crypto.LoadPSK("12345"),
    ConnectTimeout: 10*time.Second,
    Topics: []string{server.DefaultTopic},
  }, ctx, logger)
  if err != nil {
    t.Fatalf(err)
  }
  srv := server.New(t, st, &server.Opts{
    SyncPeriod: 10*time.Minute,
    DisplayName: "srv",
    MaxRecordsPerPeer: 1000,
    MaxTrackedPeers: 1000,
  }, pplog.New("srv", logger))

  d := NewDriver(srv, st, t, pplog.New("cmd", logger))
  t.Cleanup(d.Destroy)
  go d.Loop(ctx)
  return d, ctx
}

func TestContext(t *testing.T) {
  d, ctx := newTestDriver()
}


package driver

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "testing"
  "path/filepath"
  "log"
  "github.com/smartin015/peerprint/p2pgit/pkg/registry"
  "context"
  "os"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
)

var logger = log.New(os.Stderr, "", 0)

func newTestDriver(t *testing.T) (*Driver) {
  dir := t.TempDir()
  ctx, done := context.WithCancel(context.Background())
  t.Cleanup(done)
  rlocal, err := registry.New(ctx, ":memory:", true, pplog.New("rLocal", logger))
  if err != nil {
    panic(err)
  }
  rworld, err := registry.New(ctx, ":memory:", true, pplog.New("rWorld", logger))
  if err != nil {
    panic(err)
  }

  d := New(&Opts{
    Addr: "/ip4/127.0.0.1/tcp/0",
    CertsDir: filepath.Join(dir, "certs"),
    ServerCert: "srv.crt",
    ServerKey: "srv.key",
    RootCert: "root.crt",
    ConfigPath: filepath.Join(dir, "config.yaml"),
  }, rlocal, rworld, pplog.New("cmd", logger))
  t.Cleanup(d.Destroy)
  go d.Loop(ctx, false)
  return d
}

func TestHandleConnectDisconnectAndGetters(t *testing.T) {
  d := newTestDriver(t)

  if cc := d.GetConnections(true); len(cc) != 0 {
    t.Errorf("Want 0 connections, got %v", cc)
  }
  if i := d.GetInstance("foo"); i != nil {
    t.Errorf("Want nil instance, got %v", i)
  }

  dir := t.TempDir()
  if err := d.handleConnect(&pb.ConnectRequest{
    Network: "foo",
    Addr: "/ip4/127.0.0.1/tcp/0",
    Rendezvous: "foo",
    Psk: "secret",
    Local: true,
    DbPath: filepath.Join(dir, "foo.sqlite3"),
    PrivkeyPath: filepath.Join(dir, "pubkey"),
    PubkeyPath: filepath.Join(dir, "privkey"),
    ConnectTimeout: 60,
    SyncPeriod: 100,
    MaxRecordsPerPeer: 10,
    MaxTrackedPeers: 10,
  }); err != nil {
    t.Fatalf("Connect error: %v", err)
  }

  if cc := d.GetConnections(true); len(cc) != 1 {
    t.Errorf("Want 1 connection, got %d: %v", len(cc), cc)
  }
  if i := d.GetInstance("foo"); i == nil {
    t.Errorf("Want instance, got nil")
  }

  if err := d.handleDisconnect(&pb.DisconnectRequest{
    Network: "foo",
  }); err != nil {
    t.Fatalf("Disconnect error: %v", err)
  }

  if cc := d.GetConnections(true); len(cc) != 0 {
    t.Errorf("Want 0 connections, got %v", cc)
  }
  if i := d.GetInstance("foo"); i != nil {
    t.Errorf("Want nil instance, got %v", i)
  }
}


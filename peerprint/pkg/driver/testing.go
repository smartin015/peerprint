package driver

import (
  "os"
  "log"
  "testing"
  "path/filepath"
  "github.com/smartin015/peerprint/p2pgit/pkg/registry"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "context"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
)

func NewTestDriver(t *testing.T) *Driver {
  return NewTestDriverImpl(t, false)
}
func NewTestDriverWithLAN(t *testing.T) *Driver {
  return NewTestDriverImpl(t, true)
}

func writeTestCerts(dir string) {
  os.Mkdir(dir, 0777)

  ca, cab, cak, err := crypto.SelfSignedCACert("CA")
  if err != nil {
    panic(err)
  }
  if err := crypto.WritePEM(cab, cak, filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.priv")); err != nil {
    panic(err)
  }
  _, cb, ck, err := crypto.CASignedCert(ca, cak, "SubCert")
  if err != nil {
    panic(err)
  }
  if err := crypto.WritePEM(cb, ck, filepath.Join(dir, "srv.crt"), filepath.Join(dir, "srv.key")); err != nil {
    panic(err)
  }
}

func NewTestDriverImpl(t *testing.T, defaultLAN bool) *Driver {
  dir := t.TempDir()
  ctx, done := context.WithCancel(context.Background())
  t.Cleanup(done)
  rlocal, err := registry.New(ctx, ":memory:", true, pplog.New("rLocal", log.Default()))
  if err != nil {
    panic(err)
  }
  rworld, err := registry.New(ctx, ":memory:", true, pplog.New("rWorld", log.Default()))
  if err != nil {
    panic(err)
  }

  certsDir := filepath.Join(dir, "certs")
  if defaultLAN {
    writeTestCerts(certsDir)
  }

  d := New(&Opts{
    RPCAddr: "127.0.0.1:0",
    CertsDir: certsDir,
    ServerCert: "srv.crt",
    ServerKey: "srv.key",
    RootCert: "ca.crt",
    ConfigPath: filepath.Join(dir, "config.yaml"),
  }, rlocal, rworld, pplog.New("cmd", log.Default()))
  t.Cleanup(d.Destroy)
  if err := d.Start(ctx, defaultLAN); err != nil {
    panic(err)
  }
  return d
}

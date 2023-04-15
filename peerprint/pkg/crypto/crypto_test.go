package crypto 

import (
  "testing"
  "path/filepath"
  "crypto/tls"
	"github.com/libp2p/go-libp2p/core/crypto"
)

func TestGenKeyPairFileAndLoadKeys(t *testing.T) {
  dir := t.TempDir()
  privpath := filepath.Join(dir, "key.priv")
  pubpath := filepath.Join(dir, "key.pub")
  kpriv, kpub, err := GenKeyPairFile(privpath, pubpath)
  if err != nil {
    t.Error(err)
  }
  gpriv, gpub, err := LoadKeys(privpath, pubpath)
  if err != nil || !crypto.KeyEqual(gpriv, kpriv) || !crypto.KeyEqual(gpub, kpub) {
    t.Errorf("got %v, %v, %v; want %v, %v, nil", gpriv, gpub, err, kpriv, kpub)
  }
}

func TestGenCertsWriteReload(t *testing.T) {
  dir := t.TempDir()

  ca, cab, cak, err := SelfSignedCACert("CA")
  if err != nil {
    t.Errorf("gen CA cert: %v", err)
  }

  if err := WritePEM(cab, cak, filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.priv")); err != nil {
    t.Errorf("write CA PEM: %v", err)
  }

  _, cb, ck, err := CASignedCert(ca, cak, "SubCert")
  if err != nil {
    t.Errorf("gen sub cert: %v", err)
  }

  if err := WritePEM(cb, ck, filepath.Join(dir, "c.crt"), filepath.Join(dir, "c.priv")); err != nil {
    t.Errorf("write CA PEM: %v", err)
  }

  // Now read them in to ensure valid
  if _, err  := tls.LoadX509KeyPair(filepath.Join(dir, "ca.crt"), filepath.Join(dir, "ca.priv")); err != nil {
    t.Errorf("reload CA cert: %v", err)
  }
  if _, err  := tls.LoadX509KeyPair(filepath.Join(dir, "c.crt"), filepath.Join(dir, "c.priv")); err != nil {
    t.Errorf("reload sub cert: %v", err)
  }

  // And try creating a TLS config
  if _, err := NewTLSConfig(dir, "ca.crt", "c.crt", "c.priv"); err != nil {
    t.Errorf("NewTLSConfig error: %v", err)
  }
}

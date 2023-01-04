package crypto 

import (
  "testing"
  "path/filepath"
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

package registry_generator

import (
  "testing"
  "path/filepath"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
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

func TestGenRegistryFiles(t *testing.T) {
  dir := t.TempDir()
  if err := GenRegistryFiles(1, dir); err != nil {
    t.Errorf("GenRegistryFiles err: %v", err)
  }

  _, gotpub, err := LoadKeys(
    filepath.Join(dir, "trusted_peer_1.priv"),
    filepath.Join(dir, "trusted_peer_1.pub"),
  )
  if err != nil {
    t.Errorf("Error loading generated trusted peer key: %v", err)
  }

	got, err := LoadRegistry(filepath.Join(dir, "registry.yaml"))
  if err != nil {
    t.Errorf("Error loading registry: %v", err)
  }

  if id, err := peer.IDFromPublicKey(gotpub); err != nil || id.String() != got.GetQueues()[0].GetTrustedPeers()[0] {
    t.Errorf("Generated TrustedPeer ID mismatch vs generated public key (possibly error %v): want %v got %v", err, id, got.GetQueues()[0].GetTrustedPeers()[0])
  }
}

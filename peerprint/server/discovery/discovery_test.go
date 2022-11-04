package discovery

import (
  "time"
  "sort"
  "context"
  "testing"
  "log"
  "github.com/libp2p/go-libp2p"
  "github.com/libp2p/go-libp2p/core/host"
  "github.com/libp2p/go-libp2p/core/peer"
)

const rendezvous = "test_rendezvous"

func testHost(t *testing.T) (host.Host) {
  t.Helper()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
  if err != nil {
    t.Fatal(err)
  }
	t.Cleanup(func() {
		h.Close()
    h.ConnManager().Close()
	})
  return h
}

func newDiscovery(t *testing.T, h host.Host, m Method) (*Discovery) {
  t.Helper()
  return New(context.Background(), m, h, rendezvous, log.Default())
}

func assertHasPeers(t *testing.T, h host.Host, peers []host.Host) {
  t.Helper()
  want := []string{h.ID().String()}
  for _, p := range peers {
    want = append(want, p.ID().String())
  }
  got := []string{}
  for _, p := range h.Peerstore().Peers() {
    got = append(got, p.String())
  }

  sort.Strings(want)
  sort.Strings(got)

  for _, w := range want {
    found := false
    for _, g := range got {
      found = found || w == g
    }
    if !found {
      t.Errorf("Peer mismatch: %+v not found in %+v", w, got)
    }
  }
}

func getAddrInfo(p host.Host) peer.AddrInfo {
  return peer.AddrInfo {
    ID: p.ID(),
    Addrs: p.Addrs(),
  }
}

func TestNewLocal(t *testing.T) {
  SetBootstrapPeers([]peer.AddrInfo{}) // No need to dial out
  peers := []host.Host{testHost(t), testHost(t), testHost(t)}
  h := testHost(t)
  d := newDiscovery(t, h, MDNS)
  d.HandlePeerFound(getAddrInfo(peers[0]))
  d.HandlePeerFound(getAddrInfo(peers[1]))
  d.HandlePeerFound(getAddrInfo(peers[2]))


  // Don't actually need to wait long, as HandlePeerFound already triggered
  ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Millisecond)
  t.Cleanup(cancel)
  if err := d.AwaitReady(ctx); err == nil {
    t.Errorf("Expected context cancel on awaitready")
  }
  assertHasPeers(t, h, peers)
}
func TestNewDHT(t *testing.T) {
  SetBootstrapPeers([]peer.AddrInfo{}) // No need to dial out
  peers := []host.Host{testHost(t), testHost(t), testHost(t)}
  h := testHost(t)
  d := newDiscovery(t, h, DHT)
  d.HandlePeerFound(getAddrInfo(peers[0]))
  d.HandlePeerFound(getAddrInfo(peers[1]))
  d.HandlePeerFound(getAddrInfo(peers[2]))

  // Don't actually need to wait long, as HandlePeerFound already triggered
  ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Millisecond)
  t.Cleanup(cancel)
  if err := d.AwaitReady(ctx); err == nil {
    t.Errorf("Expected context cancel on awaitready")
  }
  assertHasPeers(t, h, peers)
}

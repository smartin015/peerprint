package driver

import (
  "testing"
  "log"
  "context"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/crawl"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
)

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

func testInstance(t *testing.T) *Instance {
  inst, err := NewInstance(testNetConnReq(t), pplog.New("instance", log.Default()))
  if err != nil {
    panic(err)
  }
  return inst
}

func TestCrawlExcessiveMultiAddr(t *testing.T) {
  // Prevent malicious peers from overwhelming us with addresses
  d := testInstance(t)
  addrs := []*pb.AddrInfo{}
  for i := 0; i < 2*MaxAddrPerPeer; i++ {
    addrs = append(addrs, &pb.AddrInfo{Id: makePeerID(), Addrs: []string{"/ip4/127.0.0.1/tcp/12345"}})
  }

  got := d.handleGetPeersResponse(&pb.GetPeersResponse{Addresses: addrs})
  if got == nil || len(got) != MaxAddrPerPeer {
    t.Errorf("Expected len(addrs)=%d, got %d", MaxAddrPerPeer, len(got))
  }
}

func TestCrawlPeerCyclesPrevented(t *testing.T) {
  d := testInstance(t)
  d.c = crawl.NewCrawler(d.t.GetPeerAddresses(), d.crawlPeer)
  p := &peer.AddrInfo{
    ID: "foo",
  }
  if err := d.St.LogPeerCrawl(p.ID.String(), d.c.Started.Unix()); err != nil {
    t.Fatal(err)
  }
  if got, err := d.crawlPeer(context.Background(), p); len(got) != 0 || err != nil {
    t.Errorf("crawlPeer of already crawled peer - got %v, %v want 0-len, nil", got, err)
  }
}


package crawl

import (
  "testing"
  "context"
	"github.com/libp2p/go-libp2p/core/peer"
  "github.com/google/uuid"
)

func testAI() *peer.AddrInfo {
  return &peer.AddrInfo{
    ID: peer.ID(uuid.New().String()),
  }
}

func TestOneHop(t *testing.T) {
  a1 := testAI()
  a2 := testAI()
  got := 0
  c:= NewCrawler([]*peer.AddrInfo{a1}, func(ctx context.Context, ai *peer.AddrInfo)   []*peer.AddrInfo {
    got += 1
    return []*peer.AddrInfo{a2}
  })
  if cont := c.Step(context.Background(), 100); cont == 0 {
    t.Errorf("Expected c.Step != 0, got %d", cont)
  }
  if want := 1; got != want {
    t.Errorf("num peer RPCs = %d, want %d", got, want)
  }
}

func TestMultiHop(t *testing.T) {
  aa := []*peer.AddrInfo{}
  for i := 0; i < 10; i++ {
    aa = append(aa, testAI())
  }
  got := 0
  peerChain := func(ctx context.Context, ai *peer.AddrInfo) []*peer.AddrInfo {
    got += 1
    for i := 0; i < len(aa)-1; i++ {
      if (aa[i].ID == ai.ID) {
        return []*peer.AddrInfo{aa[i+1]}
      }
    }
    return []*peer.AddrInfo{}
  }

  c:= NewCrawler([]*peer.AddrInfo{aa[0]}, peerChain)
  for i := 0; i < 4; i++ {
    if cont := c.Step(context.Background(), 100); cont == 0 {
      t.Errorf("got cont=%v on step %d", cont, i)
    }
  }
  // After 4 steps, should have starting node plus 4 peers discovered
  if want := 4; got != want {
    t.Errorf("RPCs = %d, want %d", got, want)
  }

  for i := 0; i < 20; i++ {
    c.Step(context.Background(), 100)
  }
  if cont := c.Step(context.Background(), 100); cont != 0 {
    t.Errorf("got cont=%v, want 0", cont)
  }
  if want := len(aa); got != want {
    t.Errorf("RPCs = %d, want %d", got, want)
  }
}

func TestFanOutBatched(t *testing.T) {
  aa := make(map[string][]*peer.AddrInfo)
  a1 := testAI()
  aa[a1.ID.String()] = []*peer.AddrInfo{}
  got := 0
  for i := 0; i < 10; i++ {
    aa[a1.ID.String()] = append(aa[a1.ID.String()], testAI())
    aa[aa[a1.ID.String()][i].ID.String()] = []*peer.AddrInfo{}
    for j := 0; j < 10; j++ {
      aa[aa[a1.ID.String()][i].ID.String()] = append(aa[aa[a1.ID.String()][i].ID.String()], testAI())
    }
  }
  peerChain := func(ctx context.Context, ai *peer.AddrInfo) []*peer.AddrInfo {
    got += 1
    return aa[ai.ID.String()]
  }

  c:= NewCrawler([]*peer.AddrInfo{a1}, peerChain)
  c.Step(context.Background(), 20) // Query start node
  c.Step(context.Background(), 20)
  // Only direct peers of start node
  if want := 11; got != want {
    t.Errorf("partial NumPeersDiscovered = %d, want %d", got, want)
  }

  c.Step(context.Background(), 5)
  // Only 5 new peers crawled
  if want := 16; got != want {
    t.Errorf("batch-bounded NumPeersDiscovered = %d, want %d", got, want)
  }

  if cont := c.Step(context.Background(), 100); cont == 0 {
    t.Errorf("Want cont != 0") // We haven't crawled the new peers yet
  }
  // All remaining peers discovered
  if want := 1 + 10 + 10*10; got != want {
    t.Errorf("NumPeersDiscovered = %d, want %d", got, want)
  }

  if cont := c.Step(context.Background(), 100); cont != 0 {
    t.Errorf("Want cont = 0")
  }
}

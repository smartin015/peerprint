package server

import (
  "testing"
  "context"
  "github.com/google/uuid"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
)

func testAI() *pb.AddrInfo {
  return &pb.AddrInfo{
      Id: uuid.New().String(),
      Addrs: []string{},
    }
}

func TestExcessiveMultiAddr(t *testing.T) {
  // TODO
}

func TestOneHop(t *testing.T) {
  a1 := testAI()
  a2:= testAI()
  c:= NewCrawl([]*pb.AddrInfo{a1}, func(ctx context.Context, ai *pb.AddrInfo) *pb.GetPeersResponse{
    return &pb.GetPeersResponse{
      Addresses: []*pb.AddrInfo{a2},
    }
  })
  c.Step(context.Background(), 100)
  want := 2
  if got := c.NumPeersDiscovered(); got != want {
    t.Errorf("NumPeersDiscovered = %d, want %d", got, want)
  }
}

func TestMultiHop(t *testing.T) {
  aa := []*pb.AddrInfo{}
  for i := 0; i < 10; i++ {
    aa = append(aa, testAI())
  }
  peerChain := func(ctx context.Context, ai *pb.AddrInfo) *pb.GetPeersResponse{
    for i := 0; i < len(aa)-1; i++ {
      if (aa[i] == ai) {
        return &pb.GetPeersResponse{
          Addresses: []*pb.AddrInfo{aa[i+1]},
        }
      }
    }
    return &pb.GetPeersResponse{
      Addresses: []*pb.AddrInfo{},
    }
  }

  c:= NewCrawl([]*pb.AddrInfo{aa[0]}, peerChain)
  for i := 0; i < 4; i++ {
    c.Step(context.Background(), 100)
  }
  want := 5 // After 4 steps, should have starting node plus 4 peers discovered
  if got := c.NumPeersDiscovered(); got != want {
    t.Errorf("partial NumPeersDiscovered = %d, want %d", got, want)
  }

  for i := 0; i < 20; i++ {
    c.Step(context.Background(), 100)
  }
  want = len(aa)
  if got := c.NumPeersDiscovered(); got != want {
    t.Errorf("NumPeersDiscovered = %d, want %d", got, want)
  }
}


func TestFanOutBatched(t *testing.T) {
  aa := make(map[string][]*pb.AddrInfo)
  a1 := testAI()
  aa[a1.Id] = []*pb.AddrInfo{}
  for i := 0; i < 10; i++ {
    aa[a1.Id] = append(aa[a1.Id], testAI())
    aa[aa[a1.Id][i].Id] = []*pb.AddrInfo{}
    for j := 0; j < 10; j++ {
      aa[aa[a1.Id][i].Id] = append(aa[aa[a1.Id][i].Id], testAI())
    }
  }
  peerChain := func(ctx context.Context, ai *pb.AddrInfo) *pb.GetPeersResponse{
    return &pb.GetPeersResponse{
      Addresses: aa[ai.Id],
    }
  }

  c:= NewCrawl([]*pb.AddrInfo{a1}, peerChain)
  c.Step(context.Background(), 20)
  want := 11 // Only direct peers of start node
  if got := c.NumPeersDiscovered(); got != want {
    t.Errorf("partial NumPeersDiscovered = %d, want %d", got, want)
  }

  c.Step(context.Background(), 5)
  want += 5*10 // Only the peers of 5 peers crawled
  if got := c.NumPeersDiscovered(); got != want {
    t.Errorf("batch-bounded NumPeersDiscovered = %d, want %d", got, want)
  }

  c.Step(context.Background(), 100)
  want = 1 + 10 + 10*10 // All remaining peers discovered
  if got := c.NumPeersDiscovered(); got != want {
    t.Errorf("NumPeersDiscovered = %d, want %d", got, want)
  }
}

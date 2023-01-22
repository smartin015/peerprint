package server

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "sync"
  "context"
)

type crawlPeerFn func(ctx context.Context, ai *pb.AddrInfo) *pb.GetPeersResponse

type crawl struct {
  crawlPeer crawlPeerFn
  peers map[string]*pb.AddrInfo
  fresh map[string]*pb.AddrInfo
}

func (s *Server) crawlPeer(ctx context.Context, ai *pb.AddrInfo) *pb.GetPeersResponse {
  rep := &pb.GetPeersResponse{}
  pai, err := transport.ProtoToPeerAddrInfo(ai)
  if err != nil {
    return rep
  }
  s.t.AddTempPeer(pai)
  if err := s.t.Call(ctx, pai.ID, "GetPeers", &pb.GetPeersRequest{}, rep); err != nil {
    s.l.Error("GetPeers of %s: %v", shorten(ai.Id), err)
    return rep
  } else {
    return rep
  }
}

func NewCrawl(start []*pb.AddrInfo, cpf crawlPeerFn) *crawl {
  c := &crawl{
    crawlPeer: cpf,
    peers: make(map[string]*pb.AddrInfo),
    fresh: make(map[string]*pb.AddrInfo),
  }
  for _, p := range start {
    c.peers[p.Id] = p
    c.fresh[p.Id] = p
  }
  return c
}

func (c *crawl) Step(ctx context.Context, maxConn int64) {
  var wg sync.WaitGroup
  ncon := int64(0)
  for _, p := range c.fresh {
    ncon += 1
    if ncon > maxConn {
      continue
    }
    defer delete(c.fresh, p.Id)
    wg.Add(1)
    go func(p *pb.AddrInfo) {
      defer wg.Done()
      for _, a := range c.crawlPeer(ctx, p).Addresses {
        if _, ok := c.peers[a.Id]; !ok {
          c.peers[a.Id] = a
          c.fresh[a.Id] = a
        }
      }
    }(p)
  }
  wg.Wait()
}

func (c *crawl) NumPeersDiscovered() int {
  return len(c.peers)
}

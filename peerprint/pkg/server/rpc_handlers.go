package server

import (
  "context"
  "sync"
  "fmt"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/smartin015/peerprint/p2pgit/pkg/transport"
	"github.com/smartin015/peerprint/p2pgit/pkg/storage"
)

type PeerPrintService struct {
  base *Server
}

func (s *Server) getService() *PeerPrintService {
  return &PeerPrintService{
    base: s,
  }
}

func (s *PeerPrintService) GetSignedRecords(ctx context.Context, reqChan <-chan string, repChan chan<- *pb.SignedRecord) error {
  select {
  case peer := <- reqChan:
    return s.base.s.GetSignedRecords(ctx, repChan, storage.WithLimit(1000), storage.WithSigners([]string{s.base.ID(), peer})) // repChan closed by impl
  case <- ctx.Done():
    return fmt.Errorf("Context timeout")
  }
}

func (s *PeerPrintService) GetSignedCompletions(ctx context.Context, reqChan <-chan string, repChan chan<- *pb.SignedCompletion) error {
  select {
  case peer := <- reqChan:
    return s.base.s.GetSignedCompletions(ctx, repChan, storage.WithSigners([]string{s.base.ID(), peer}), storage.WithLimit(1000)) // repChan closed by impl
  case <- ctx.Done():
    return fmt.Errorf("Context timeout")
  }
}

func (s *PeerPrintService) GetPeers(ctx context.Context, req *pb.GetPeersRequest, rep *pb.GetPeersResponse) error {
  for _, a := range s.base.t.GetPeerAddresses() {
    rep.Addresses = append(rep.Addresses, transport.PeerToProtoAddrInfo(a))
  }
  return nil
}

func (s *PeerPrintService) GetStatus(ctx context.Context, req *pb.GetStatusRequest, rep *pb.PeerStatus) error {
  cur := make(chan *pb.PeerStatus)
  var wg sync.WaitGroup
  wg.Add(1)
  go func () {
    defer wg.Done()
    ps, ok := <-cur
    if ok && ps != nil {
      *rep = *ps
    }
  }()
  err := s.base.s.GetPeerStatuses(ctx, cur, storage.WithSigners([]string{s.base.ID()}), storage.WithLimit(1)) // cur closed by impl
  wg.Wait()
  return err
}

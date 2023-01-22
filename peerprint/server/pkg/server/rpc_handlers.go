package server

import (
  "context"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/smartin015/peerprint/p2pgit/pkg/transport"
)

type PeerPrintService struct {
  base *Server
}

func (s *Server) GetService() interface{} {
  return &PeerPrintService{
    base: s,
  }
}

func (s *PeerPrintService) GetSignedRecords(ctx context.Context, reqChan <-chan struct{}, repChan chan<- *pb.SignedRecord) error {
  defer close(repChan)
  return s.base.s.GetSignedRecords(ctx, repChan)
}

func (s *PeerPrintService) GetSignedCompletions(ctx context.Context, reqChan <-chan struct{}, repChan chan<- *pb.SignedCompletion) error {
  defer close(repChan)
  return s.base.s.GetSignedCompletions(ctx, repChan)
}

func (s *PeerPrintService) GetPeers(ctx context.Context, req *pb.GetPeersRequest, rep *pb.GetPeersResponse) error {
  for _, a := range s.base.t.GetPeerAddresses() {
    rep.Addresses = append(rep.Addresses, transport.PeerToProtoAddrInfo(&a))
  }
  return nil
}

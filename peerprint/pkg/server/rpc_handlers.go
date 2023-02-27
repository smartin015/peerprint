package server

import (
  "context"
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

func (s *PeerPrintService) GetSignedRecords(ctx context.Context, reqChan <-chan struct{}, repChan chan<- *pb.SignedRecord) error {
  return s.base.s.GetSignedRecords(ctx, repChan, storage.WithLimit(1000), storage.WithSigner(s.base.ID())) // repChan closed by impl
}

func (s *PeerPrintService) GetSignedCompletions(ctx context.Context, reqChan <-chan struct{}, repChan chan<- *pb.SignedCompletion) error {
  return s.base.s.GetSignedCompletions(ctx, repChan, storage.WithSigner(s.base.ID()), storage.WithLimit(1000)) // repChan closed by impl
}

func (s *PeerPrintService) GetPeers(ctx context.Context, req *pb.GetPeersRequest, rep *pb.GetPeersResponse) error {
  for _, a := range s.base.t.GetPeerAddresses() {
    rep.Addresses = append(rep.Addresses, transport.PeerToProtoAddrInfo(a))
  }
  return nil
}

func (s *PeerPrintService) GetStatus(ctx context.Context, req *pb.GetStatusRequest, rep *pb.PeerStatus) error {
  rep.Name = s.base.opts.DisplayName
  rep.Printers = []*pb.PrinterStatus{}
  for _, ps := range s.base.printerStatuses {
    rep.Printers = append(rep.Printers, ps)
  }
  return nil
}

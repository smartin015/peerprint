package server

import (
  "context"
  pb "github.com/smartin015/peerprint/p2pgit/proto"
)

type PeerPrintService struct {
  base *Server
}

func (s *PeerPrintService) GetState(ctx context.Context, req *pb.GetStateRequest, rep *pb.GetStateResponse) error {
  if g, err := s.base.s.GetSignedGrants(); err != nil {
    return err
  } else {
    rep.Grants = g
  }
  if r, err := s.base.s.GetSignedRecords(); err != nil {
    return err
  } else {
    rep.Records = r
  }
  return nil
}


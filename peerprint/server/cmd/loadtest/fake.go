package main

import (

  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "context"
)

type fakeServer struct {
  Id string
}


func (s *fakeServer) ID() string {
  return s.Id
}
func (s *fakeServer) ShortID() string {
  return s.Id
}

func (s *fakeServer) GetService() interface{} {
  return nil
}

func (s *fakeServer) Run(ctx context.Context) {
  return
}

func (s *fakeServer) SetRecord(r *pb.Record) error {
  return nil
}

func (s *fakeServer) SetGrant(g *pb.Grant) error {
  return nil
}

func (s *fakeServer) WaitUntilReady() {
}

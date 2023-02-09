package driver

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "google.golang.org/grpc/peer"
  "google.golang.org/grpc/status"
  "google.golang.org/grpc/codes"
  "google.golang.org/grpc/credentials"
  "context"
  "sync"
)

type commandServer struct {
  pb.UnimplementedCommandServer
  d *Driver
}

func newCmdServer(d *Driver) *commandServer {
  return &commandServer{d:d}
}

func verifyPeer(ctx context.Context) error {
  p, ok := peer.FromContext(ctx)
  if !ok {
    return status.Error(codes.Unauthenticated, "no peer found")
  }
  tlsAuth, ok := p.AuthInfo.(credentials.TLSInfo)
  if !ok {
    return status.Error(codes.Unauthenticated, "unexpected peer transport credentials")
  }

  if len(tlsAuth.State.VerifiedChains) == 0 || len(tlsAuth.State.VerifiedChains[0]) == 0 {
    return status.Error(codes.Unauthenticated, "could not verify peer certificate")
  }

  return nil
}


func (s *commandServer) Ping(ctx context.Context, req *pb.HealthCheck) (*pb.Ok, error) {
  if err := verifyPeer(ctx); err != nil {
    return nil, err
  }
  return &pb.Ok{}, nil
}

func (s *commandServer) GetId(ctx context.Context, req *pb.GetIDRequest) (*pb.GetIDResponse, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  return &pb.GetIDResponse{
    Id: inst.S.ID(),
  }, nil
}

func (s *commandServer) GetNetworks(ctx context.Context, req *pb.GetNetworksRequest) (*pb.GetNetworksResponse, error) {
  return &pb.GetNetworksResponse{
    Networks: s.d.GetConfigs(true),
  }, nil
}

func (s *commandServer) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.Ok, error) {
  if err := s.d.handleConnect(req); err != nil {
    return nil, status.Errorf(codes.Internal, "Connect: %w", err)
  } else {
    return &pb.Ok{}, nil
  }
}

func (s *commandServer) Disconnect(ctx context.Context, req *pb.DisconnectRequest) (*pb.Ok, error) {
  if err := s.d.handleDisconnect(req); err != nil {
    return nil, status.Errorf(codes.Internal, "Disconnect: %w", err)
  } else {
    return &pb.Ok{}, nil
  }
}

func (s *commandServer) SetRecord(ctx context.Context, req *pb.SetRecordRequest) (*pb.Ok, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  if _, err := inst.S.IssueRecord(req.Record, true); err != nil {
    return nil, status.Error(codes.Internal, err.Error())
  }
  return &pb.Ok{}, nil
}

func (s *commandServer) SetCompletion(ctx context.Context, req *pb.SetCompletionRequest) (*pb.Ok, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  if _, err := inst.S.IssueCompletion(req.Completion, true); err != nil {
    return nil, status.Error(codes.Internal, err.Error())
  }
  return &pb.Ok{}, nil
}

func (s *commandServer) Crawl(ctx context.Context, req *pb.CrawlRequest) (*pb.CrawlResult, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  return s.d.handleCrawl(ctx, inst, req)
}

func (s *commandServer) StreamEvents(req *pb.StreamEventsRequest, stream pb.Command_StreamEventsServer) error {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  c := make(chan *pb.Event)
  defer close(c)
  inst.S.RegisterEventCallback(c)
  for {
    select {
    case <-stream.Context().Done():
      return nil
    case v, ok := <-c:
      if !ok {
        return nil
      }
      if err := stream.Send(v); err != nil {
        return err
      }
    }
  }
}

func (s *commandServer) StreamRecords(req *pb.StreamRecordsRequest, stream pb.Command_StreamRecordsServer) error {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  ch := make(chan *pb.SignedRecord)
  var cherr error
  var wg sync.WaitGroup
  wg.Add(1)
  go func () {
    defer wg.Done()
    for v := range ch {
      if err := stream.Send(v); err != nil {
        cherr = err
        return
      }
    }
  }()
  if err := inst.St.GetSignedRecords(stream.Context(), ch); err != nil {
    return err
  }
  wg.Wait()
  return cherr
}

func (s *commandServer) StreamCompletions(req *pb.StreamCompletionsRequest, stream pb.Command_StreamCompletionsServer) error {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  ch := make(chan *pb.SignedCompletion)
  var cherr error
  var wg sync.WaitGroup
  wg.Add(1)
  go func () {
    defer wg.Done()
    for v := range ch {
      if err := stream.Send(v); err != nil {
        cherr = err
        return
      }
    }
  }()
  if err := inst.St.GetSignedCompletions(stream.Context(), ch); err != nil {
    return err
  }
  wg.Wait()
  return cherr
}

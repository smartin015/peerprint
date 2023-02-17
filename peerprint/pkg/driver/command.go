package driver

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/registry"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "google.golang.org/grpc/peer"
  "google.golang.org/grpc/status"
  "google.golang.org/grpc/codes"
  "google.golang.org/grpc/credentials"
  "context"
  "sync"
)

type CommandServer struct {
  pb.UnimplementedCommandServer
  d *Driver
}

func newCmdServer(d *Driver) *CommandServer {
  return &CommandServer{d:d}
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


func (s *CommandServer) Ping(ctx context.Context, req *pb.HealthCheck) (*pb.Ok, error) {
  if err := verifyPeer(ctx); err != nil {
    return nil, err
  }
  return &pb.Ok{}, nil
}

func (s *CommandServer) GetId(ctx context.Context, req *pb.GetIDRequest) (*pb.GetIDResponse, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  return &pb.GetIDResponse{
    Id: inst.S.ID(),
  }, nil
}

func (s *CommandServer) GetConnections(ctx context.Context, req *pb.GetConnectionsRequest) (*pb.GetConnectionsResponse, error) {
  return &pb.GetConnectionsResponse{
    Networks: s.d.GetConfigs(true),
  }, nil
}

func (s *CommandServer) SetStatus(ctx context.Context, req *pb.SetStatusRequest) (*pb.Ok, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  inst.S.SetStatus(req.Status)
  return &pb.Ok{}, nil
}

func (s *CommandServer) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.Ok, error) {
  if err := s.d.handleConnect(req); err != nil {
    return nil, status.Errorf(codes.Internal, "Connect: %w", err)
  } else {
    return &pb.Ok{}, nil
  }
}

func (s *CommandServer) Disconnect(ctx context.Context, req *pb.DisconnectRequest) (*pb.Ok, error) {
  if err := s.d.handleDisconnect(req); err != nil {
    return nil, status.Errorf(codes.Internal, "Disconnect: %w", err)
  } else {
    return &pb.Ok{}, nil
  }
}

func (s *CommandServer) SetRecord(ctx context.Context, req *pb.SetRecordRequest) (*pb.Ok, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  if _, err := inst.S.IssueRecord(req.Record, true); err != nil {
    return nil, status.Error(codes.Internal, err.Error())
  }
  return &pb.Ok{}, nil
}

func (s *CommandServer) SetCompletion(ctx context.Context, req *pb.SetCompletionRequest) (*pb.Ok, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  if _, err := inst.S.IssueCompletion(req.Completion, true); err != nil {
    return nil, status.Error(codes.Internal, err.Error())
  }
  return &pb.Ok{}, nil
}

func (s *CommandServer) Crawl(ctx context.Context, req *pb.CrawlRequest) (*pb.CrawlResult, error) {
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return nil, status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }
  return s.d.handleCrawl(ctx, inst, req)
}

func (s *CommandServer) StreamEvents(req *pb.StreamEventsRequest, stream pb.Command_StreamEventsServer) error {
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

func (s *CommandServer) StreamRecords(req *pb.StreamRecordsRequest, stream pb.Command_StreamRecordsServer) error {
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
  opts := []any{}
  if req.Uuid != "" {
    opts = append(opts, storage.WithUUID(req.Uuid))
  }
  if err := inst.St.GetSignedRecords(stream.Context(), ch, opts...); err != nil {
    return err
  }
  wg.Wait()
  return cherr
}

func (s *CommandServer) StreamCompletions(req *pb.StreamCompletionsRequest, stream pb.Command_StreamCompletionsServer) error {
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
  opts := []any{}
  if req.Uuid != "" {
    opts = append(opts, storage.WithUUID(req.Uuid))
  }
  if err := inst.St.GetSignedCompletions(stream.Context(), ch, opts...); err != nil {
    return err
  }
  wg.Wait()
  return cherr
}

func (s *CommandServer) ResolveRegistry(local bool) *registry.Registry {
  if local {
    return s.d.RLocal
  } else {
    return s.d.RWorld
  }
}

func (s *CommandServer) StreamNetworks(req *pb.StreamNetworksRequest, stream pb.Command_StreamNetworksServer) error {
  ch := make(chan *pb.Network)
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
  if err := s.ResolveRegistry(req.Local).DB.GetRegistry(stream.Context(), ch, storage.RegistryTable, true); err != nil {
    return err
  }
  wg.Wait()
  return cherr
}

func (s *CommandServer) Advertise(ctx context.Context, req *pb.AdvertiseRequest) (*pb.Ok, error) {
  if err := s.ResolveRegistry(req.Local).DB.UpsertConfig(req.Config, []byte(""), storage.RegistryTable); err != nil {
    return nil, status.Errorf(codes.Internal, err.Error())
  }
  return &pb.Ok{}, nil
}

func (s *CommandServer) StopAdvertising(ctx context.Context, req *pb.StopAdvertisingRequest) (*pb.Ok, error) {
  if err := s.ResolveRegistry(req.Local).DB.DeleteConfig(req.Uuid, storage.RegistryTable); err != nil {
    return nil, status.Errorf(codes.Internal, err.Error())
  }
  return &pb.Ok{}, nil
}

func (s *CommandServer) SyncLobby(ctx context.Context, req *pb.SyncLobbyRequest) (*pb.Ok, error) {
  return nil, status.Errorf(codes.Unimplemented, "TODO restart registries")
}

func (s *CommandServer) StreamAdvertisements(req *pb.StreamAdvertisementsRequest, stream pb.Command_StreamAdvertisementsServer) error {
  ch := make(chan *pb.Network)
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
  if err := s.ResolveRegistry(req.Local).DB.GetRegistry(stream.Context(), ch, storage.LobbyTable, true); err != nil {
    return err
  }
  wg.Wait()
  return cherr
}


func (s *CommandServer) StreamPeers(req *pb.StreamPeersRequest, stream pb.Command_StreamPeersServer) error {
  s.d.l.Info("TODO handle StreamPeers")
  return nil
  /*
  inst, ok := s.d.inst[req.Network]
  if !ok {
    return status.Errorf(codes.InvalidArgument, "Network not found: %s", req.Network)
  }*/
}
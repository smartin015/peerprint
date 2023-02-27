package driver

import (
  "strings"
  "testing"
  "context"
  "path/filepath"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
)

const (
  TestNet = "testnet"
)

func testNetConnReq(t *testing.T) *pb.ConnectRequest {
  dir := t.TempDir()
  return &pb.ConnectRequest{
    Network: TestNet,
    Addr: "/ip4/127.0.0.1/tcp/0",
    Rendezvous: "foo",
    Psk: "secret",
    Local: true,
    DbPath: filepath.Join(dir, "foo.sqlite3"),
    PrivkeyPath: filepath.Join(dir, "pubkey"),
    PubkeyPath: filepath.Join(dir, "privkey"),
    ConnectTimeout: 60,
    SyncPeriod: 100,
    MaxRecordsPerPeer: 10,
    MaxTrackedPeers: 10,
  }
}

func authCtx() context.Context {
  // Fakes an authenticated WWW request to bypass peer verification
  return context.WithValue(context.Background(), "webauthn", true)
}

func testCmdServerWithConn(t *testing.T) *CommandServer {
  s := testCmdServer(t)
  if err := s.d.handleConnect(testNetConnReq(t)); err != nil {
    t.Fatalf("Connect error: %v", err)
  }
  return s
}

func testCmdServer(t *testing.T) *CommandServer {
  d := newTestDriver(t)
  return newCmdServer(d)
}

func TestPingNoAuth (t *testing.T) {
  if _, err := testCmdServer(t).Ping(context.Background(), &pb.HealthCheck{}); err == nil || !strings.Contains(err.Error(), "Unauthenticated") {
    t.Errorf("Expected unauthenticated error, got %v", err)
  }
}

func TestPing(t *testing.T) {
  if ok, err := testCmdServer(t).Ping(authCtx(), &pb.HealthCheck{}); ok == nil || err != nil {
    t.Errorf("Ping = %v, %v; want ok, nil", ok, err)
  }
}

func TestGetId(t *testing.T) {
  s := testCmdServerWithConn(t)
  rep, err := s.GetId(authCtx(), &pb.GetIDRequest{Network: TestNet})
  if err != nil || rep.Id == "" {
    t.Errorf("GetId = %v, %v; want non-empty, nil", rep, err)
  }
}

func TestSetStatus(t *testing.T) {
  t.Skip("TODO");
}

func TestConnectGetAndDisconnect(t *testing.T) {
  s := testCmdServer(t)
  if ok, err := s.Connect(authCtx(), testNetConnReq(t)); err != nil || ok == nil {
    t.Errorf("Connect = %v, %v, want ok, nil", ok, err)
  }
  if cc, err := s.GetConnections(authCtx(), &pb.GetConnectionsRequest{}); err != nil || len(cc.Networks) != 1 {
    t.Errorf("GetConnections = %v, %v, want 1 conn, nil", cc, err)
  }
  if ok, err := s.Disconnect(authCtx(), &pb.DisconnectRequest{Network: TestNet}); err != nil || ok == nil {
    t.Errorf("Disconnect = %v, %v, want ok, nil", ok, err)
  }
  if cc, err := s.GetConnections(authCtx(), &pb.GetConnectionsRequest{}); err != nil || len(cc.Networks) != 0 {
    t.Errorf("GetConnections = %v, %v, want 0 conn, nil", cc, err)
  }
}

func TestRecordSetAndStream(t *testing.T) {
  s := testCmdServerWithConn(t)
  if ok, err := s.SetRecord(authCtx(), &pb.SetRecordRequest{
    Network: TestNet,
    Record: &pb.Record{
      Uuid: "foo",
      Approver: "bar",
      Rank: &pb.Rank{},
    },
  }); err != nil || ok == nil {
    t.Errorf("SetRecord = %v, %v, want ok, nil", ok, err)
  }
  acc := []*pb.SignedRecord{}
  send := func(sr *pb.SignedRecord) error {
    acc = append(acc, sr)
    return nil
  }
  if err := s.streamRecordsImpl(&pb.StreamRecordsRequest{Network: TestNet}, send, context.Background()); err != nil {
    t.Errorf("StreamRecords: %v", err)
  } else if len(acc) != 1 || acc[0].Record.Uuid != "foo" {
    t.Errorf("Want single foo record, got %v", acc)
  }
}

func TestCompletionSetAndStream(t *testing.T) {
  s := testCmdServerWithConn(t)
  if ok, err := s.SetCompletion(authCtx(), &pb.SetCompletionRequest{
    Network: TestNet,
    Completion: &pb.Completion{
      Uuid: "foo",
      CompleterState: []byte("asdf"),
    },
  }); err != nil || ok == nil {
    t.Errorf("SetCompletion = %v, %v, want ok, nil", ok, err)
  }
  acc := []*pb.SignedCompletion{}
  send := func(sr *pb.SignedCompletion) error {
    acc = append(acc, sr)
    return nil
  }
  if err := s.streamCompletionsImpl(&pb.StreamCompletionsRequest{Network: TestNet}, send, context.Background()); err != nil {
    t.Errorf("StreamCompletions: %v", err)
  } else if len(acc) != 1 || acc[0].Completion.Uuid != "foo" {
    t.Errorf("Want single foo completion, got %v", acc)
  }
}

func TestStreamEvents(t *testing.T) {
  t.Skip("TODO");
}

func TestAdvertiseDoStreamStop(t *testing.T) {
  s := testCmdServerWithConn(t)
  if ok, err := s.Advertise(authCtx(), &pb.AdvertiseRequest{
    Local: true,
    Config: &pb.NetworkConfig{
      Uuid: "foo",
    },
  }); err != nil || ok == nil {
    t.Errorf("Advertise = %v, %v, want ok, nil", ok, err)
  }
  acc := []*pb.Network{}
  send := func(n *pb.Network) error {
    acc = append(acc, n)
    return nil
  }
  if err := s.streamNetworksImpl(&pb.StreamNetworksRequest{Local: true}, send, context.Background()); err != nil {
    t.Errorf("StreamNetworks: %v", err)
  } else if len(acc) != 1 || acc[0].Config.Uuid != "foo" {
    t.Errorf("Want single foo net, got %v", acc)
  }

  if ok, err := s.StopAdvertising(authCtx(), &pb.StopAdvertisingRequest{Local: true, Uuid: "foo"}); err != nil || ok == nil {
    t.Errorf("StopAdvertising = %v, %v, want ok, nil", ok, err)
  }

  acc2 := []*pb.Network{}
  send2 := func(n *pb.Network) error {
    acc2 = append(acc2, n)
    return nil
  }
  if err := s.streamNetworksImpl(&pb.StreamNetworksRequest{Local: true}, send2, context.Background()); err != nil {
    t.Errorf("StreamNetworks: %v", err)
  } else if len(acc2) != 0 {
    t.Errorf("Want empty list, got %v", acc2)
  }
}

func TestStreamAdvertisements(t *testing.T) {
  t.Skip("TODO")
}
func TestSyncLobby(t *testing.T) {
  t.Skip("TODO")
}
func TestStreamPeers(t *testing.T) {
  t.Skip("TODO")
}


package server

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "google.golang.org/protobuf/proto"
	"github.com/libp2p/go-libp2p/core/peer"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "sync"
  "testing"
  "fmt"
  "time"
  "context"
  "log"
  "os"
)

func NewTestServer(ctx context.Context, rendezvous string) *Server {
  // Creates a server with a random set of keys, inmemory storage,
  // and a local transport with MDNS
  logger := log.New(os.Stderr, "", 0)
  kpriv, kpub, err := crypto.GenKeyPair()
  t, err := transport.New(&transport.Opts{
    PubsubAddr: "/ip4/127.0.0.1/tcp/0",
    Rendezvous: rendezvous,
    Local: true,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: crypto.LoadPSK("1234"),
    ConnectTimeout: 5*time.Second,
    Topics: []string{DefaultTopic},
  }, ctx, logger)
  if err != nil {
    panic(err)
  }
  st, err := storage.NewSqlite3(":memory:")
  if err != nil {
    panic(err)
  }
  id, err := peer.IDFromPublicKey(kpub)
  name := id.Pretty()
  name = name[len(name)-4:]
  s := New(t, st, &Opts{
    SyncPeriod: 1*time.Hour,
    DisplayName: name,
    MaxRecordsPerPeer: 10,
    MaxCompletionsPerPeer: 10,
    MaxTrackedPeers: 3,
  }, pplog.New(name, logger))
  return s
}

type tsFunc func(context.Context, *Server, *Server) error

func twoServerTest(rendezvous string, setup tsFunc, test tsFunc) error {
  ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  defer cancel()
  s1 := NewTestServer(ctx, rendezvous)
  s2 := NewTestServer(ctx, rendezvous)

  if err := setup(ctx, s1, s2); err != nil {
    return err
  }

  var wg sync.WaitGroup
  wg.Add(2)
  go func() {
    defer wg.Done()
    s1.Run(ctx)
  }()
  go func() {
    defer wg.Done()
    s2.Run(ctx)
  }()
  wg.Wait()
  select {
  case <-ctx.Done():
    return fmt.Errorf("Timeout before peers connected")
  default:
    return test(ctx, s1, s2)
  }
}

func getSCs(s storage.Interface) ([]*pb.SignedCompletion, error) {
  result := []*pb.SignedCompletion{}
  ch := make(chan *pb.SignedCompletion)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer wg.Done()
    for {
      if v, ok := <-ch; ok {
        result = append(result, v)
      } else {
        break
      }
    }
  }()
  if err := s.GetSignedCompletions(context.Background(), ch); err != nil {
    return nil, err
  }
  wg.Wait()
  return result, nil
}

func getSRs(s storage.Interface) ([]*pb.SignedRecord, error) {
  result := []*pb.SignedRecord{}
  ch := make(chan *pb.SignedRecord)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer wg.Done()
    for {
      if v, ok := <-ch; ok {
        result = append(result, v)
      } else {
        break
      }
    }
  }()
  if err := s.GetSignedRecords(context.Background(), ch); err != nil {
    return nil, err
  }
  wg.Wait()
  return result, nil
}

func TestIDAndShortID(t *testing.T) {
  s1 := NewTestServer(context.Background(), "testid")
  sid := s1.ID()
  if err := peer.ID(sid).Validate(); err != nil {
    t.Errorf("Expected valid ID when extracting public key, got %v: %v", sid, err)
  }

  if shid := s1.ShortID(); len(shid) > 10 {
    t.Errorf("Short ID %v is long", shid)
  }
}

func TestGetService(t *testing.T) {
  s1 := NewTestServer(context.Background(), "testid")
  if srv := s1.GetService(); srv == nil {
    t.Errorf("service is nil")
  }
}

func TestConnectBasic(t *testing.T) {
  con := false
  if err := twoServerTest("t1",
    func(ctx context.Context, s1, s2 *Server) error {return nil},
    func(ctx context.Context, s1, s2 *Server) error {
      con = true
      return nil
    },
  ); err != nil {
    t.Errorf(err.Error())
  }
  if !con {
    t.Errorf("Expected connection")
  }
}

func TestIssueRecord(t *testing.T) {
  var sr *pb.SignedRecord
  var err error
  if err := twoServerTest("TestIssueRecord",
    func(ctx context.Context, s1, s2 *Server) error { return nil },
    func(ctx context.Context, s1, s2 *Server) error {
      sr, err = s1.IssueRecord(&pb.Record{
        Uuid: "record1",
        Approver: "otherpeer",
        Rank: &pb.Rank{},
        Created: time.Now().Unix(),
        Manifest: "CID",
      }, true);
      return err
      if srs, err := getSRs(s2.s); err != nil {
        t.Errorf("get srs: %v", err)
      } else if len(srs) != 1 || !proto.Equal(srs[0], sr) {
        t.Errorf("GetSignedRecord got %v; want {%v}", srs, sr)
      }
      return nil
    },
  ); err != nil {
    t.Errorf(err.Error())
  }
}

func TestIssueCompletion(t *testing.T) {
  var sc *pb.SignedCompletion
  var err error
  if err := twoServerTest("TestIssueCompletion",
    func(ctx context.Context, s1, s2 *Server) error { return nil },
    func(ctx context.Context, s1, s2 *Server) error {
      sc, err = s1.IssueCompletion(&pb.Completion{
          Uuid: "completion1",
          Completer: "otherpeer",
          CompleterState: []byte("foo"),
          Timestamp: time.Now().Unix(),
      }, true)
      return err
      if scs, err := getSCs(s2.s); err != nil {
        t.Errorf("get scs: %v", err)
      } else if len(scs) != 1 || !proto.Equal(scs[0], sc) {
        t.Errorf("GetSignedCompletion got %v; want {%v}", scs, sc)
      }
      return nil
    },
  ); err != nil {
    t.Errorf(err.Error())
  }
}

func TestOnUpdate(t *testing.T) {
  var m proto.Message
  var wg sync.WaitGroup
  s1 := NewTestServer(context.Background(), "testid")
  wg.Add(1)
  go func() {
    defer wg.Done()
    m = <- s1.OnUpdate()
  }()
  s1.notify(&pb.Ok{})
  wg.Wait()
  if !proto.Equal(m, &pb.Ok{}) {
    t.Errorf("Unexpected message: %v", m)
  }
}

func TestHandleCompletion(t *testing.T) {
  t.Skip("TODO")
}

func TestHandleRecord(t *testing.T) {
  t.Skip("TODO")
}

func TestGetSummary(t *testing.T) {
  s1 := NewTestServer(context.Background(), "testid")
  if s := s1.GetSummary(); s == nil {
    t.Errorf("Expected summary, got nil")
  }
}

func TestSync(t *testing.T) {
  var sr *pb.SignedRecord
  var sc *pb.SignedCompletion
  var err error

  if err := twoServerTest("t1",
    func(ctx context.Context, s1, s2 *Server) error {
      if sr, err = s2.IssueRecord(&pb.Record{
        Uuid: "record1",
        Approver: s2.ID(),
        Rank: &pb.Rank{},
        Tags: []string{"asdf", "ghjk"},
        Created: time.Now().Unix(),
        Manifest: "CID",
      }, false); err != nil {
        return err
      }
      if sc, err = s2.IssueCompletion(&pb.Completion{
          Uuid: "completion1",
          Completer: s2.ID(),
          CompleterState: []byte("foo"),
          Timestamp: time.Now().Unix(),
      }, false); err != nil {
        return err
      }
      return nil
    },
    func(ctx context.Context, s1, s2 *Server) error {
      // TODO some kind of synchronization needed here
      if srs, err := getSRs(s1.s); err != nil {
        t.Errorf("get srs: %v", err)
      } else if len(srs) != 1 || !proto.Equal(srs[0], sr) {
        t.Errorf("GetSignedRecord got %v; want {%v}", srs, sr)
      }
      if scs, err := getSCs(s1.s); err != nil {
        t.Errorf("get scs: %v", err)
      } else if len(scs) != 1 || !proto.Equal(scs[0], sc) {
        t.Errorf("GetSignedCompletion got %v; want {%v}", scs, sc)
      }
      return nil
    },
  ); err != nil {
    t.Errorf(err.Error())
  }
}

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
  "strings"
  "time"
  "context"
  "log"
  "os"
)

func NewTestServer(t *testing.T, rendezvous string) *Server {
  // Creates a server with a random set of keys, inmemory storage,
  // and a local transport with MDNS
  logger := log.New(os.Stderr, "", 0)
  kpriv, kpub, err := crypto.GenKeyPair()
  ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  t.Cleanup(cancel)
  tp, err := transport.New(&transport.Opts{
    Addr: "/ip4/127.0.0.1/tcp/0",
    Rendezvous: rendezvous,
    Local: true,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: crypto.LoadPSK("1234"),
    ConnectTimeout: 5*time.Second,
    Topics: []string{DefaultTopic},
  }, ctx, pplog.New("srv", logger))
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
  s := New(tp, st, &Opts{
    SyncPeriod: 1*time.Hour,
    DisplayName: name,
    MaxRecordsPerPeer: 10,
    MaxTrackedPeers: 10,
  }, pplog.New(name, logger))
  return s
}

type tsFunc func(*Server, *Server) error

func twoServerTest(t *testing.T, rendezvous string, setup tsFunc, test tsFunc) error {
  s1 := NewTestServer(t, rendezvous)
  s2 := NewTestServer(t, rendezvous)

  if err := setup(s1, s2); err != nil {
    return err
  }

  var wg sync.WaitGroup
  ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
  t.Cleanup(cancel)
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
    return test(s1, s2)
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
  s1 := NewTestServer(t, "testid")
  sid := s1.ID()
  if err := peer.ID(sid).Validate(); err != nil {
    t.Errorf("Expected valid ID when extracting public key, got %v: %v", sid, err)
  }

  if shid := s1.ShortID(); len(shid) > 10 {
    t.Errorf("Short ID %v is long", shid)
  }
}

func TestGetService(t *testing.T) {
  s1 := NewTestServer(t, "testid")
  if srv := s1.getService(); srv == nil {
    t.Errorf("service is nil")
  }
}

func TestConnectBasic(t *testing.T) {
  con := false
  if err := twoServerTest(t, "t1",
    func(s1, s2 *Server) error {return nil},
    func(s1, s2 *Server) error {
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
  if err := twoServerTest(t, "TestIssueRecord",
    func(s1, s2 *Server) error { return nil },
    func(s1, s2 *Server) error {
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
  if err := twoServerTest(t, "TestIssueCompletion",
    func(s1, s2 *Server) error { return nil },
    func(s1, s2 *Server) error {
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

func TestHandleCompletionInvalidSignature(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  if err := s1.handleCompletion("peer", &pb.Completion{}, &pb.Signature{Signer: s1.t.ID()}); err == nil || !strings.Contains(err.Error(), "invalid signature") {
    t.Errorf("Want invalid sig error, got %v", err)
  }
}

func TestHandleCompletionValidateFail(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  s2 := NewTestServer(t, "rendy")

  cmp := &pb.Completion{
    Uuid: "foo",
  }
  sig, err := s2.t.Sign(cmp)
  if err != nil {
    panic(err)
  }

  if err := s1.handleCompletion("invalidpeer", cmp, &pb.Signature{Signer: s2.t.ID(), Data: sig}); err == nil || !strings.Contains(err.Error(), "validation") {
    t.Errorf("Want validation error, got %v", err)
  }
}

func TestHandleCompletionInProgress(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  s2 := NewTestServer(t, "rendy")
  if err := doHandleRecord(s1, s2); err != nil {
    t.Fatalf("handleRecord error: %v", err)
  }

  cmp := &pb.Completion{
    Uuid: "foo",
    CompleterState: []byte("testing"),
  }
  sig, err := s2.t.Sign(cmp)
  if err != nil {
    panic(err)
  }

  if err := s1.handleCompletion("peer", cmp, &pb.Signature{Signer: s2.t.ID(), Data: sig}); err != nil {
    t.Errorf("handleCompletion error: %v", err)
  }
}

func numCmp(s *Server) int {
  num := 0
  ch := make(chan *pb.SignedCompletion)
  var wg sync.WaitGroup
  wg.Add(1)
  go func(){
    defer wg.Done()
    for range ch {
      num++
    }
  }()
  err := s.s.GetSignedCompletions(context.Background(), ch)
  if err != nil {
    panic(err)
  }
  wg.Wait()
  return num
}

func TestHandleCompletionCollapses(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  s2 := NewTestServer(t, "rendy")
  if err := doHandleRecord(s1, s2); err != nil {
    t.Fatalf("handleRecord error: %v", err)
  }

  for i := 0; i < 5; i++ {
    if err := s1.s.SetSignedCompletion(&pb.SignedCompletion{
      Completion: &pb.Completion{
        Uuid: "foo",
        Completer: "bar",
        CompleterState: []byte("asdf"),
      },
      Signature: &pb.Signature{
        Signer: fmt.Sprintf("peer%d", i),
        Data: []byte("123"),
      },
    }); err != nil {
      panic(err)
    }
  }
  if n := numCmp(s1); n != 5 {
    t.Fatalf("Setup failure: want 5 completions got %d", n)
  }

  cmp := &pb.Completion{
    Uuid: "foo",
    CompleterState: []byte("testing"),
  }
  sig, err := s2.t.Sign(cmp)
  if err != nil {
    panic(err)
  }

  // Peer is signer is record approver,
  // collapses completions
  if err := s1.handleCompletion(s2.t.ID(), cmp, &pb.Signature{Signer: s2.t.ID(), Data: sig}); err != nil {
    t.Errorf("handleCompletion error: %v", err)
  }

  if n := numCmp(s1); n != 1 {
    t.Errorf("handleCompletion: want 1 stored completion, got %d", n)
  }
}

func TestHandleRecordInvalidSignature(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  if err := s1.handleRecord("peer", &pb.Record{}, &pb.Signature{Signer: s1.t.ID()}); err == nil || !strings.Contains(err.Error(), "invalid signature") {
    t.Errorf("Want invalid sig error, got %v", err)
  }
}

func TestHandleRecordValidateFail(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  s2 := NewTestServer(t, "rendy")

  rec := &pb.Record{
    Uuid: "foo",
    Approver: s2.t.ID(),
  }
  sig, err := s2.t.Sign(rec)
  if err != nil {
    panic(err)
  }

  if err := s1.handleRecord("invalidpeer", rec, &pb.Signature{Signer: s2.t.ID(), Data: sig}); err == nil || !strings.Contains(err.Error(), "validation") {
    t.Errorf("Want validation error, got %v", err)
  }
}

func doHandleRecord(s1, s2 *Server) error {
  rec := &pb.Record{
    Uuid: "foo",
    Approver: s2.t.ID(),
    Rank: &pb.Rank{},
  }
  sig, err := s2.t.Sign(rec)
  if err != nil {
    panic(err)
  }
  return s1.handleRecord(s2.t.ID(), rec, &pb.Signature{Signer: s2.t.ID(), Data: sig})
}

func TestHandleRecordOK(t *testing.T) {
  s1 := NewTestServer(t, "rendy")
  s2 := NewTestServer(t, "rendy")
  if err := doHandleRecord(s1, s2); err != nil {
    t.Errorf("handleRecord error: %v", err)
  }
}

func TestGetSummary(t *testing.T) {
  s1 := NewTestServer(t, "testid")
  if s := s1.GetSummary(); s == nil {
    t.Errorf("Expected summary, got nil")
  }
}

func TestSync(t *testing.T) {
  var sr *pb.SignedRecord
  var sc *pb.SignedCompletion
  var err error

  if err := twoServerTest(t, "t1",
    func(s1, s2 *Server) error {
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
    func(s1, s2 *Server) error {
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

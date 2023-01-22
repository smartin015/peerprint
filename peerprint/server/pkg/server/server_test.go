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
    Topics: []string{DefaultTopic, StatusTopic},
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
    StatusPeriod: 1*time.Hour,
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
/*
func TestConnectBasic(t *testing.T) {
  con := false
  if err := twoServerTest("t1",
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
*/

func TestSync(t *testing.T) {

  var sr *pb.SignedRecord
  var sc *pb.SignedCompletion
  var err error

  if err := twoServerTest("t1",
    func(ctx context.Context, s1, s2 *Server) error {
      if sr, err = s2.IssueRecord(&pb.Record{
        Uuid: "record1",
        Approver: "otherpeer",
        Rank: &pb.Rank{},
        Created: time.Now().Unix(),
        Location: "CID",
      }, false); err != nil {
        return err
      }
      if sc, err = s2.IssueCompletion(&pb.Completion{
          Uuid: "completion1",
          Completer: "otherpeer",
          Timestamp: time.Now().Unix(),
      }, false); err != nil {
        return err
      }
      return nil
    },
    func(ctx context.Context, s1, s2 *Server) error {
      gr := &pb.SignedRecord{}
      if err = s1.s.GetSignedRecord("record1", gr); err != nil || !proto.Equal(sr, gr) {
        t.Errorf("GetSignedRecord got %v, %v; want %v, nil", gr, err, sr)
      }
      gcChan := make(chan *pb.SignedCompletion, 10)
      if err = s1.s.GetSignedCompletions(ctx, gcChan); err != nil {
        t.Errorf(err.Error())
      }
      select {
      case gc := <-gcChan:
        if !proto.Equal(sc, gc) {
          t.Errorf("GetSignedCompletions got %v; want %v", gc, sc)
        }
      default:
        t.Errorf("Unable to get signed completion from %s (channel empty)", shorten(s1.ID()))
      }
      return nil
    },
  ); err != nil {
    t.Errorf(err.Error())
  }
}

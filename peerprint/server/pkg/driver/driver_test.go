package driver

import (
  "testing"
	"github.com/libp2p/go-libp2p/core/peer"
  "google.golang.org/protobuf/proto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "gopkg.in/zeromq/goczmq.v4"
  "path/filepath"
  "sync"
  "log"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/crawl"
  "github.com/smartin015/peerprint/p2pgit/pkg/cmd"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "time"
  "context"
  "os"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  //"context"
	//"github.com/libp2p/go-libp2p/core/peer"
)

var logger = log.New(os.Stderr, "", 0)

func makeIPC(t *testing.T) string {
  return "ipc://" + filepath.Join(t.TempDir(), "ipc")
}

func makePeerID() string {
  kpriv, _, err := crypto.GenKeyPair()
  if err != nil {
    panic(err)
  }
  id, err := peer.IDFromPrivateKey(kpriv)
  if err != nil {
    panic(err)
  }
  return id.String()
}

func newTestDriver() (*Driver, context.Context, context.CancelFunc) {
  ctx, done := context.WithTimeout(context.Background(), 60*time.Second)
  kpriv, kpub, err := crypto.GenKeyPair()
  if err != nil {
    panic(err)
  }
  st, err := storage.NewSqlite3(":memory:")
  if err != nil {
    panic(err)
  }
  t, err := transport.New(&transport.Opts{
    PubsubAddr: "/ip4/127.0.0.1/tcp/0",
    Rendezvous: "drivertest",
    Local: true,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: crypto.LoadPSK("12345"),
    ConnectTimeout: 10*time.Second,
    Topics: []string{server.DefaultTopic},
  }, ctx, logger)
  if err != nil {
    panic(err)
  }
  srv := server.New(t, st, &server.Opts{
    SyncPeriod: 10*time.Minute,
    DisplayName: "srv",
    MaxRecordsPerPeer: 1000,
    MaxTrackedPeers: 1000,
  }, pplog.New("srv", logger))

  return NewDriver(srv, st, t, pplog.New("cmd", logger)), ctx, done
}

func TestCrawlExcessiveMultiAddr(t *testing.T) {
  // Prevent malicious peers from overwhelming us with addresses
  d, _, done := newTestDriver()
  defer done()
  addrs := []*pb.AddrInfo{}
  for i := 0; i < 2*MaxAddrPerPeer; i++ {
    addrs = append(addrs, &pb.AddrInfo{Id: makePeerID(), Addrs: []string{"/ip4/127.0.0.1/tcp/12345"}})
  }

  got := d.handleGetPeersResponse(&pb.GetPeersResponse{Addresses: addrs})
  if got == nil || len(got) != MaxAddrPerPeer {
    t.Errorf("Expected len(addrs)=%d, got %d", MaxAddrPerPeer, len(got))
  }
}

func runCmdTest(t *testing.T, d *Driver, req_addr string, snd proto.Message) proto.Message {
  req, err := goczmq.NewReq(req_addr)
  if err != nil {
    panic(err)
  }
  t.Cleanup(req.Destroy)

  ser, err := cmd.Serialize(snd)
  if err != nil {
    t.Fatal(err)
  }
  req.SendMessage([][]byte{ser})

  m, err := req.RecvMessage()
  if err != nil {
    t.Fatal(err)
  }
  got, err := cmd.Deserialize(m)
  if err != nil {
    t.Fatal(err)
  }
  return got
}

func TestReceiveHealthCheck(t *testing.T) {
  d, ctx, done := newTestDriver()
  defer done()
  reqrep := makeIPC(t)
  go d.Loop(ctx, reqrep, makeIPC(t), 10*time.Second)
  want := &pb.HealthCheck{}
  if got := runCmdTest(t, d, reqrep, &pb.HealthCheck{}); !proto.Equal(got, want) {
    t.Errorf("reply: want %+v got %+v", want, got)
  }
}

func TestReceiveUnknownCommand(t *testing.T) {
  d, ctx, done := newTestDriver()
  defer done()
  reqrep := makeIPC(t)
  go d.Loop(ctx, reqrep, makeIPC(t), 10*time.Second)
  want :=  &pb.Error{Reason: "Unrecognized command"}
  runCmdTest(t, d, reqrep, &pb.Ok{})
  if got := runCmdTest(t, d, reqrep, &pb.Ok{}); !proto.Equal(got, want) {
    t.Errorf("reply: want %+v got %+v", want, got)
  }
}

func TestReceiveGetId(t *testing.T) {
  d, ctx, done := newTestDriver()
  defer done()
  reqrep := makeIPC(t)
  go d.Loop(ctx, reqrep, makeIPC(t), 10*time.Second)
  want := &pb.IDResponse{Id: d.s.ID()}
  if got := runCmdTest(t, d, reqrep, &pb.GetID{}); !proto.Equal(got, want) {
    t.Errorf("reply: want %+v got %+v", want, got)
  }
}

func TestReceiveRecordCommand(t *testing.T) {
  d, ctx, done := newTestDriver()
  defer done()
  reqrep := makeIPC(t)
  go d.Loop(ctx, reqrep, makeIPC(t), 10*time.Second)
  got := runCmdTest(t, d, reqrep, &pb.Record{
    Uuid: "r1",
    Rank: &pb.Rank{},
  }).(*pb.SignedRecord)
  if got.Record.Uuid != "r1" {
    t.Errorf("reply: %+v want ID r1", got)
  }
}

func TestReceiveCompletionCommand(t *testing.T) {
  d, ctx, done := newTestDriver()
  defer done()
  reqrep := makeIPC(t)
  go d.Loop(ctx, reqrep, makeIPC(t), 10*time.Second)
  got := runCmdTest(t, d, reqrep, &pb.Completion{
    Uuid: "r1",
    CompleterState: []byte("foo"),
  }).(*pb.SignedCompletion)
  if got.Completion.Uuid != "r1" {
    t.Errorf("reply: %+v want ID r1", got)
  }
}

func TestWatchdog(t *testing.T) {
  d, ctx, done := newTestDriver()
  defer done()
  err := d.Loop(ctx, makeIPC(t), makeIPC(t), 1*time.Millisecond)
  if err != ErrWatchdog {
    t.Errorf("Want ErrWatchdog, got err %v", err)
  }
}

func TestContext(t *testing.T) {
  d, ctx, done := newTestDriver()
  var err error
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer wg.Done()
    err = d.Loop(ctx, makeIPC(t), makeIPC(t), 10*time.Second)
  }()
  done()
  wg.Wait()
  if err != ErrContext {
    t.Errorf("Want ErrContext, got err %v", err)
  }
}

func TestCrawlPeerCyclesPrevented(t *testing.T) {
  d, ctx, done := newTestDriver()
  defer done()
  d.c = crawl.NewCrawler(d.t.GetPeerAddresses(), d.crawlPeer)
  p := &peer.AddrInfo{
    ID: "foo",
  }
  if err := d.st.LogPeerCrawl(p.ID.String(), d.c.Started.Unix()); err != nil {
    t.Fatal(err)
  }
  if got, err := d.crawlPeer(ctx, p); len(got) != 0 || err != nil {
    t.Errorf("crawlPeer of already crawled peer - got %v, %v want 0-len, nil", got, err)
  }
}


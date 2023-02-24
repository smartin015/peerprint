package driver

import (
  "testing"
)


/*
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
*/

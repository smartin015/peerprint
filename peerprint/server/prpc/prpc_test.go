package prpc

import (
  "testing"
  "context"
  "fmt"

  "time"
  "sync"
	"google.golang.org/protobuf/proto"
  "strings"
  "github.com/libp2p/go-libp2p"
  "google.golang.org/protobuf/types/known/wrapperspb"
  "github.com/libp2p/go-libp2p-core/host"
  "github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const TestTopic = "testtopic"

func testHost(t *testing.T) host.Host {
  t.Helper()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
  if err != nil {
    t.Fatal(err)
  }
	t.Cleanup(func() {
		h.Close()
    h.ConnManager().Close()
	})
  return h
}

func testPS(t *testing.T, h host.Host, peers []peer.AddrInfo) *pubsub.PubSub {
  for _, ai := range peers {
    if err := h.Connect(context.Background(), ai); err != nil {
      t.Fatalf(err.Error())
    }
  }

  ps, err := pubsub.NewGossipSub(context.Background(), h, pubsub.WithDirectPeers(peers))
  if err != nil {
    t.Fatal(err)
  }
  return ps
}

func testEnv(t *testing.T) []*PRPC {
  ctx := context.Background()
  h1 := testHost(t)
  h2 := testHost(t)
  h1a := peer.AddrInfo{ID: h1.ID(), Addrs: h1.Addrs()}
  h2a := peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}

  p1 := testPS(t, h1, []peer.AddrInfo{h2a})
  p2 := testPS(t, h2, []peer.AddrInfo{h1a})

  pp := []*PRPC{
    New(ctx, h1.ID().String(), p1),
    New(ctx, h2.ID().String(), p2),
  }
  t.Cleanup(func() {
    for _, p := range pp {
      p.Close()
    }
  })
  return pp
}

func TestJoinLeaveRejoin(t *testing.T) {
  pp := testEnv(t)
  ctx := context.Background()

  if err := pp[0].JoinTopic(ctx, TestTopic); err != nil {
    t.Error(err)
  }
  if err := pp[0].LeaveTopic(TestTopic); err != nil {
    t.Error(err)
  }
  if err := pp[0].JoinTopic(ctx, TestTopic); err != nil {
    t.Error(err)
  }
}

func TestJoinExisting(t *testing.T) {
  pp := testEnv(t)
  ctx := context.Background()

  if err := pp[0].JoinTopic(ctx, TestTopic); err != nil {
    t.Error(err)
  }
  err := pp[0].JoinTopic(ctx, TestTopic)
  if err == nil {
    t.Errorf("expected error from second JoinTopic")
  }
  wantsub := "Already subscribed"
  if !strings.Contains(err.Error(), wantsub) {
    t.Errorf("Wanted %q error substr, got %v", wantsub, err)
  }
}

func testEnvWithJoinedPeers(t *testing.T) []*PRPC {
  ctx := context.Background()
  pp := testEnv(t)

  if err := pp[0].JoinTopic(ctx, TestTopic); err != nil {
    t.Fatalf(err.Error())
  }
  if err := pp[1].JoinTopic(ctx, TestTopic); err != nil {
    t.Fatalf(err.Error())
  }

  // We must wait a moment for the pubsub peer state to settle before
  // publishing messages
  for {
    if pp[0].PeerCount(TestTopic) > 0 && pp[1].PeerCount(TestTopic) > 0 {
      break
    }
    time.Sleep(100 * time.Millisecond)
  }

  return pp
}

func TestPublishReply(t *testing.T) {
  want := &wrapperspb.StringValue{Value: "test1"}
  wantRep := &wrapperspb.StringValue{Value: "testrep"}
  pp := testEnvWithJoinedPeers(t)

  var got0 proto.Message
  var got1 proto.Message
  var wg sync.WaitGroup
  wg.Add(2)
  pp[1].RegisterCallback(func(topic string, peer string, msg proto.Message) (proto.Message, error) {
    got1 = msg
    wg.Done()
    return wantRep, nil
  })
  pp[0].RegisterCallback(func(topic string, peer string, msg proto.Message) (proto.Message, error) {
    got0 = msg
    wg.Done()
    return nil, nil
  })

  pp[0].Publish(context.Background(), TestTopic,  want)
  wg.Wait()
  if !proto.Equal(got1, want) {
    t.Errorf("Peer 1 got peer 0 request %v, want %v", got1, want)
  }
  if !proto.Equal(got0, wantRep) {
    t.Errorf("Peer 0 got peer 1 reply %v, want %v", got0, wantRep)
  }
}

func TestReplyError(t *testing.T) {

  pp := testEnvWithJoinedPeers(t)
  var wg sync.WaitGroup
  wg.Add(1)
  pp[1].RegisterCallback(func(topic string, peer string, msg proto.Message) (proto.Message, error) {
    wg.Done()
    return nil, fmt.Errorf("Intentional testing error")
  })

  pp[0].Publish(context.Background(), TestTopic,&wrapperspb.StringValue{Value: "test1"})
  wg.Wait()

  e, more := <-pp[1].ErrChan
  if !more {
    t.Errorf("expected error in ErrChan, got none")
  } else if !strings.Contains(e.Error(), "Intentional") {
    t.Errorf("want \"Intentional\" error, got %v", e)
  }
}

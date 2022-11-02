package topic_receiver

import (
  "testing"
  "context"
  "time"
	"google.golang.org/protobuf/proto"
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

type TestPeer struct {
  Recv chan TopicMsg
  Err chan error
  Pub chan<- proto.Message
  PubSub *pubsub.PubSub
  Host host.Host
  AddrInfo peer.AddrInfo
}

func testEnv(t *testing.T) []*TestPeer  {
  ctx := context.Background()

  pp := []*TestPeer{}
  for i:= 0; i < 2; i++ {
    h := testHost(t)
    p := &TestPeer{
      Recv: make(chan TopicMsg),
      Err: make(chan error),
      AddrInfo: peer.AddrInfo{ID: h.ID(), Addrs: h.Addrs()},
      Host: h,
    }
    t.Cleanup(func() {
      close(p.Recv)
      close(p.Err)
    })
    pp = append(pp, p)
  }

  for _, p := range pp {
    peers := []peer.AddrInfo{}
    for _, p2 := range pp {
      if p2 != p {
        peers = append(peers, p2.AddrInfo)
      }
    }
    p.PubSub = testPS(t, p.Host, peers)
    pub, err := NewTopicChannel(ctx, p.Recv, p.Host.ID().String(), p.PubSub, TestTopic, p.Err)
    if err != nil {
      t.Fatalf(err.Error())
    }
    // Closed by channel event loop
    p.Pub = pub
  }

  // We must wait a moment for the pubsub peer state to settle before
  // publishing messages
  for {
    ready := true
    for _, p := range pp {
      ready = ready && len(p.PubSub.ListPeers(TestTopic)) > 0
    }
    if ready {
      break
    }
    time.Sleep(100 * time.Millisecond)
  }
  return pp
}

func TestPublishReceive(t *testing.T) {
  want := &wrapperspb.StringValue{Value: "test1"}
  pp := testEnv(t)
  pp[0].Pub <- want
  got := <-pp[1].Recv

  if !proto.Equal(got.Msg, want) {
    t.Errorf("Destination peer received %v, want %v", got, want)
  }

  select {
  case v := <-pp[0].Recv:
    t.Errorf("Received unexpected value on sending peer's Recv channel: %v", v)
  case err := <-pp[0].Err:
    t.Errorf(err.Error())
  case err := <-pp[1].Err:
    t.Errorf(err.Error())
  default:
  }
}


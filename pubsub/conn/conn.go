package conn

import (
	"context"
	"flag"
  "log"
	"os"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
)

var (
	topicNameFlag = flag.String("topicName", "applesauce", "name of topic to join")
  stderr = log.New(os.Stderr, "", 0)
)

type Conn struct {
  ctx context.Context
  addr string
  rendezvous string
  h host.Host
  ps *pubsub.PubSub
  onReady chan bool
}

func New(ctx context.Context, addr string, rendezvous string) *Conn {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"))
	if err != nil {
		panic(err)
	}

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}

  c := &Conn {
    ctx: ctx,
    addr: addr,
    rendezvous: rendezvous,
    h: h,
    ps: ps,
    onReady: make(chan bool),
  }
	go c.discoverPeers(rendezvous)
  return c
}

func (c *Conn) GetID() string {
  return c.h.ID().String()
}

func (c *Conn) initDHT() *dht.IpfsDHT {
	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kademliaDHT, err := dht.New(c.ctx, c.h)
	if err != nil {
		panic(err)
	}
	if err = kademliaDHT.Bootstrap(c.ctx); err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.h.Connect(c.ctx, *peerinfo); err != nil {
				stderr.Println("Bootstrap warning: %s", err)
			}
		}()
	}
	wg.Wait()
	return kademliaDHT
}

func (c *Conn) discoverPeers(rendezvous string) {
	kademliaDHT := c.initDHT()
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(c.ctx, routingDiscovery, rendezvous)

	// Look for others who have announced and attempt to connect to them
	anyConnected := false
	for !anyConnected {
		stderr.Println("Searching for peers...")
		peerChan, err := routingDiscovery.FindPeers(c.ctx, rendezvous)
		if err != nil {
			panic(err)
		}
		for peer := range peerChan {
      if peer.ID == c.h.ID() {
				continue // No self connection
			}
			err := c.h.Connect(c.ctx, peer)
			if err != nil {
				stderr.Println("Failed connecting to ", peer.ID.Pretty(), ", error:", err)
			} else {
				stderr.Println("Connected to:", peer.ID.Pretty())
				anyConnected = true
			}
		}
	}
	stderr.Println("Peer discovery complete")
  c.onReady <- true
}

func (c *Conn) GetPubSub() *pubsub.PubSub {
  return c.ps
}

func (c *Conn) AwaitReady(ctx context.Context) error {
  select {
  case <-c.onReady:
    return nil
  case <-ctx.Done():
    return ctx.Err()
  }
}

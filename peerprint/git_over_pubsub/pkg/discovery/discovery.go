// Package discovery wraps a libp2p host / key ID and discovers peers based on a rendezvous string.
// NOTE: this package is not threadsafe on AwaitReady.
package discovery

import (
	"context"
  "fmt"
	"log"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
)

type Method int64
const (
  MDNS Method = iota
  DHT
)

type Discovery struct {
	ctx          context.Context
	h            host.Host
	onReady      chan bool
	newConn      chan bool
  l *log.Logger
  method Method
  rendezvous string
}

var (
  bootstrapPeers []peer.AddrInfo
)

func SetBootstrapPeers(bp []peer.AddrInfo) {
  bootstrapPeers = bp
}

func New(ctx context.Context, m Method, h host.Host, rendezvous string, logger *log.Logger) *Discovery {
	c := &Discovery{
		ctx:          ctx,
		h:            h,
		onReady:      make(chan bool),
		newConn:      make(chan bool),
    rendezvous: rendezvous,
    method: m,
    l: logger,
	}

  if bootstrapPeers == nil {
    bootstrapPeers = dht.GetDefaultBootstrapPeerAddrInfos()
  }

	return c
}

func (c *Discovery) Run() {
	switch c.method {
  case MDNS:
		c.discoverPeersMDNS(c.rendezvous)
  case DHT:
		c.discoverPeersDHT(c.rendezvous)
  default:
    panic(fmt.Errorf("Unhandled discovery method: %+v", c.method))
  }
}

func (c *Discovery) bootstrapPeer(peer peer.AddrInfo, wg *sync.WaitGroup) {
  defer wg.Done()
  if err := c.h.Connect(c.ctx, peer); err != nil {
    c.l.Printf("Bootstrap warning: %s\n", err)
  }
}

func (c *Discovery) initDHT() *dht.IpfsDHT {
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
	for _, peer := range bootstrapPeers {
		wg.Add(1)
		go c.bootstrapPeer(peer, &wg)
	}
	wg.Wait()
	return kademliaDHT
}

// interface to be called when new  peer is found
func (c *Discovery) HandlePeerFound(peer peer.AddrInfo) {
	if peer.ID == c.h.ID() {
		return // No self connection
	}
	err := c.h.Connect(c.ctx, peer)
	if err != nil {
		c.l.Println("Failed connecting to ", peer.ID.Pretty(), ", error:", err)
	} else {
		c.l.Println("Connected to:", peer.ID.Pretty())
		notify(c.newConn)
	}
}

// Try to notify on a channel - return immediately if the channel is full
func notify(c chan bool) {
  select {
    case c <- true:
    default:
  }
}

func (c *Discovery) discoverPeersMDNS(rendezvous string) {
	// srv calls HandlePeerFound()
	srv := mdns.NewMdnsService(c.h, rendezvous, c)
	if err := srv.Start(); err != nil {
		panic(err)
	}
  select {
  case <-c.newConn:
    notify(c.onReady)
    return
  case <-c.ctx.Done():
    return
  }
}

func (c *Discovery) discoverPeersDHT(rendezvous string) {
	kademliaDHT := c.initDHT()
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(c.ctx, routingDiscovery, rendezvous)

	// Look for others who have announced and attempt to connect to them
	c.l.Println("Searching for peers...")
	for {
    select {
    case <-c.newConn:
      c.l.Println("Peer discovery complete")
      notify(c.onReady)
      return
    default:
    }
		peerChan, err := routingDiscovery.FindPeers(c.ctx, rendezvous)
		if err != nil {
			panic(err)
		}
		for peer := range peerChan {
			c.HandlePeerFound(peer)
		}
	}
}

func (c *Discovery) AwaitReady(ctx context.Context) error {
	select {
	case <-c.onReady:
    c.l.Println("Ready state achieved, returning from AwaitReady")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

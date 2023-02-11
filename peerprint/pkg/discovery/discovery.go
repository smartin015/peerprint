// Package discovery wraps a libp2p host / key ID and discovers peers based on a rendezvous string.
// NOTE: this package is not threadsafe on AwaitReady.
package discovery

import (
	"context"
  "fmt"
	"sync"

  "github.com/smartin015/peerprint/p2pgit/pkg/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
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
  connect bool
  l *log.Sublog
  method Method
  rendezvous string
}

var (
  bootstrapPeers []peer.AddrInfo
)

func SetBootstrapPeers(bp []peer.AddrInfo) {
  bootstrapPeers = bp
}

func New(ctx context.Context, m Method, h host.Host, rendezvous string, connect_on_discover bool, logger *log.Sublog) *Discovery {
	c := &Discovery{
		ctx:          ctx,
		h:            h,
		onReady:      make(chan bool),
		newConn:      make(chan bool),
    rendezvous: rendezvous,
    connect: connect_on_discover,
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
    c.l.Warning("Bootstrap: %s\n", err)
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
func (c *Discovery) HandlePeerFound(p peer.AddrInfo) {
	if p.ID == c.h.ID() {
		return // No self connection
	}
  if c.connect {
    err := c.h.Connect(c.ctx, p)
    if err != nil {
      c.l.Println("Failed connecting to ", p.ID.Pretty(), ", error:", err)
    } else {
      c.l.Println("Connected to:", p.ID.Pretty())
      notify(c.newConn)
    }
  } else if len(c.h.Peerstore().Addrs(p.ID)) == 0 {
    c.h.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
    // c.l.Println("Added peer to PeerStore:", p.ID.Pretty())
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

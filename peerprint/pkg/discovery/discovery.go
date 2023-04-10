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

  PeerDiscoverChanSize = 20
)

type Discovery struct {
	ctx          context.Context
	h            host.Host
	onReady      chan bool
	PeerDiscovered chan peer.AddrInfo
  connect bool
  l *log.Sublog
  method Method
  rendezvous string
  bootstrapPeers []peer.AddrInfo
}

func (c *Discovery) SetBootstrapPeers(bp []peer.AddrInfo) {
  c.bootstrapPeers = bp
}

func New(ctx context.Context, m Method, h host.Host, rendezvous string, connectOnDiscover bool, extraBootstrapPeers []string, logger *log.Sublog) *Discovery {
	c := &Discovery{
		ctx:          ctx,
		h:            h,
		onReady:      make(chan bool),
		PeerDiscovered:      make(chan peer.AddrInfo, PeerDiscoverChanSize),
    rendezvous: rendezvous,
    connect: connectOnDiscover,
    method: m,
    l: logger,
    bootstrapPeers: dht.GetDefaultBootstrapPeerAddrInfos(),
	}
  for _, ebp := range extraBootstrapPeers {
    ai, err := peer.AddrInfoFromString(ebp)
    if err != nil {
      c.l.Warning("failed to parse extra bootstrap peer %s: %v", ebp, err)
      continue
    }
    c.bootstrapPeers = append(c.bootstrapPeers, *ai)
  }

	return c
}

func (c *Discovery) Destroy() {
  c.l.Warning("TODO Destroy()")
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
    c.l.Warning("Bootstrap (%s): %s\n", peer.ID, err)
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
  c.l.Info("Bootstrapping %d peers", len(c.bootstrapPeers))
	for _, peer := range c.bootstrapPeers {
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
  if len(p.Addrs) == 0 {
    return // Don't add unreachable peers
  }
  if c.connect {
    err := c.h.Connect(c.ctx, p)
    if err != nil {
      c.l.Println("Failed connecting to ", p.ID.Pretty(), ", error:", err)
    } else {
      c.l.Println("Connected to:", p.ID.Pretty())
      c.notify(p)
    }
  } else if len(c.h.Peerstore().Addrs(p.ID)) == 0 {
    c.h.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
    //c.l.Println("Added peer to PeerStore:", p.ID.Pretty())
    c.notify(p)
  }
}

func (c *Discovery) notify(p peer.AddrInfo) {
  select {
    case c.onReady <- true:
    default:
  }
  select {
    case c.PeerDiscovered <- p:
    default:
  }
}

func (c *Discovery) discoverPeersMDNS(rendezvous string) {
	// srv calls HandlePeerFound()
	srv := mdns.NewMdnsService(c.h, rendezvous, c)
	if err := srv.Start(); err != nil {
    c.l.Error("MDNS: %v", err)
    return
	}
  select {
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
		peerChan, err := routingDiscovery.FindPeers(c.ctx, rendezvous)
		if err != nil {
      c.l.Error("DHT: %v", err)
      return
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

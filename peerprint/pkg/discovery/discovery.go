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
  discoveredPeers map[string]struct{}
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
    discoveredPeers: make(map[string]struct{}),
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
  } else {
    c.l.Info("Connected to bootstrap peer %s", peer.ID)
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
  c.l.Info("Bootstrapping DHT background thread")
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
  if _, ok := c.discoveredPeers[p.ID.String()]; ok {
    return
  }

  c.discoveredPeers[p.ID.String()] = struct{}{}
  if c.connect {
    err := c.h.Connect(c.ctx, p)
    if err != nil {
      c.l.Warning("Couldn't connect to: %s, error %w", p.ID.Pretty(), err)
    } else {
      c.l.Info("Connected to: %s", p.ID.Pretty())
      c.notify(p)
    }
  } else {
    c.h.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
    c.l.Info("Added peer to PeerStore: %s", p.ID.Pretty())
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

  c.l.Info("DHT initialized; beginning advertisement with rendezvous %s", rendezvous)
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(c.ctx, routingDiscovery, rendezvous)

	// Look for others who have announced and attempt to connect to them
	for {
		peerChan, err := routingDiscovery.FindPeers(c.ctx, rendezvous)
		if err != nil {
      c.l.Error("DHT: %v", err)
      continue
		}
		for peer := range peerChan {
			c.HandlePeerFound(peer)
		}
	}
}

func (c *Discovery) AwaitReady(ctx context.Context) error {
	select {
	case <-c.onReady:
    c.l.Info("Ready state achieved, returning from AwaitReady")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

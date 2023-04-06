package main

import (
  "fmt"
	libp2p "github.com/libp2p/go-libp2p"
  "github.com/smartin015/peerprint/p2pgit/pkg/discovery"
  "flag"
  "context"
  "log"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "os"
)

var (
  // Registry server flags
  addr = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Host address")
  rendezvous = flag.String("rendezvous", "testrendy", "Test rendezvous string")
  local = flag.Bool("local", false, "If true, use MDNS for discovery. If false, use DHT")
  logger = log.New(os.Stderr, "", 0)
)

func main() {
  flag.Parse()
  ctx := context.Background()

  // Initialize base pubsub infra
	h, err := libp2p.New(libp2p.ListenAddrStrings(*addr), libp2p.EnableRelay())
	if err != nil {
    panic(fmt.Errorf("PubSub host creation failure: %w", err))
	}
  logger.Printf("Host: %v %v\n", h.ID(), h.Addrs())

  // Initialize discovery service for pubsub
	disco := discovery.DHT
	if *local {
    logger.Println("Using MDNS (local) discovery")
		disco = discovery.MDNS
	} else {
    logger.Println("Using DHT (global) discovery")
  }

  logger.Printf("Rendezvous string: %s\n", *rendezvous)
  d := discovery.New(ctx, disco, h, *rendezvous, false, pplog.New("discovery", logger))

  logger.Printf("Listening for peers...")
  go d.Run()
  for {
    select {
    case p := <-d.PeerDiscovered:
      logger.Printf("Peer discovered: %+v\n", p)
    }
  }
}


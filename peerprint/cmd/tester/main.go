package main

import (
  "fmt"
	libp2p "github.com/libp2p/go-libp2p"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
  "github.com/libp2p/go-libp2p/core/network"
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
  connect = flag.Bool("connect", true, "Try to connect to discovered peers")
  logger = log.New(os.Stderr, "", 0)
)

func createResourceManager() network.ResourceManager {
	// Start with the default scaling limits.
	scalingLimits := rcmgr.DefaultLimits
	// Add limits around included libp2p protocols
	libp2p.SetDefaultServiceLimits(&scalingLimits)
	// Turn the scaling limits into a concrete set of limits using `.AutoScale`. This
	// scales the limits proportional to your system memory.
	scaledDefaultLimits := scalingLimits.AutoScale()
	// Tweak certain settings
	cfg := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			// Allow unlimited outbound streams
			StreamsOutbound: rcmgr.Unlimited,
		},
		// Everything else is default. The exact values will come from `scaledDefaultLimits` above.
	}
	// Create our limits by using our cfg and replacing the default values with values from `scaledDefaultLimits`
	limits := cfg.Build(scaledDefaultLimits)
	// The resource manager expects a limiter, se we create one from our limits.
	limiter := rcmgr.NewFixedLimiter(limits)
	// Initialize the resource manager
	rm, err := rcmgr.NewResourceManager(limiter)
	if err != nil {
		panic(err)
	}
  return rm
}

func main() {
  flag.Parse()
  ctx := context.Background()

	rm := createResourceManager()
	h, err := libp2p.New(libp2p.ListenAddrStrings(*addr), libp2p.EnableRelay(), libp2p.ResourceManager(rm))
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
  d := discovery.New(ctx, disco, h, *rendezvous, *connect, pplog.New("discovery", logger))

  logger.Printf("Listening for peers...")
  go d.Run()
  for {
    select {
    case p := <-d.PeerDiscovered:
      logger.Printf("Peer discovered: %+v\n", p)
    }
  }
}


package main

import (
  "github.com/smartin015/peerprint/p2pgit/transport"
  "github.com/smartin015/peerprint/p2pgit/storage"
  "github.com/smartin015/peerprint/p2pgit/server"
  "github.com/smartin015/peerprint/p2pgit/crypto"
	"github.com/libp2p/go-libp2p/core/pnet"
  "context"
  "flag"
  "fmt"
  "log"
  "time"
  "os"
)

var (
  // Command flags
  genPSKFlag = flag.Bool("genPSK", false, "Generate a PSK file and then exit")

  // Address flags
	addrFlag     = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to host the service")

  // Path flags
  dbPathFlag = flag.String("db", "/tmp/raft.db", "Path to database")
	privkeyfileFlag    = flag.String("privKeyPath", "/tmp/priv.key", "Path to serialized private key (if not present, one will be created at that location)")
	pubkeyfileFlag     = flag.String("pubKeyPath", "/tmp/pub.key", "Path to serialized public key (if not present, one will be created at that location)")
  pskFileFlag = flag.String("pskPath", "", "Path to pre-shared key for network security")

  // Other network flags
	rendezvousFlag     = flag.String("rendezvous", "", "String to use for discovery (required)")
	localFlag          = flag.Bool("local", true, "Use local MDNS (instead of global DHT) for discovery")

  // Timing flags
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
	statusPeriodFlag = flag.Duration("statusPeriod", 2*time.Minute, "Time between self-reports of status after initial setup")
  accessionDelayFlag = flag.Duration("accessionDelay", 5*time.Second, "Wait at least this long during handshaking phase before assuming leadership")

  logger = log.New(os.Stderr, "", 0)
)

func main() {
  flag.Parse()
  if *genPSKFlag {
    if *pskFileFlag == "" {
      panic("-genPSK requires -pskPath to be set")
    }
    if err := crypto.GenPSKFile(*pskFileFlag); err != nil {
      panic(err)
    }
    logger.Println("Wrote new PSK file to", *pskFileFlag)
    return
  }

	if *rendezvousFlag == "" {
		panic("-rendezvous must be specified!")
	}
	kpriv, kpub, err := crypto.LoadOrGenerateKeys(*privkeyfileFlag, *pubkeyfileFlag)
	if err != nil {
		panic(fmt.Errorf("Error loading keys: %w", err))
	}

  var psk pnet.PSK
  if *pskFileFlag == "" {
    logger.Println("\n\n\n ================= WARNING =================\n\n",
      "No PSK path is set - your session will be INSECURE\n",
      "It is STRONGLY RECOMMENDED to specify a PSK file with -pskPath\n",
      "or else anybody can become a node in your network\n",
      "\n ================= WARNING =================\n\n\n")
  } else {
    psk, err = crypto.LoadPSKFile(*pskFileFlag)
    if err != nil {
      panic(fmt.Errorf("Error loading PSK: %w", err))
    }
  }

  st := storage.NewInMemory()
  t, err := transport.New(&transport.Opts{
    PubsubAddr: *addrFlag,
    Rendezvous: *rendezvousFlag,
    Local: *localFlag,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: psk,
    ConnectTimeout: *connectTimeoutFlag,
    Topics: []string{server.DefaultTopic, server.StatusTopic},
  }, context.Background(), logger)
  if err != nil {
    panic(fmt.Errorf("Error initializing transport layer: %w", err))
  }
  s := server.New(t, st, &server.Opts{
    StatusPeriod: *statusPeriodFlag,
    AccessionDelay: *accessionDelayFlag,
  }, logger)
  if err := t.Register(server.PeerPrintProtocol, s.GetService()); err != nil {
    panic(fmt.Errorf("Failed to register RPC server: %w", err))
  }
  s.Run(context.Background())
}

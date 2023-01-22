package main

import (
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/automation"
	lp2p_crypto "github.com/libp2p/go-libp2p/core/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/cmd"
	"github.com/libp2p/go-libp2p/core/pnet"
	"github.com/libp2p/go-libp2p/core/peer"
  "context"
  "flag"
  "fmt"
  "log"
  "time"
  "os"
)

var (
  // Testing flags
  testQPSFlag   = flag.Float64("testQPS", 0, "set nonzero to automatically generate traffic")
  testRecordsFlag = flag.Int64("testRecordTarget", 100, "set target number of records to contribute to the queue")

  // Address flags
	addrFlag     = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to host the service")

  // Data flags
  inmemFlag     = flag.Bool("inmem", false, "use volatile/inmemory storage of keys and data. This overrides all other data flags")
  dbPathFlag = flag.String("db", "/tmp/raft.db", "Path to database")
	privkeyfileFlag    = flag.String("privKeyPath", "/tmp/priv.key", "Path to serialized private key (if not present, one will be created at that location)")
	pubkeyfileFlag     = flag.String("pubKeyPath", "/tmp/pub.key", "Path to serialized public key (if not present, one will be created at that location)")

  // Other network flags
	rendezvousFlag     = flag.String("rendezvous", "", "String to use for discovery (required)")
  pskFlag = flag.String("psk", "", "Pre-shared key for secure connection to the p2p network")
	localFlag          = flag.Bool("local", true, "Use local MDNS (instead of global DHT) for discovery")
  displayNameFlag = flag.String("displayName", "", "Human-readable name for this node")

  // Timing flags
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
	statusPeriodFlag = flag.Duration("statusPeriod", 2*time.Minute, "Time between self-reports of status after initial setup")
  syncPeriodFlag = flag.Duration("syncPeriod", 10*time.Minute, "Time between syncing with peers to correct missed data")

  // Safety flags
  maxRecordsPerPeerFlag = flag.Int64("maxRecordsPerPeer", 100, "Maximum number of records to allow each peer to store in our DB")
  maxCompletionsPerPeerFlag = flag.Int64("maxCompletionsPerPeer", 100, "Maximum number of completions to record from neighboring peers")
  maxTrackedPeersFlag = flag.Int64("maxTrackedPeers", 100,"Maximum number of peers for which we keep state information (including records and completions)")
  
  // IPC flags
	zmqRepFlag         = flag.String("zmq", "", "zmq server PAIR address (can be IPC, socket, etc.) defaults to none")
	zmqLogAddrFlag     = flag.String("zmqlog", "", "zmq server PAIR address (can be IPC, socket, etc.) defaults to none")

  logger = log.New(os.Stderr, "", 0)
)

func main() {
  flag.Parse()
	if *rendezvousFlag == "" {
		panic("-rendezvous must be specified!")
	}
	if *zmqLogAddrFlag != "" {
		var dlog cmd.Destructor
		logger, dlog = cmd.NewLog(*zmqLogAddrFlag)
		defer dlog()
	}

  var kpriv lp2p_crypto.PrivKey
  var kpub lp2p_crypto.PubKey
  var st storage.Interface
  var err error
  if *inmemFlag {
    kpriv, kpub, err = crypto.GenKeyPair()
    if err != nil {
      panic(fmt.Errorf("Error generating ephemeral keys: %w", err))
    }
    st, err = storage.NewSqlite3(":memory:")
    if err != nil {
      panic(fmt.Errorf("Error initializing inmemory DB: %w", err))
    }
  } else {
    kpriv, kpub, err = crypto.LoadOrGenerateKeys(*privkeyfileFlag, *pubkeyfileFlag)
    if err != nil {
      panic(fmt.Errorf("Error loading keys: %w", err))
    }

    st, err = storage.NewSqlite3(*dbPathFlag)
    if err != nil {
      panic(fmt.Errorf("Error initializing DB: %w", err))
    }
  }

  var psk pnet.PSK
  if *pskFlag == "" {
    logger.Println("\n\n\n ================= WARNING =================\n\n",
      "No PSK path is set - your session will be INSECURE\n",
      "It is STRONGLY RECOMMENDED to specify a PSK file with -pskPath\n",
      "or else anybody can become a node in your network\n",
      "\n ================= WARNING =================\n\n\n")
  } else {
    psk = crypto.LoadPSK(*pskFlag)
    logger.Printf("PSK: %x\n", []byte(psk))
  }

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

  id, err := peer.IDFromPublicKey(kpub)
  name := id.Pretty()
  name = name[len(name)-4:]
  s := server.New(t, st, &server.Opts{
    StatusPeriod: *statusPeriodFlag,
    SyncPeriod: *syncPeriodFlag,
    DisplayName: *displayNameFlag,
    MaxRecordsPerPeer: *maxRecordsPerPeerFlag,
    MaxCompletionsPerPeer: *maxCompletionsPerPeerFlag,
    MaxTrackedPeers: *maxTrackedPeersFlag,
  }, pplog.New(name, logger))
  go s.Run(context.Background())

  // Initialize command service if specified
  if *zmqRepFlag != "" {
    /*
    s.cmdRecv = make(chan proto.Message, 5)
    s.errChan = make(chan error, 5)
	  s.cmdSend = cmd.New(*zmqRepFlag, s.cmdRecv, s.errChan)
    */
    panic("TODO handle ZMQ control")
  } else if *testQPSFlag > 0.0 {
    a := automation.NewLoadTester(*testQPSFlag, *testRecordsFlag, s, st)
    a.Run(context.Background())
  }
}

/*
func (s *Transport) ReplyCmd(msg proto.Message) error {
  select {
  case s.cmdSend<- msg:
  default:
    return fmt.Errorf("ReplyCmd() error: channel full")
  }
  return nil
}
*/

package main

import (
  "google.golang.org/protobuf/proto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
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
  dbPathFlag = flag.String("db", ":memory:", "Path to database (use :memory: for ephemeral, inmemory DB")
	privkeyfileFlag    = flag.String("privKeyPath", "", "Path to serialized private key - default inmemory (if not present, one will be created at that location)")
	pubkeyfileFlag     = flag.String("pubKeyPath", "", "Path to serialized public key - default inmemory (if not present, one will be created at that location)")

  // Other network flags
	rendezvousFlag     = flag.String("rendezvous", "", "String to use for discovery (required)")
  pskFlag = flag.String("psk", "", "Pre-shared key for secure connection to the p2p network")
	localFlag          = flag.Bool("local", true, "Use local MDNS (instead of global DHT) for discovery")
  displayNameFlag = flag.String("displayName", "", "Human-readable name for this node")

  // Timing flags
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
	statusPeriodFlag = flag.Duration("statusPeriod", 2*time.Minute, "Time between self-reports of status after initial setup")
  syncPeriodFlag = flag.Duration("syncPeriod", 10*time.Minute, "Time between syncing with peers to correct missed data")

  // Safety and cleanup flags
  maxRecordsPerPeerFlag = flag.Int64("maxRecordsPerPeer", 100, "Maximum number of records to allow each peer to store in our DB")
  maxCompletionsPerPeerFlag = flag.Int64("maxCompletionsPerPeer", 100, "Maximum number of completions to record from neighboring peers")
  maxTrackedPeersFlag = flag.Int64("maxTrackedPeers", 100,"Maximum number of peers for which we keep state information (including records and completions)")
  trustCleanupThresholdFlag = flag.Float64("trustCleanupThreshold", 2.0, "Trust value to consider a peer as 'trusted' when cleaning up DB entries")
  trustCleanupTTLFlag = flag.Duration("trustCleanupTTL", 10*time.Day, "Amount of time before peers are considered for removal")
  trustedPeersFlag = flag.String("trustedPeers", "", "peer IDs to trust implicitly")

  // IPC flags
	zmqRepFlag         = flag.String("zmq", "", "zmq server REP address (can be IPC, socket, etc.), required")
	zmqPushFlag         = flag.String("zmqPush", "", "zmq server PUSH address (can be IPC, socket, etc.), required")
	zmqLogAddrFlag     = flag.String("zmqLog", "", "zmq server PUSH address (can be IPC, socket, etc.) defaults to none")

  logger = log.New(os.Stderr, "", 0)
)

func main() {
  flag.Parse()
	if *rendezvousFlag == "" {
		panic("-rendezvous must be specified!")
	}
  if *zmqRepFlag == "" {
    panic("-zmq must be specified!")
  }
  if *zmqPushFlag == "" {
    panic("-zmqPush must be specified!")
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
  if *privkeyfileFlag == "" && *pubkeyfileFlag == "" {
    kpriv, kpub, err = crypto.GenKeyPair()
    if err != nil {
      panic(fmt.Errorf("Error generating ephemeral keys: %w", err))
    }
  } else {
    kpriv, kpub, err = crypto.LoadOrGenerateKeys(*privkeyfileFlag, *pubkeyfileFlag)
    if err != nil {
      panic(fmt.Errorf("Error loading keys: %w", err))
    }
  }

  st, err = storage.NewSqlite3(*dbPathFlag)
  if err != nil {
    panic(fmt.Errorf("Error initializing DB: %w", err))
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
  loopZMQ(s)
}

func loopZMQ(s server.Interface) {
  cmdRecv := make(chan proto.Message, 5)
  errChan := make(chan error, 5)
  cmdSend, cmdPush := cmd.New(*zmqRepFlag, *zmqPushFlag, cmdRecv, errChan)
  for {
    select {
    case <-s.OnUpdate():
      logger.Println("Pushing update notification")
      cmdPush<- &pb.StateUpdate{}
    case e := <-errChan:
      logger.Println(e.Error())
    case c := <-cmdRecv:
      logger.Println("Handling command")
      rep, err := handleCommand(c)
      if err != nil {
        cmdSend<- &pb.Error{Reason: err.Error()}
      } else {
        cmdSend<- rep
      }
    }
  }
}

func handleCommand(c proto.Message) (proto.Message, error) {
  switch v := c.(type) {
  case *pb.Record:
    return nil, fmt.Errorf("TODO implement Record setting: %v", v)
  case *pb.Completion:
    return nil, fmt.Errorf("TODO implement Completion setting: %v", v)
  default:
    return nil, fmt.Errorf("Unrecognized command")
  }
}

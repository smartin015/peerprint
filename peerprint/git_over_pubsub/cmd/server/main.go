package main

import (
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/cmd"
	"github.com/libp2p/go-libp2p/core/pnet"
  "context"
  "flag"
  "fmt"
  "log"
  "time"
  "os"
)

var (
  // Address flags
	addrFlag     = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to host the service")

  // Path flags
  dbPathFlag = flag.String("db", "/tmp/raft.db", "Path to database")
	privkeyfileFlag    = flag.String("privKeyPath", "/tmp/priv.key", "Path to serialized private key (if not present, one will be created at that location)")
	pubkeyfileFlag     = flag.String("pubKeyPath", "/tmp/pub.key", "Path to serialized public key (if not present, one will be created at that location)")

  // Other network flags
	rendezvousFlag     = flag.String("rendezvous", "", "String to use for discovery (required)")
  pskFlag = flag.String("psk", "", "Pre-shared key for secure connection to the p2p network")
	localFlag          = flag.Bool("local", true, "Use local MDNS (instead of global DHT) for discovery")

  // Timing flags
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
	statusPeriodFlag = flag.Duration("statusPeriod", 2*time.Minute, "Time between self-reports of status after initial setup")
  accessionDelayFlag = flag.Duration("accessionDelay", 5*time.Second, "Wait at least this long during handshaking phase before assuming leadership")

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

	kpriv, kpub, err := crypto.LoadOrGenerateKeys(*privkeyfileFlag, *pubkeyfileFlag)
	if err != nil {
		panic(fmt.Errorf("Error loading keys: %w", err))
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

  st, err := storage.NewSqlite3(*dbPathFlag)
  if err != nil {
    panic(fmt.Errorf("Error initializing DB: %w", err))
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

  s := server.New(t, st, &server.Opts{
    StatusPeriod: *statusPeriodFlag,
    AccessionDelay: *accessionDelayFlag,
  }, logger)
  if err := t.Register(server.PeerPrintProtocol, s.GetService()); err != nil {
    panic(fmt.Errorf("Failed to register RPC server: %w", err))
  }
  go s.Run(context.Background())

  // Initialize command service if specified
  if *zmqRepFlag != "" {
    s.cmdRecv = make(chan proto.Message, 5)
    s.errChan = make(chan error, 5)
	  s.cmdSend = cmd.New(*zmqRepFlag, s.cmdRecv, s.errChan)
  }
}



func (s *Transport) ReplyCmd(msg proto.Message) error {
  select {
  case s.cmdSend<- msg:
  default:
    return fmt.Errorf("ReplyCmd() error: channel full")
  }
  return nil
}

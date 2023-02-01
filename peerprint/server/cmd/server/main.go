package main

import (
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/www"
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
  // Address flags
	addrFlag     = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to host the service")
  wwwFlag      = flag.String("www", "localhost:0", "Address for hosting status page - set empty to disable")

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
  syncPeriodFlag = flag.Duration("syncPeriod", 10*time.Minute, "Time between syncing with peers to correct missed data")
  watchdogFlag = flag.Duration("wdt", 10*time.Second, "Time before exit after no interaction - 0 to disable")

  // Safety and cleanup flags
  maxRecordsPerPeerFlag = flag.Int64("maxRecordsPerPeer", 15, "Maximum number of records to allow each peer to store in our DB")
  maxTrackedPeersFlag = flag.Int64("maxTrackedPeers", 100,"Maximum number of peers for which we keep state information (including records and completions)")

  // IPC flags
	zmqRepFlag         = flag.String("zmq", "", "zmq server REP address (can be IPC, socket, etc.), required")
	zmqPushFlag         = flag.String("zmqPush", "", "zmq server PUSH address (can be IPC, socket, etc.), required")
	zmqLogAddrFlag     = flag.String("zmqLog", "", "zmq server PUSH address (can be IPC, socket, etc.) defaults to none")

  logger = log.New(os.Stderr, "", 0)
)

func main() {
  flag.Parse()

	if *zmqLogAddrFlag != "" {
		var dlog cmd.Destructor
		logger, dlog = cmd.NewLog(*zmqLogAddrFlag)
		defer dlog()
	}

  var st storage.Interface
  var err error
  st, err = storage.NewSqlite3(*dbPathFlag)
  if err != nil {
    panic(fmt.Errorf("Error initializing DB: %w", err))
  }
  storage.SetPanicHandler(st)
  defer storage.HandlePanic()

	if *rendezvousFlag == "" {
		panic("-rendezvous must be specified!")
	}
  if *zmqRepFlag == "" {
    panic("-zmq must be specified!")
  }
  if *zmqPushFlag == "" {
    panic("-zmqPush must be specified!")
  }

  var kpriv lp2p_crypto.PrivKey
  var kpub lp2p_crypto.PubKey
  if *privkeyfileFlag == "" && *pubkeyfileFlag == "" {
    logger.Printf("WARNING: generating ephemeral key pair; this will change on restart")
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

  ctx := context.Background()
  t, err := transport.New(&transport.Opts{
    PubsubAddr: *addrFlag,
    Rendezvous: *rendezvousFlag,
    Local: *localFlag,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: psk,
    ConnectTimeout: *connectTimeoutFlag,
    Topics: []string{server.DefaultTopic},
  }, ctx, logger)
  if err != nil {
    panic(fmt.Errorf("Error initializing transport layer: %w", err))
  }

  id, err := peer.IDFromPublicKey(kpub)
  name := id.Pretty()
  name = name[len(name)-4:]
	st.SetId(id.String())

  s := server.New(t, st, &server.Opts{
    SyncPeriod: *syncPeriodFlag,
    DisplayName: *displayNameFlag,
    MaxRecordsPerPeer: *maxRecordsPerPeerFlag,
    MaxTrackedPeers: *maxTrackedPeersFlag,
  }, pplog.New(name, logger))

  if *wwwFlag != "" {
    wsrv := www.New(pplog.New("www", logger), s, st)
    go wsrv.Serve(*wwwFlag, ctx)
  }


  go s.Run(ctx)

  d := driver.NewDriver(s, st, t, pplog.New("cmd", logger))
  d.Loop(ctx, *zmqRepFlag, *zmqPushFlag, *watchdogFlag)
}


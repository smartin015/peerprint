package main

import (
  "google.golang.org/protobuf/proto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
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
  watchdogFlag = flag.Duration("wdt", 3*time.Second, "Time before exit after no interaction")

  // Safety and cleanup flags
  maxRecordsPerPeerFlag = flag.Int64("maxRecordsPerPeer", 100, "Maximum number of records to allow each peer to store in our DB")
  maxCompletionsPerPeerFlag = flag.Int64("maxCompletionsPerPeer", 100, "Maximum number of completions to record from neighboring peers")
  maxTrackedPeersFlag = flag.Int64("maxTrackedPeers", 100,"Maximum number of peers for which we keep state information (including records and completions)")
  trustCleanupThresholdFlag = flag.Float64("trustCleanupThreshold", 2.0, "Trust value to consider a peer as 'trusted' when cleaning up DB entries")
  trustCleanupTTLFlag = flag.Duration("trustCleanupTTL", 24*10*time.Hour, "Amount of time before peers are considered for removal")
  trustRebroadcastThresholdFlag = flag.Float64("trustRebroadcastThreshold", 1.0, "How much to trust a peer before rebroadcasting its assertion that it's working on our record - this prevents other peers from speculatively working on the same record")
  trustedPeersFlag = flag.String("trustedPeers", "", "peer IDs to trust implicitly")

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

  t, err := transport.New(&transport.Opts{
    PubsubAddr: *addrFlag,
    Rendezvous: *rendezvousFlag,
    Local: *localFlag,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: psk,
    ConnectTimeout: *connectTimeoutFlag,
    Topics: []string{server.DefaultTopic},
  }, context.Background(), logger)
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
    MaxCompletionsPerPeer: *maxCompletionsPerPeerFlag,
    MaxTrackedPeers: *maxTrackedPeersFlag,
    TrustRebroadcastThreshold: *trustRebroadcastThresholdFlag,
  }, pplog.New(name, logger))

  if *wwwFlag != "" {
    wsrv := www.New(pplog.New("www", logger), s, st)
    go wsrv.Serve(*wwwFlag, context.Background())
  }


  go s.Run(context.Background())
  d := &driver{
    s: s,
    st: st,
    l: pplog.New("cmd", logger),
  }
  d.Loop()
}

type driver struct {
  s server.Interface
  st storage.Interface
  l *pplog.Sublog
}

func (d *driver) Loop() {
  cmdRecv := make(chan proto.Message, 5)
  errChan := make(chan error, 5)
  cmdSend, cmdPush := cmd.New(*zmqRepFlag, *zmqPushFlag, cmdRecv, errChan)
  wdt := time.NewTimer(*watchdogFlag)
  for {
    select {
    case m := <-d.s.OnUpdate():
      cmdPush<- m
    case e := <-errChan:
      d.l.Error(e.Error())
    case c := <-cmdRecv:
      rep, err := d.handleCommand(c)
      if err != nil {
        cmdSend<- &pb.Error{Reason: err.Error()}
      } else {
        cmdSend<- rep
      }
    case <-wdt.C:
      d.l.Error("Watchdog timeout, exiting")
      return
    }
    wdt.Reset(*watchdogFlag)
  }
}

func (d *driver) handleCommand(c proto.Message) (proto.Message, error) {
  switch v := c.(type) {
  case *pb.HealthCheck:
    return &pb.HealthCheck{}, nil
  case *pb.GetID:
    return &pb.IDResponse{
      Id: d.s.ID(),
    }, nil
  case *pb.Record:
    return d.s.IssueRecord(v, true)
  case *pb.Completion:
    return d.s.IssueCompletion(v, true)
  case *pb.SetTrust:
    if err := d.st.SetTrust(v.Peer, v.Trust); err != nil {
      return nil, err
    } else {
      return &pb.Ok{}, nil
    }
  case *pb.SetWorkability:
    if err := d.st.SetWorkability(v.Uuid, v.Workability); err != nil {
      return nil, err
    } else {
      return &pb.Ok{}, nil
    }
  default:
    return nil, fmt.Errorf("Unrecognized command")
  }
}

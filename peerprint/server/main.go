package main

import (
	"context"
	"flag"
	"fmt"
	libp2p "github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/smartin015/peerprint/peerprint_server/cmd"
	"github.com/smartin015/peerprint/peerprint_server/discovery"
	"github.com/smartin015/peerprint/peerprint_server/poll"
	"github.com/smartin015/peerprint/peerprint_server/raft"
	reggen "github.com/smartin015/peerprint/peerprint_server/registry_generator"
	"github.com/smartin015/peerprint/peerprint_server/server"
	tr "github.com/smartin015/peerprint/peerprint_server/topic_receiver"
	"google.golang.org/protobuf/proto"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	pubsubAddrFlag     = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to join for pubsub")
	raftAddrFlag       = flag.String("raftAddr", "/ip4/0.0.0.0/tcp/0", "Address to join for RAFT consensus")
	raftPathFlag       = flag.String("raftPath", "./state.raft", "Path to raft state snapshot")
	rendezvousFlag     = flag.String("rendezvous", "", "String to use for discovery (required")
	trustedPeersFlag   = flag.String("trustedPeers", "", "Comma-separated list of peer IDs to consider as trusted")
	localFlag          = flag.Bool("local", true, "Use local MDNS (instead of global DHT) for discovery")
	privkeyfileFlag    = flag.String("privkeyfile", "./priv.key", "Path to serialized private key (if not present, one will be created at that location)")
	pubkeyfileFlag     = flag.String("pubkeyfile", "./pub.key", "Path to serialized public key (if not present, one will be created at that location)")
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
	zmqRepFlag         = flag.String("zmq", "", "zmq server PAIR address (can be IPC, socket, etc.) defaults to none")
	zmqPushFlag        = flag.String("zmqpush", "", "zmq server PUSH address (can be IPC, socket, etc.) defaults to none")
	zmqLogAddrFlag     = flag.String("zmqlog", "", "zmq server PAIR address (can be IPC, socket, etc.) defaults to none")
	logger             = log.New(os.Stderr, "", 0)
)

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func loadOrGenerateKeys(privkeyFile string, pubkeyFile string) (crypto.PrivKey, crypto.PubKey, error) {
	privEx := fileExists(privkeyFile)
	pubEx := fileExists(pubkeyFile)
	if pubEx != privEx {
		return nil, nil, fmt.Errorf("Partial existance of public/private keys, cannot continue: (public %v, private %v)", pubEx, privEx)
	}

	if privEx && pubEx {
		return reggen.LoadKeys(privkeyFile, pubkeyFile)
	} else {
		return reggen.GenKeyPairFile(privkeyFile, pubkeyFile)
	}
}

func main() {
	if os.Args[1] == "generate_registry" {
		if len(os.Args) != 4 {
			log.Fatal("Usage ", os.Args[0], " generate_registry <num_peers> <dest_dir>")
		}
		npeer, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal("generate_registry: <npeer> argument not an integer")
		}
		dest_dir := os.Args[3]
		if err := reggen.GenRegistryFiles(npeer, dest_dir); err != nil {
			log.Fatal(fmt.Errorf("Error writing generated registry to %s: %v", dest_dir, err).Error())
		}
		fmt.Println("Generated registry YAML file and peer keys in dir", dest_dir)
		return
	}

	flag.Parse()
	if *rendezvousFlag == "" {
		panic("-rendezvous must be specified!")
	}
	if *trustedPeersFlag == "" {
		panic("-trustedPeers must be specified!")
	}
	if *zmqLogAddrFlag != "" {
		var dlog cmd.Destructor
		logger, dlog = cmd.NewLog(*zmqLogAddrFlag)
		defer dlog()
	}

	tpstr := ""
	tps := []string{}
	for _, tp := range strings.Split(*trustedPeersFlag, ",") {
		tpstr = tpstr + fmt.Sprintf("  - %s\n", tp)
		tps = append(tps, strings.TrimSpace(tp))
	}
	logger.Printf("Config:\n\tRendezvous:%s\n\tTrusted Peers:\n%s\n", *rendezvousFlag, tpstr)

	ctx := context.Background()
	kpriv, _, err := loadOrGenerateKeys(*privkeyfileFlag, *pubkeyfileFlag)
	if err != nil {
		panic(fmt.Errorf("Error loading keys: %w", err))
	}

	h, err := libp2p.New(libp2p.ListenAddrStrings(*pubsubAddrFlag), libp2p.Identity(kpriv))
	if err != nil {
		panic(err)
	}
	rh, err := libp2p.New(libp2p.ListenAddrStrings(*raftAddrFlag), libp2p.Identity(kpriv))
	if err != nil {
		panic(err)
	}

	logger.Printf("Discovering pubsub peers (self ID %v, timeout %v)\n", h.ID().String(), *connectTimeoutFlag)
	disco := discovery.DHT
	if *localFlag {
		disco = discovery.MDNS
	}
	d := discovery.New(ctx, disco, h, *rendezvousFlag, logger)
	connectCtx, cancel := context.WithTimeout(ctx, *connectTimeoutFlag)
	defer cancel()
	if err := d.AwaitReady(connectCtx); err != nil {
		panic(fmt.Errorf("Error connecting to peers: %w", err))
	} else {
		logger.Println("Peers found; discovery complete")
	}

	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		panic(err)
	}
	r := raft.New(context.Background(), rh, *raftPathFlag, logger)

	cmdRecvChan := make(chan proto.Message, 5)
	subChan := make(chan tr.TopicMsg, 5)
	errChan := make(chan error, 5)

	openFn := func(topic string) (chan<- proto.Message, error) {
		return tr.NewTopicChannel(ctx, subChan, h.ID().String(), ps, topic, errChan)
	}

	cmdSend, cmdPush := cmd.New(*zmqRepFlag, *zmqPushFlag, cmdRecvChan, errChan)
	logger.Println("ZMQ sockets at", *zmqRepFlag, *zmqPushFlag)

	poller := poll.New(ctx)

	s := server.New(server.ServerOptions{
		ID:           h.ID().String(),
		TrustedPeers: tps,
		Logger:       logger,
		Raft:         r,
		Poller:       poller,

		RecvPubsub: subChan,
		RecvCmd:    cmdRecvChan,
		SendCmd:    cmdSend,
		PushCmd:    cmdPush,

		Opener: openFn,
	})
	s.Loop(ctx)
}

package main

import (
  "strings"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/ghodss/yaml"
	ipfs "github.com/ipfs/go-ipfs-api"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/smartin015/peerprint/pubsub/conn"
	pb "github.com/smartin015/peerprint/pubsub/proto"
	"github.com/smartin015/peerprint/pubsub/prpc"
	"github.com/smartin015/peerprint/pubsub/server"
	"github.com/smartin015/peerprint/pubsub/cmd"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"log"
	"os"
	"time"
)

var (
	pubsubAddrFlag     = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to join for pubsub")
	raftAddrFlag       = flag.String("raftAddr", "/ip4/0.0.0.0/tcp/0", "Address to join for RAFT consensus")
	raftPathFlag       = flag.String("raftPath", "./state.raft", "Path to raft state snapshot")
	registryFlag       = flag.String("registry", "QmZQ4bLHRCrcmJUnTbY7updR6chfvumbaEx6cCya3chz9n", "IPFS content ID of the queue (required)")
	queueFlag          = flag.String("queue", "Test queue", "Name of the registered queue  (required)")
	ipfsServerFlag     = flag.String("ipfs_server", "localhost:5001", "Route to the IPFS daemon / server")
	localFlag          = flag.Bool("local", false, "Use local MDNS (instead of global DHT) for discovery")
	privkeyfileFlag    = flag.String("privkeyfile", "./priv.key", "Path to serialized private key (if not present, one will be created at that location)")
	pubkeyfileFlag     = flag.String("pubkeyfile", "./pub.key", "Path to serialized public key (if not present, one will be created at that location)")
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
  zmqAddrFlag        = flag.String("zmq", "", "zmq server PAIR address (can be IPC, socket, etc.) defaults to none")
  zmqLogAddrFlag        = flag.String("zmqlog", "", "zmq server PAIR address (can be IPC, socket, etc.) defaults to none")
  bootstrapFlag      = flag.Bool("bootstrap", false, "Bootstrap storage (run this once for a new queue")
	logger = log.New(os.Stderr, "", 0)
)

func getFileAsJSON(cid string) ([]byte, error) {
  var fh io.ReadCloser
  var err error
  if strings.Contains(cid, ".y") {
    fh, err = os.Open(cid)
  } else {
    // TODO this assumes the daemon is running - should probably
    // guard this
    sh := ipfs.NewShell(*ipfsServerFlag)
    fh, err = sh.Cat(cid)
  }
  if err != nil {
    return nil, fmt.Errorf("failed to open %s: %w", cid, err)
  }
  defer fh.Close()

	y := make([]byte, 4096)
  n, err := fh.Read(y)
  if err != nil && err != io.EOF {
    return nil, fmt.Errorf("ipfs Read() error: %w", err)
  }
	return yaml.YAMLToJSON(y[:n])
}

func getRegistry(cid string) (*pb.Registry, error) {
	j, err := getFileAsJSON(cid)
	if err != nil {
		return nil, err
	}
	m := &pb.Registry{}
	err = protojson.Unmarshal(j, m)
	if err != nil {
		return nil, fmt.Errorf("Registry unmarshal error: %w", err)
	}
	return m, nil
}

func getQueueFromRegistry(cid string, queue string) (*pb.Queue, error) {
	reg, err := getRegistry(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry %v: %w", cid, err)
	}
	for _, q := range reg.Queues {
		if q.Name == queue {
			return q, nil
		}
	}
	return nil, fmt.Errorf("failed to find queue %v in registry %v (%d entries)", queue, cid, len(reg.Queues))
}

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
		data, err := os.ReadFile(privkeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("Read %s: %w", privkeyFile, err)
		}
		priv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, nil, fmt.Errorf("UnmarshalPrivateKey: %w", err)
		}

		data, err = os.ReadFile(pubkeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("Read %s: %w", pubkeyFile, err)
		}
		pub, err := crypto.UnmarshalPublicKey(data)
		if err != nil {
			return nil, nil, fmt.Errorf("UnmarshalPublicKey: %w", err)
		}
		return priv, pub, nil
	} else {
		priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, nil, fmt.Errorf("Generating keypair error: %w", err)
		}
		data, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return nil, nil, fmt.Errorf("Marshal private key: %w", err)
		}
		if err := os.WriteFile(privkeyFile, data, 0644); err != nil {
			return nil, nil, fmt.Errorf("Write %s: %w", privkeyFile, err)
		}
		data, err = crypto.MarshalPublicKey(pub)
		if err != nil {
			return nil, nil, fmt.Errorf("Marshal public key: %w", err)
		}
		if err := os.WriteFile(pubkeyFile, data, 0644); err != nil {
			return nil, nil, fmt.Errorf("Write %s: %w", pubkeyFile, err)
		}
		return priv, pub, nil
	}
}

func main() {
	flag.Parse()
	if *registryFlag == "" {
		panic("-registry must be specified!")
	}
	if *queueFlag == "" {
		panic("-queue must be specified!")
	}
  if *zmqLogAddrFlag != "" {
    logger = cmd.NewLog(*zmqLogAddrFlag)
  }

	logger.Printf("Fetching queue %v details from registry %v", *queueFlag, *registryFlag)
	queue, err := getQueueFromRegistry(*registryFlag, *queueFlag)
	if err != nil {
		panic(fmt.Errorf("Error fetching queue: %w", err))
	}

	logger.Printf("Queue:\n- Name: %v\n- Description: %v\n- URL: %v\n- Trusted Peers: %d\n\n", queue.Name, queue.Desc, queue.Url, len(queue.TrustedPeers))

	ctx := context.Background()
	logger.Printf("Loading keys...")
	kpriv, _, err := loadOrGenerateKeys(*privkeyfileFlag, *pubkeyfileFlag)
	if err != nil {
		panic(fmt.Errorf("Error loading keys: %w", err))
	}
	c := conn.New(ctx, *localFlag, *pubsubAddrFlag, queue.Rendezvous, kpriv, logger)
	logger.Printf("Discovering pubsub peers (ID %v, timeout %v)\n", c.GetID(), *connectTimeoutFlag)
	connectCtx, _ := context.WithTimeout(ctx, *connectTimeoutFlag)
	if err := c.AwaitReady(connectCtx); err != nil {
		panic(fmt.Errorf("Error connecting to peers: %w", err))
	} else {
		logger.Println("Peers found; discovery complete")
	}
	p := prpc.New(c.GetID(), c.GetPubSub())

	logger.Println("PRPC interface established")
	s := server.New(ctx, p, queue.TrustedPeers, *raftAddrFlag, *raftPathFlag, *zmqAddrFlag, *bootstrapFlag, kpriv, logger)

	logger.Println("Entering main loop")
	s.Loop()
}

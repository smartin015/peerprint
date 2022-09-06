package main

import (
  "io"
  "flag"
  "os"
  "context"
  "log"
  "time"
  "fmt"
	"crypto/rand"
  pb "github.com/smartin015/peerprint/pubsub/proto"
	"github.com/libp2p/go-libp2p-core/crypto"
  "google.golang.org/protobuf/encoding/protojson"
  "github.com/smartin015/peerprint/pubsub/conn"
  "github.com/smartin015/peerprint/pubsub/prpc"
  "github.com/smartin015/peerprint/pubsub/server"
  ipfs "github.com/ipfs/go-ipfs-api"
  "github.com/ghodss/yaml"
)

var (
  addrFlag = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to join")
	registryFlag = flag.String("registry", "QmRztyRjsMCsC3vMx7NFo85fCV5i4zVtbFeSJwSRQhNTxP", "IPFS content ID of the queue (required)")
	queueFlag = flag.String("queue", "Test queue", "Name of the registered queue  (required)")
  ipfsServerFlag = flag.String("ipfs_server", "localhost:5001", "Route to the IPFS daemon / server")
	localFlag = flag.Bool("local", false, "Use local MDNS (instead of global DHT) for discovery")
	privkeyfileFlag = flag.String("privkeyfile", "./priv.key", "Path to serialized private key (if not present, one will be created at that location)")
	pubkeyfileFlag = flag.String("pubkeyfile", "./pub.key", "Path to serialized public key (if not present, one will be created at that location)")
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
  stderr = log.New(os.Stderr, "", 0)
)

func getRegistry(cid string) (*pb.Registry, error) {
  // TODO this assumes the daemon is running - should probably
  // guard this
  sh := ipfs.NewShell(*ipfsServerFlag)
  fh, err := sh.Cat(cid)
  if err != nil {
    return nil, fmt.Errorf("ipfs Cat() error: %w", err)
  } else {
  }
  defer fh.Close()
  y := make([]byte, 4096)
  n, err := fh.Read(y)
  if err != nil && err != io.EOF {
    return nil, fmt.Errorf("ipfs Read() error: %w", err)
  }
  j, err := yaml.YAMLToJSON(y[:n])
  if err != nil {
    return nil, fmt.Errorf("ipfs JSON convert error: %w", err)
  }

  m := &pb.Registry{}
  err = protojson.Unmarshal(j, m)
  if err != nil {
    return nil, fmt.Errorf("ipfs Unmarshal() error: %w", err)
  }
  return m, nil
}

func getQueueFromRegistry(cid string, queue string) (*pb.Queue, error) {
  reg, err := getRegistry(cid)
  if err != nil {
    return nil, fmt.Errorf("failed to get registry %v: %w", cid, err)
  }
  for _, q := range(reg.Queues) {
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

  stderr.Printf("Fetching queue %v details from registry %v", *queueFlag, *registryFlag)
  queue, err := getQueueFromRegistry(*registryFlag, *queueFlag)
  if err != nil {
    panic(fmt.Errorf("Error fetching queue: %w", err))
  }

  log.Printf("Queue:\n- Name: %v\n- Description: %v\n- URL: %v\n- Trusted Peers: %d\n\n", queue.Name, queue.Desc, queue.Url, len(queue.TrustedPeers))

	ctx := context.Background()
	log.Printf("Loading keys...")
	kpriv, _, err := loadOrGenerateKeys(*privkeyfileFlag, *pubkeyfileFlag)
	if err != nil {
		panic(fmt.Errorf("Error loading keys: %w", err))
	}
  c := conn.New(ctx, *localFlag, *addrFlag, queue.Rendezvous, kpriv)
  log.Printf("Connecting (ID %v)\n", c.GetID())
  connectCtx, _ := context.WithTimeout(ctx, *connectTimeoutFlag)
  if err := c.AwaitReady(connectCtx); err != nil {
    panic(fmt.Errorf("Error connecting to peers: %w", err))
  } else {
    log.Println("Peers found; ending search")
  }
  p := prpc.New(c.GetID(), c.GetPubSub())

  log.Println("PRPC interface established, self ID")
  s := server.New(ctx, p, queue.TrustedPeers, stderr)
  log.Println("Entering main loop")
  s.Loop()
}


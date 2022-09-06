package main

import (
  "io"
  "flag"
  "os"
  "context"
  "log"
  "time"
  "fmt"
  pb "github.com/smartin015/peerprint/pubsub/proto"
  "google.golang.org/protobuf/encoding/protojson"
  "github.com/smartin015/peerprint/pubsub/conn"
  "github.com/smartin015/peerprint/pubsub/prpc"
  "github.com/smartin015/peerprint/pubsub/server"
  ipfs "github.com/ipfs/go-ipfs-api"
  "github.com/ghodss/yaml"
)

var (
  addrFlag = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to join")
	registryFlag = flag.String("registry", "QmYLZq7ioLYiK3TYQvpHzV5kz1997s4dcPUoEUMPWWM2vo", "IPFS content ID of the queue (required)")
	queueFlag = flag.String("queue", "Test queue", "Name of the registered queue  (required)")
  ipfsServerFlag = flag.String("ipfs_server", "localhost:5001", "Route to the IPFS daemon / server")
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

  log.Printf("Queue:\n- Name: %v\n- Description: %v\n- URL: %v\n\n", queue.Name, queue.Desc, queue.Url)

	ctx := context.Background()
  log.Println("Connecting...")
  c := conn.New(ctx, *addrFlag, queue.Rendezvous)
  connectCtx, _ := context.WithTimeout(ctx, *connectTimeoutFlag)
  if err := c.AwaitReady(connectCtx); err != nil {
    panic(fmt.Errorf("Error connecting to peers: %w", err))
  } else {
    log.Println("Peers found; ending search")
  }
  p := prpc.New(c.GetID(), c.GetPubSub())

  log.Println("PRPC interface established")
  s := server.New(ctx, p, queue.TrustedPeers, stderr)
  log.Println("Entering main loop")
  s.Loop()
}


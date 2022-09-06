package main

import (
  "flag"
  "os"
  "context"
  "log"
  "time"
  "fmt"
  "github.com/smartin015/peerprint/pubsub/conn"
  "github.com/smartin015/peerprint/pubsub/prpc"
  "github.com/smartin015/peerprint/pubsub/server"
  // pb "github.com/smartin015/peerprint/pubsub/proto"
)

var (
  addrFlag = flag.String("addr", "/ip4/0.0.0.0/tcp/0", "Address to join")
	queueFlag = flag.String("queueName", "", "Name of the queue to join - must be specified")
	connectTimeoutFlag = flag.Duration("connectTimeout", 2*time.Minute, "How long to wait for initial connection")
  stderr = log.New(os.Stderr, "", 0)
)


func main() {
	flag.Parse()
  if *queueFlag == "" {
    panic("-queueName must be specified!")
  }

	ctx := context.Background()
  c := conn.New(ctx, *addrFlag, *queueFlag)
  log.Println("Connecting...")
  connectCtx, _ := context.WithTimeout(ctx, *connectTimeoutFlag)
  if err := c.AwaitReady(connectCtx); err != nil {
    panic(fmt.Errorf("Error connecting to peers: %w", err))
  } else {
    log.Println("Peers found; ending search")
  }
  p := prpc.New(c.GetID(), c.GetPubSub())

  log.Println("PRPC interface established")
  s := server.New(ctx, p, stderr)
  log.Println("Entering main loop")
  s.Loop()
}


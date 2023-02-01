// package cmd implements process interface for python and other language processes to interact with peerprint
package cmd

import (
  "fmt"
  "log"
  "sync"
  "gopkg.in/zeromq/goczmq.v4"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
  // Import custom protos to add them to the global type registry, so anypb can resolve them when unmarshalling
	_ "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/smartin015/peerprint/p2pgit/pkg/storage"
)

const (
	MAX_RECV_RETRY = 10
	MAX_SEND_RETRY = 3
)

type Zmq struct {
  running bool
  wg sync.WaitGroup
  recvChan chan<- proto.Message
  errChan chan<- error
  sendChan chan proto.Message
  pushChan chan proto.Message
}

type Destructor func()

func New(rep_addr string, push_addr string, recvChan chan<- proto.Message, errChan chan<- error) (chan<- proto.Message, chan<- proto.Message, Destructor) {
  // print("CZMQ version is", goczmq.CZMQVersionMajor, ".", goczmq.CZMQVersionMinor, " (zeromq ", goczmq.ZMQVersionMajor, ".", goczmq.ZMQVersionMinor, ")\n")

  z := &Zmq {
    running: true,
    sendChan: make(chan proto.Message), // req/rep, no buffering
    pushChan: make(chan proto.Message, 5),
    recvChan: recvChan,
    errChan: errChan,
 }
 z.wg.Add(2)
 go z.syncpipe(z.sendChan, rep_addr)
 go z.pipe(z.pushChan, push_addr)
 return z.sendChan, z.pushChan, z.Destroy
}

// Returns a logger and its destructor
func NewLog(addr string) (*log.Logger, Destructor) {
  s, err := goczmq.NewPush(addr)
  if err != nil {
    panic(fmt.Errorf("create log pusher: %w", err))
  }
  rw, err := goczmq.NewReadWriter(s)
  if err != nil {
    panic(fmt.Errorf("create readwriter: %w", err))
  }
  return log.New(rw, "", 0), rw.Destroy
}

func (z *Zmq) Destroy() {
  z.running = false
  close(z.pushChan) // Closing pushChan breaks out of the pipe() loop for the PUSH socket
  z.wg.Wait()
}

func (z *Zmq) syncpipe(send <-chan proto.Message, rep_addr string) {
  defer storage.HandlePanic()
  defer z.wg.Done()
	var err error
  c, err := goczmq.NewRep(rep_addr) // will Bind() by default
  if err != nil {
    panic(fmt.Errorf("Create REP socket: %w", err))
  }
  p, err := goczmq.NewPoller(c)
  if err != nil {
    panic(fmt.Errorf("Create poller: %w", err))
  }

  for z.running {
		var mm [][]byte

    var pollin *goczmq.Sock
    for {
      pollin = p.Wait(100)
      if !z.running {
        return
      }
      if pollin == c {
        break
      }
    }

		// Experimentally observed zero-frame messages with ErrRecvFrame ("recv frame error") reported periodically when reading.
		// This happens without anything being sent from the wrapper. Retrying
		// the RecvMessage() call appears to fix this the vast majority of the time
		// although it does still sometimes happen permanently, requiring a 
		// close/reopen of the socket.
		retried := 0
		for ; retried < MAX_RECV_RETRY && (err == goczmq.ErrRecvFrame || len(mm) == 0); retried++ {
			mm, err = c.RecvMessage()
		}

    nbyte := 0
    for _, m := range mm {
      nbyte += len(m)
    }
    //print("   -> GO LEN ", nbyte, " NFRAME ", len(mm), "\n")

    if err != nil {
      z.errChan <- fmt.Errorf("receiver error (%d retries): %w", retried, err)
      if err == goczmq.ErrRecvFrame {
        z.errChan <- fmt.Errorf("closing and re-opening REP socket")
        // Recreate the socket
        p.Remove(c)
        c.SetLinger(0)
        c.Destroy()
        c, err = goczmq.NewRep(rep_addr)
        p.Add(c)
        if err != nil {
          panic(fmt.Errorf("Recreate REP socket: %w", err))
        }
      }
      continue
    }

    req, err := Deserialize(mm)
    var rep []byte

    if err != nil {
      z.errChan <- fmt.Errorf("deserialize error: %w", err)
      rep = []byte("")
    } else {
      z.recvChan <- req
      nn, more := <-send
      if !more {
        z.errChan <- fmt.Errorf("send channel closed")
        c.Destroy()
        return
      }
      rep, err = Serialize(nn)
      if err != nil {
        z.errChan <- fmt.Errorf("serialize error: %w", err)
        rep = []byte("") // balance req/rep
      }
    }
    //print("GO ->    LEN ", len(rep), " NFRAME 1\n")
    retried = 0
    for ; retried < MAX_SEND_RETRY; retried++ {
      err := c.SendMessage([][]byte{rep})
      if err == nil {
        break
      }
      z.errChan <- fmt.Errorf("REP error (%d retries): %w", retried, err)
    }
  }
}

func (z *Zmq) pipe(in <-chan proto.Message, push_addr string) {
  defer storage.HandlePanic()
  defer z.wg.Done()
  c, err := goczmq.NewPush(push_addr) // will Bind() by default
  if err != nil {
    panic(fmt.Errorf("Create PUSH socket: %w", err))
  }
  defer c.Destroy()
  for z.running {
    req, more := <-in
    if !more {
      return
    }
    data, err := Serialize(req)
    if err != nil {
      z.errChan <- err
    }

    retried := 0
    for ; retried < MAX_SEND_RETRY; retried++ {
      err := c.SendMessage([][]byte{data})
      if err == nil {
        break
      }
      z.errChan <- fmt.Errorf("Push error (%d retries): %w", retried, err)
    }
    if retried >= MAX_SEND_RETRY {
      z.errChan <- fmt.Errorf("closing and re-opening PUSH socket")
      // Recreate the socket
      c.SetLinger(0)
      c.Destroy()
      c, err = goczmq.NewPush(push_addr)
      if err != nil {
        panic(fmt.Errorf("Recreate PUSH socket: %w", err))
      }
    }
  }
}

func Deserialize(mm [][]byte) (proto.Message, error) {
  if len(mm) == 0 {
    return nil, fmt.Errorf("cannot deserialize empty msg")
  }

  any := anypb.Any{}
  if err := proto.Unmarshal(mm[0], &any); err != nil {
    return nil, fmt.Errorf("Failed to unmarshal ZMQ message to Any(): %w", err)
  }
  msg, err := any.UnmarshalNew()
  if err != nil {
    return nil, fmt.Errorf("Failed to unmarshal ZMQ message from Any() to final type: %w", err)
  }
  return msg, nil
}

func Serialize(req proto.Message) ([]byte, error) {
	any, err := anypb.New(req)
	if err != nil {
		return nil, fmt.Errorf("any-cast failed")
	}
	msg, err := proto.Marshal(any)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}
  return msg, nil
}

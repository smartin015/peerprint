// package cmd implements process interface for python and other language processes to interact with peerprint
package cmd

import (
  "fmt"
  "log"
  "gopkg.in/zeromq/goczmq.v4"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
  // Import custom protos to add them to the global type registry, so anypb can resolve them when unmarshalling
	_ "github.com/smartin015/peerprint/p2pgit/pkg/proto"
	"github.com/smartin015/peerprint/p2pgit/pkg/storage"
)

const (
	MAX_RECV_RETRY = 10
)

type Zmq struct {
  p *goczmq.Channeler
  recvChan chan<- proto.Message
  errChan chan<- error
  sendChan chan proto.Message
  pushChan chan proto.Message
}

type Destructor func()

func New(rep_addr string, push_addr string, recvChan chan<- proto.Message, errChan chan<- error) (chan<- proto.Message, chan<- proto.Message) {
  // print("CZMQ version is", goczmq.CZMQVersionMajor, ".", goczmq.CZMQVersionMinor, " (zeromq ", goczmq.ZMQVersionMajor, ".", goczmq.ZMQVersionMinor, ")\n")

  z := &Zmq {
    p: goczmq.NewPushChanneler(push_addr), // will Bind() by default
    sendChan: make(chan proto.Message), // req/rep, no buffering
    pushChan: make(chan proto.Message, 5),
    recvChan: recvChan,
    errChan: errChan,
 }
 go z.syncpipe(z.sendChan, rep_addr)
 go z.pipe(z.pushChan, z.p)
 return z.sendChan, z.pushChan
}

// Returns a logger and its destructor
func NewLog(addr string) (*log.Logger, Destructor) {
  s, err := goczmq.NewPush(addr)
  if err != nil {
    panic(err)
  }
  rw, err := goczmq.NewReadWriter(s)
  if err != nil {
    panic(err)
  }
  return log.New(rw, "", 0), rw.Destroy
}

func (z *Zmq) Destroy() {
  z.p.Destroy()
}

func (z *Zmq) syncpipe(send <-chan proto.Message, rep_addr string) {
  defer storage.HandlePanic()
	var err error
  c, err := goczmq.NewRep(rep_addr) // will Bind() by default
  if err != nil {
    panic(err)
  }
  for {
		var mm [][]byte
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
        c.SetLinger(0)
        c.Destroy()
        c, err = goczmq.NewRep(rep_addr)
        if err != nil {
          panic(err)
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
    c.SendMessage([][]byte{rep})
  }
}

func (z *Zmq) pipe(in <-chan proto.Message, c *goczmq.Channeler) {
  defer storage.HandlePanic()
  defer c.Destroy()
  for {
    req, more := <-in
    if !more {
      return
    }
    data, err := Serialize(req)
    if err != nil {
      z.errChan <- err
    }
    c.SendChan<- [][]byte{data}
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

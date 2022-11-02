// package cmd implements process interface for python and other language processes to interact with peerprint
package cmd

import (
  "fmt"
  "log"
  "gopkg.in/zeromq/goczmq.v4"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
  // Import custom protos to add them to the global type registry, so anypb can resolve them when unmarshalling
  _ "github.com/smartin015/peerprint/peerprint_server/proto" 
)

type Zmq struct {
  c *goczmq.Channeler
  p *goczmq.Channeler
  recvChan chan<- proto.Message
  errChan chan<- error
  sendChan chan proto.Message
  pushChan chan proto.Message
}

type Destructor func()

func New(rep_addr string, push_addr string, recvChan chan<- proto.Message, errChan chan<- error) (chan<- proto.Message, chan<- proto.Message) {
  z := &Zmq {
    c: goczmq.NewRepChanneler(rep_addr), // will Bind() by default
    p: goczmq.NewPushChanneler(push_addr), // will Connect() by default
    sendChan: make(chan proto.Message, 5),
    pushChan: make(chan proto.Message, 5),
    recvChan: recvChan,
    errChan: errChan,
 }
 go z.receiver()
 go z.pipe(z.sendChan, z.c.SendChan)
 go z.pipe(z.pushChan, z.p.SendChan)
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
  z.c.Destroy()
  z.p.Destroy()
}

func (z *Zmq) receiver() {
  for {
    select {
      case mm, more := <-z.c.RecvChan:
        if !more {
          return // Channel closed; no more messages
        }
        msg, err := Deserialize(mm)
        if err != nil {
          z.errChan <- fmt.Errorf("receiver error: %w", err)
        } else {
          z.recvChan <- msg
        }
    }
  }
}

func (z *Zmq) pipe(in <-chan proto.Message, out chan<- [][]byte) {
  for {
    req, more := <-in
    if !more {
      return
    }
    data, err := Serialize(req)
    if err != nil {
      z.errChan <- err
    }
    out<- data
  }
}

func Deserialize(mm [][]byte) (proto.Message, error) {
  for _, m := range(mm) {
    any := anypb.Any{}
    if err := proto.Unmarshal(m, &any); err != nil {
      return nil, fmt.Errorf("Failed to unmarshal ZMQ message to Any(): %w", err)
    }
    msg, err := any.UnmarshalNew()
    if err != nil {
      return nil, fmt.Errorf("Failed to unmarshal ZMQ message from Any() to final type: %w", err)
    }
    return msg, nil
  }
  return nil, fmt.Errorf("Deserialize failure for msg: %+v", mm)
}

func Serialize(req proto.Message) ([][]byte, error) {
	any, err := anypb.New(req)
	if err != nil {
		return nil, fmt.Errorf("any-cast failed")
	}
	msg, err := proto.Marshal(any)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}
  return [][]byte{msg}, nil
}

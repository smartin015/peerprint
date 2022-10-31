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
}

type Destructor func()

func New(rep_addr string, push_addr string) *Zmq {
  return &Zmq {
    c: goczmq.NewRepChanneler(rep_addr), // will Bind() by default
    p: goczmq.NewPushChanneler(push_addr), // will Connect() by default
  }
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

func (z *Zmq) Loop(cb func(proto.Message)) {
  for {
    select {
      case mm, more := <-z.c.RecvChan:
        if !more {
          return // Channel closed; no more messages
        }
        msg, err := z.Deserialize(mm)
        if err != nil {
          log.Println("cmd Loop() error:", err)
          panic("TEST")
        } else {
          cb(msg)
        }
    }
  }
}

func (z *Zmq) Deserialize(mm [][]byte) (proto.Message, error) {
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

func (z *Zmq) Serialize(req proto.Message) ([][]byte, error) {
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

func (z *Zmq) Push(req proto.Message) error {
  data, err := z.Serialize(req)
  if err != nil {
    return err
  }
  z.p.SendChan <- data
  return nil
}

func (z *Zmq) Send(req proto.Message) error {
  data, err := z.Serialize(req)
  if err != nil {
    return err
  }
	z.c.SendChan <- data
  return nil
}

// package cmd implements process interface for python and other language processes to interact with peerprint
package cmd

import (
  "fmt"
  "gopkg.in/zeromq/goczmq.v4"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Zmq struct {
  c *goczmq.Channeler
}

func New(addr string) *Zmq {
  return &Zmq {
    c: goczmq.NewRepChanneler(addr), // will Bind() by default
  }
}

func (z *Zmq) Loop(cb func(interface{}, string)) {
  for {
    select {
      case mm := <-z.c.RecvChan:
        for _, m := range(mm) {
          any := anypb.Any{}
          if err := proto.Unmarshal(m, &any); err != nil {
            panic(fmt.Errorf("Failed to unmarshal ZMQ message to Any(): %w", err))
          }
          msg, err := any.UnmarshalNew()
          if err != nil {
            panic(fmt.Errorf("Failed to unmarshal ZMQ message from Any() to final type: %w", err))
          }
          cb(msg, string(any.MessageName()))
      }
    }
  }
}

func (z *Zmq) Send(req proto.Message) error {
	any, err := anypb.New(req)
	if err != nil {
		return fmt.Errorf("any-cast failed")
	}
	msg, err := proto.Marshal(any)
	if err != nil {
		return fmt.Errorf("marshal error:", err)
	}
	z.c.SendChan <- [][]byte{msg}
  return nil
}

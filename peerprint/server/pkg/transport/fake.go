package transport

import (
  "context"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/smartin015/peerprint/p2pgit/pkg/topic"
	"google.golang.org/protobuf/proto"
)

type fakeTransport struct {
  id string
}
func Fake(id string) *fakeTransport {
  return &fakeTransport{id: id}
}

func (t *fakeTransport)  Register(protocol string, srv interface{}) error {
  return nil
}
func (t *fakeTransport)  Run(context.Context) {}
func (t *fakeTransport)  ID() string {
  return t.id
}
func (t *fakeTransport)  Sign([]byte) ([]byte, error) {
  return nil, nil
}
func (t *fakeTransport)  PubKey() crypto.PubKey {
  return nil
}
func (t *fakeTransport)  Publish(topic string, msg proto.Message) error {
  return nil
}
func (t *fakeTransport)  OnMessage() <-chan topic.TopicMsg {
  return nil
}
func (t *fakeTransport)  GetRandomNeighbor() (peer.ID, error) {
  return "", nil
}
func (t *fakeTransport)  Call(pid peer.ID, method string, req proto.Message, rep proto.Message) error {
  return nil
}

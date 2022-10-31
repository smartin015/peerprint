package prpc

import (
	"context"
	"fmt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type TopicMsg struct {
  Topic string
  Peer string
  Msg proto.Message
}

type TopicReceiver struct {
  id string
  Topic *pubsub.Topic
  s *pubsub.Subscription
  closed bool
  recvChan chan TopicMsg
  errChan chan error
}

func NewTopicReceiver(ctx context.Context, recvChan chan TopicMsg, selfID string, p *pubsub.PubSub, topic string, errChan chan error) (*TopicReceiver, error) {
	t, err := p.Join(topic)
	if err != nil {
		return nil, fmt.Errorf("pubsub.Join() failure: %w", err)
	}
	s, err := t.Subscribe()
	if err != nil {
    t.Close()
		return nil, fmt.Errorf("pubsub Subscribe() failure: %w", err)
	}

  tc := &TopicReceiver {
    id: selfID, 
    Topic: t,
    s: s,
    closed: false,
    recvChan: recvChan,
    errChan: errChan,
  }
  go tc.Run(ctx)
  return tc, nil
}

func (t *TopicReceiver) Close() error {
  t.closed = true
  t.s.Cancel()
  // Note: do not close(t.recv) as it is owned by our creator
  return t.Topic.Close()
}

func (t *TopicReceiver) Run(ctx context.Context) {
	for {
		m, err := t.s.Next(ctx)
		if err != nil {
      if t.closed {
        return
      }
			t.errChan <- err
      continue
		}
		peer := m.ReceivedFrom.String()
		if peer == t.id {
			continue // Ignore messages coming from ourselves
		}
		any := anypb.Any{}
		if err := proto.Unmarshal(m.Message.Data, &any); err != nil {
			t.errChan <- err
      continue
		}
		msg, err := any.UnmarshalNew()
		if err != nil {
		  t.errChan <- err
      continue
		}
    t.recvChan <- TopicMsg{Topic: t.s.Topic(), Peer: peer, Msg: msg}
	}
}

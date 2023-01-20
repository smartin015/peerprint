package topic_receiver

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

type TopicChannel struct {
  id string
  topic *pubsub.Topic
  s *pubsub.Subscription
  closed bool
  sendChan chan proto.Message
  recvChan chan<- TopicMsg
  errChan chan<- error
}

func NewTopicChannel(ctx context.Context, recvChan chan<- TopicMsg, selfID string, p *pubsub.PubSub, topic string, errChan chan<- error) (chan<- proto.Message, error) {
	t, err := p.Join(topic)
	if err != nil {
		return nil, fmt.Errorf("pubsub.Join() failure: %w", err)
	}
	s, err := t.Subscribe()
	if err != nil {
    t.Close()
		return nil, fmt.Errorf("pubsub Subscribe() failure: %w", err)
	}

  sc := make(chan proto.Message, 5)
  tc := &TopicChannel {
    id: selfID, 
    topic: t,
    s: s,
    closed: false,
    sendChan: sc,
    recvChan: recvChan,
    errChan: errChan,
  }

  if err != nil {
    return nil, err
  }
  go tc.sendLoop(ctx)
  go tc.recvLoop(ctx)
  return sc, nil
}

func (t *TopicChannel) destroy() error {
  t.closed = true
  t.s.Cancel()
  // Note: do not close any owner-provided channels
  close(t.sendChan)
  return t.topic.Close()
}

func (t *TopicChannel) sendLoop(ctx context.Context) {
  for {
    req, more := <-t.sendChan
    if !more {
      if err := t.destroy(); err != nil {
        t.errChan <- err
      }
      return
    }
    any, err := anypb.New(req)
    if err != nil {
      t.errChan <- fmt.Errorf("sendLoop() any-cast failed")
      continue
    }
    msg, err := proto.Marshal(any)
    if err != nil {
      t.errChan <- fmt.Errorf("sendLoop() marshal error: %w", err)
      continue
    }
    if err = t.topic.Publish(ctx, msg); err != nil {
      t.errChan <- fmt.Errorf("sendLoop() publish error: %w", err)
      continue
    }
  }
}

func (t *TopicChannel) recvLoop(ctx context.Context) {
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

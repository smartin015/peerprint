// package prpc sends and receives proto messages over pubsub
package prpc

import (
	"context"
	"fmt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Callback func(string, string, proto.Message) (proto.Message, error)

type PRPC struct {
	ID     string
	ps     *pubsub.PubSub
	receivers map[string]*TopicReceiver
	cb    Callback
  subChan chan TopicMsg
  ErrChan chan error
}

func New(ctx context.Context, id string, ps *pubsub.PubSub) *PRPC {
  p := &PRPC{
		ID:     id,
		ps:     ps,
		receivers: make(map[string]*TopicReceiver),
		cb:    nil,
    subChan: make(chan TopicMsg, 10),
    ErrChan: make(chan error, 10),
	}
  go p.Loop(ctx)
  return p
}

func (p *PRPC) Loop(ctx context.Context) {
  for {
    select {
    case <- ctx.Done():
      return
    case msg, more := <-p.subChan:
      if !more {
        return
      }
      rep, err := p.cb(msg.Topic, msg.Peer, msg.Msg)
      if err != nil {
        p.ErrChan <- fmt.Errorf("callback error: %w", err)
        continue
      }
      if rep != nil {
        if err := p.Publish(ctx, msg.Topic, rep.(proto.Message)); err != nil {
          p.ErrChan <- fmt.Errorf("Response error: %w", err)
        }
      }
    }
  }
}

func (p *PRPC) RegisterCallback(c Callback) {
	p.cb = c
}

func (p *PRPC) PeerCount(topic string) int {
  return len(p.ps.ListPeers(topic))
}

func (p *PRPC) JoinTopic(ctx context.Context, topic string) error {
	if _, ok := p.receivers[topic]; ok {
		return fmt.Errorf("Already subscribed to topic %v", topic)
	}
  r, err := NewTopicReceiver(ctx, p.subChan, p.ID, p.ps, topic, p.ErrChan)
  if err != nil {
    return err
  }
  p.receivers[topic] = r
	return nil
}

func (p *PRPC) LeaveTopic(topic string) error {
  if err := p.receivers[topic].Close(); err != nil {
    return err
  }
  delete(p.receivers, topic)
  return nil
}

func (p *PRPC) Close() error {
  for topic, _ := range p.receivers {
    if err := p.LeaveTopic(topic); err != nil {
      return err
    }
  }
  return nil
}

func (p *PRPC) Publish(ctx context.Context, topic string, req proto.Message) error {
	any, err := anypb.New(req)
	if err != nil {
		return fmt.Errorf("prpc.Publish() any-cast failed")
	}
	msg, err := proto.Marshal(any)
	if err != nil {
		return fmt.Errorf("prpc.Publish() marshal error: %w", err)
	}

	r, ok := p.receivers[topic]
	if !ok {
		return fmt.Errorf("attempted to publish to topic %s without first calling JoinTopic()", topic)
	}

	if err = r.Topic.Publish(ctx, msg); err != nil {
		return fmt.Errorf("prpc.Publish() publish error: %w", err)
	}
	return nil
}


// package prpc sends and receives proto messages over pubsub
package prpc

import (
	"context"
	"fmt"
  "log"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type Callback func(string, string, proto.Message) (proto.Message, error)

type PRPC struct {
	ID     string
	ps     *pubsub.PubSub
	topics map[string]*pubsub.Topic
	cb    Callback
}

func New(id string, ps *pubsub.PubSub) *PRPC {
	return &PRPC{
		ID:     id,
		ps:     ps,
		topics: make(map[string]*pubsub.Topic),
		cb:    nil,
	}
}

func (p *PRPC) RegisterCallback(c Callback) {
	p.cb = c
}

func (p *PRPC) handleSub(ctx context.Context, sub *pubsub.Subscription, l *log.Logger) {
	for {
		m, err := sub.Next(ctx)
		if err != nil {
			panic(err)
		}
		peer := m.ReceivedFrom.String()
		if peer == p.ID {
			continue // Ignore messages coming from ourselves
		}
		any := anypb.Any{}
		if err := proto.Unmarshal(m.Message.Data, &any); err != nil {
			panic(err)
		}
		msg, err := any.UnmarshalNew()
		if err != nil {
			panic(err)
		}
    rep, err := p.cb(sub.Topic(), peer, msg)
    if rep != nil && err == nil {
      l.Println("Sending response")
      err = p.Publish(ctx, sub.Topic(), rep.(proto.Message))
    }
    if err != nil {
      l.Println(fmt.Errorf("handleSub callback / publish error: %w", err).Error())
    }
	}
}

func (p *PRPC) JoinTopic(ctx context.Context, topic string, l *log.Logger) error {
	if _, ok := p.topics[topic]; ok {
		return fmt.Errorf("Already subscribed to topic %v", topic)
	}
	t, err := p.ps.Join(topic)
	if err != nil {
		return fmt.Errorf("pubsub.Join() failure: %w", err)
	}
	sub, err := t.Subscribe()
	if err != nil {
		return fmt.Errorf("pubsub Subscribe() failure: %w", err)
	}
	p.topics[topic] = t
	go p.handleSub(ctx, sub, l)
	return nil
}

func (p *PRPC) LeaveTopic(topic string) error {
	return fmt.Errorf("Unimplemented")
}

func (p *PRPC) Close() error { return nil }

func (p *PRPC) Publish(ctx context.Context, topic string, req proto.Message) error {
	any, err := anypb.New(req)
	if err != nil {
		return fmt.Errorf("prpc.Publish() any-cast failed")
	}
	msg, err := proto.Marshal(any)
	if err != nil {
		return fmt.Errorf("prpc.Publish() marshal error:", err)
	}

	t, ok := p.topics[topic]
	if !ok {
		return fmt.Errorf("attempted to publish to topic %s without first calling JoinTopic()", topic)
	}

	if err = t.Publish(ctx, msg); err != nil {
		return fmt.Errorf("prpc.Publish() publish error:", err)
	}
	return nil
}


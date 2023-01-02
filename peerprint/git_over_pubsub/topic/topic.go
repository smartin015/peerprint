package topic

import (
	"context"
	"fmt"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"google.golang.org/protobuf/proto"
  pb "github.com/smartin015/peerprint/p2pgit/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type TopicMsg struct {
  Topic string
  Peer string
  Msg proto.Message
  PubKey crypto.PubKey
  Signature *pb.Signature
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

func pubkeyFromMsg(m *pubsub.Message) (crypto.PubKey, error) {
  // See https://github.com/libp2p/go-libp2p-pubsub/blob/4f56e8f0a75b06aff68ae79717915ebadc1698f6/sign.go#L77
  var pubk crypto.PubKey
  pid, err := peer.IDFromBytes(m.From)
  if err != nil {
    return nil, err
  }

	if m.Key == nil {
		// no attached key, it must be extractable from the source ID
		pubk, err = pid.ExtractPublicKey()
		if err != nil {
			return nil, fmt.Errorf("cannot extract signing key: %s", err.Error())
		}
		if pubk == nil {
			return nil, fmt.Errorf("cannot extract signing key")
		}
	} else {
		pubk, err = crypto.UnmarshalPublicKey(m.Key)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal signing key: %s", err.Error())
		}

		// verify that the source ID matches the attached key
		if !pid.MatchesPublicKey(pubk) {
			return nil, fmt.Errorf("bad signing key; source ID %s doesn't match key", pid)
		}
	}
  return pubk, nil
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
      t.errChan <- fmt.Errorf("recvLoop unmarshal to Any: %w", err)
      continue
		}
		msg, err := any.UnmarshalNew()
		if err != nil {
      t.errChan <- fmt.Errorf("recvLoop unpack Any: %w", err)
      continue
		}

		pub, err := pubkeyFromMsg(m)
		if err != nil {
      t.errChan <- fmt.Errorf("recLoop key unmarshal from %v: %w", m.GetKey(), err)
      continue
		}
    t.recvChan <- TopicMsg{
      Topic: t.s.Topic(),
      Peer: peer,
      Signature: &pb.Signature{
        Signer: peer,
        Data: m.GetSignature(),
      },
      Msg: msg,
      PubKey: pub,
    }
	}
}

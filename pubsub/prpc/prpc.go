// package prpc sends and receives proto messages over pubsub
package prpc 

import (
  "fmt"
  "context"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
  pb "github.com/smartin015/peerprint/pubsub/proto"
  "google.golang.org/protobuf/proto"
)

type Callbacks interface {
  OnPollPeersRequest(string, *pb.PollPeersRequest)
  OnPollPeersResponse(string, *pb.PollPeersResponse)
  OnAssignmentRequest(string, *pb.AssignmentRequest)
  OnAssignmentResponse(string, *pb.AssignmentResponse)

  OnSetJobRequest(string, *pb.SetJobRequest)
  OnDeleteJobRequest(string, *pb.DeleteJobRequest)
  OnAcquireJobRequest(string, *pb.AcquireJobRequest)
  OnReleaseJobRequest(string, *pb.ReleaseJobRequest)
  OnJobMutationResponse(string, *pb.JobMutationResponse)
  OnGetJobsRequest(string, *pb.GetJobsRequest)
  OnGetJobsResponse(string, *pb.GetJobsResponse)
}

type PRPC struct {
  ID string
  ps *pubsub.PubSub
  topics map[string]*pubsub.Topic
  cbs Callbacks
  topicLeaders map[string]string
  polling bool
}

func New(id string, ps *pubsub.PubSub) *PRPC {
  return &PRPC{
    ID: id,
    ps: ps,
    topics: make(map[string]*pubsub.Topic),
    topicLeaders: make(map[string]string),
    cbs: nil,
    polling: false,
  }
}

func (p *PRPC) SetLeader(topic string, id string) {
  p.topicLeaders[topic] = id
}
func (p *PRPC) SetPolling(polling bool) {
  p.polling = polling
}

func (p *PRPC) RegisterCallbacks(c Callbacks) {
  p.cbs = c
}

func (p *PRPC) HandleSub(ctx context.Context, sub *pubsub.Subscription) {
	for {
		m, err := sub.Next(ctx)
		if err != nil {
			panic(err)
		}
    peer := m.ReceivedFrom.String()
    if peer == p.ID {
      continue // Ignore messages coming from ourselves
    }
    var msg proto.Message
    if err := proto.Unmarshal(m.Message.Data, msg); err != nil {
      panic(err)
    }
    p.Recv(ctx, sub.Topic(), peer, msg)
	}
}

func (p *PRPC) JoinTopic(ctx context.Context, topic string, requiredPeer string) error {
  if _, ok := p.topics[topic]; ok {
    return fmt.Errorf("Already subscribed to topic %v", topic)
  }
  t, err := p.ps.Join(topic)
  if err != nil {
    return fmt.Errorf("pubsub.Join() failure: %w", err)
  }
  sub, err := t.Subscribe()
  if err != nil {
    panic(err)
  }
  p.topics[topic] = t
  go p.HandleSub(ctx, sub)
  return nil
}

func (p *PRPC) LeaveTopic(topic string) error {
  return fmt.Errorf("Unimplemented")
}

func (p *PRPC) Close() error {return nil}

func (p *PRPC) Publish(ctx context.Context, topic string, req interface{}) error {
  reqp, ok := req.(proto.Message)
  if !ok {
    return fmt.Errorf("prpc.Publish() type assertion failed")
  }
  msg, err := proto.Marshal(reqp)
  if err != nil {
    return fmt.Errorf("prpc.Publish() marshal error:", err)
  }
  if err = p.topics[topic].Publish(ctx, msg, ); err != nil {
    return fmt.Errorf("prpc.Publish() publish error:", err)
  }
  return nil
}

func (p *PRPC) Recv(ctx context.Context, topic string, peer string, resp interface{}) error {
  leader := p.topicLeaders[topic]
  switch v := resp.(type) {
    // proto/peers.proto
    case *pb.AssignmentRequest:
      if leader == p.ID {
        p.cbs.OnAssignmentRequest(topic, v)
      }
    case *pb.AssignmentResponse:
      if peer == leader && v.Id == p.ID {
        p.cbs.OnAssignmentResponse(topic, v)
      }
    case *pb.PollPeersRequest:
      p.cbs.OnPollPeersRequest(topic, v)
    case *pb.PollPeersResponse:
      if p.polling {
        p.cbs.OnPollPeersResponse(topic, v)
      }

    // proto/jobs.proto
    case *pb.SetJobRequest:
      if leader == p.ID {
        p.cbs.OnSetJobRequest(topic, v)
      }
    case *pb.DeleteJobRequest:
      if leader == p.ID {
        p.cbs.OnDeleteJobRequest(topic, v)
      }
    case *pb.AcquireJobRequest:
      if leader == p.ID {
        p.cbs.OnAcquireJobRequest(topic, v)
      }
    case *pb.ReleaseJobRequest:
      if leader == p.ID {
        p.cbs.OnReleaseJobRequest(topic, v)
      }
    case *pb.JobMutationResponse:
      if leader == peer {
        p.cbs.OnJobMutationResponse(topic, v)
      }
    case *pb.GetJobsRequest:
      if leader == p.ID {
        p.cbs.OnGetJobsRequest(topic, v)
      }
    case *pb.GetJobsResponse:
      if leader == peer {
        p.cbs.OnGetJobsResponse(topic, v)
      }

    default:
      return fmt.Errorf("Received unknown type response on topic")
  }
  return nil
}

func (p *PRPC) Call(req *proto.Message) *proto.Message {
  // TODO reflect to get topic

  // TODO if call is leader election, perform RAFT consensus and return result
  return nil
}

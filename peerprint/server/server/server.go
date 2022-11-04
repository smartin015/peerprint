// package server implements handlers for peerprint service
package server

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/proto"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
	tr "github.com/smartin015/peerprint/peerprint_server/topic_receiver"
	"github.com/smartin015/peerprint/peerprint_server/raft"
	"github.com/smartin015/peerprint/peerprint_server/poll"
	"log"
	"time"
)

const (
	AssignmentTopic = "ASSIGN"
	DefaultTopic    = "0"
  LockTimeoutSeconds = 2*60*60
)

type Server struct {
	l                *log.Logger
	trustedPeers     map[string]struct{}
  roleAssigned     chan pb.PeerType

  recvPubsub      <-chan tr.TopicMsg
  recvCmd         <-chan proto.Message

  sendPubsub      map[string](chan<- proto.Message)
  sendCmd         chan<- proto.Message
  pushCmd         chan<- proto.Message

	raft             raft.Raft
  poller           poll.Poller
  status           pb.PeerStatus
  open             func(string) (chan proto.Message, error)
}

type ServerOptions struct {
  ID string
  TrustedPeers []string
  Logger *log.Logger
  Raft raft.Raft
  Poller poll.Poller

  RecvPubsub      <-chan tr.TopicMsg
  RecvCmd         <-chan proto.Message

  SendCmd         chan<- proto.Message
  PushCmd         chan<- proto.Message

  Opener func(string)(chan proto.Message, error)
}

func New(opts ServerOptions) *Server {
	tp := make(map[string]struct{})
	for _, p := range opts.TrustedPeers {
		tp[p] = struct{}{}
	}
  s := &Server{
    l:                opts.Logger,
    trustedPeers:     tp,
    roleAssigned:     make(chan pb.PeerType),

    recvPubsub:       opts.RecvPubsub,
    recvCmd:          opts.RecvCmd,

    sendPubsub:       make(map[string](chan<- proto.Message)),
    sendCmd:          opts.SendCmd,
    pushCmd:         opts.PushCmd,

    raft:            opts.Raft,
    poller:          opts.Poller,
    status:          pb.PeerStatus{
      Id: opts.ID,
    },
    open: opts.Opener,
	}
  s.setup()
  return s
}

func (t *Server) setup() {
  // Whether or not we're a trusted peer, we need to join the assignment topic.
	// Leader election is also broadcast here.
  atc, err := t.open(AssignmentTopic)
  if err != nil {
		panic(err)
	}
  t.sendPubsub[AssignmentTopic] = atc

	if t.amTrusted() {
		// t.l.Println("We are a trusted peer; overriding assignment")
		t.OnAssignmentResponse(AssignmentTopic, t.getID(), &pb.AssignmentResponse{
			Id:       t.getID(),
			Topic:    DefaultTopic,
			Type:     pb.PeerType_ELECTABLE,
			LeaderId: "",
		})
	}
}

func (t *Server) amTrusted() bool {
	_, ok := t.trustedPeers[t.getID()]
  return ok
}

func (t *Server) getID() string {
  return t.status.GetId()
}

func (t *Server) getTopic() string {
  return t.status.GetTopic()
}

func (t *Server) getType() pb.PeerType {
  return t.status.GetType()
}

func (t *Server) getLeader() string {
  return t.raft.Leader()
}

func (t *Server) getRaftPeers() []*pb.AddrInfo {
  return t.raft.GetPeers()
}


func (t *Server) Loop(ctx context.Context) {
	if !t.amTrusted() {
    t.status.Topic = AssignmentTopic
    t.l.Println("Beginning assignment request loop")
    for t.getTopic() == AssignmentTopic {
      select {
      case t.sendPubsub[AssignmentTopic] <- &pb.AssignmentRequest{}:
      default:
        panic("requestAssignment: channel overflow during request")
      }

      t.l.Println("Sent pubsub assignment request")
      tmr := time.NewTimer(10 * time.Second)
      select {
      case <-ctx.Done():
        return
      case <-t.roleAssigned:
        break
      case <-tmr.C:
        continue
      }
    }
  }
	for {
    select {
    case req := <-t.recvCmd:
      t.l.Println("recvCmd")
      var err error
      var rep proto.Message
      // Skip pubsub if we're the leader, as we are authoritative
      if t.getID() == t.getLeader() {
        rep, err = t.Handle(t.getTopic(), t.getID(), req)
        if err == nil && rep != nil {
          t.sendPubsub[t.getTopic()]<- rep
          t.pushCmd<- rep
        }
      } else {
        // Otherwise we publish the command as-is, return OK
        t.sendPubsub[t.getTopic()]<- req
      }

      if err != nil {
        t.sendCmd<- &pb.Error{Status: err.Error()}
      } else {
        t.sendCmd<- &pb.Ok{}
      }
    case msg := <-t.recvPubsub:
      rep, err := t.Handle(msg.Topic, msg.Peer, msg.Msg)
      if err != nil {
        t.l.Println(fmt.Errorf("callback error: %w", err))
        continue
      }
      if rep != nil {
        t.sendPubsub[msg.Topic] <- rep.(proto.Message)
      }
		case <-t.raft.LeaderChan():
			if t.getLeader() == t.getID() {
        t.sendPubsub[t.getTopic()]<-&pb.Leader{
          Id: t.getLeader(),
        }
        // Important to publish state after initial leader election,
        // so all listener nodes are also on the same page.
        // This may fail on new queue that hasn't been bootstrapped yet
        s, err := t.raft.Get()
        if err == nil {
          t.sendPubsub[t.getTopic()]<- s
          t.pushCmd<- s
        }
			}
      t.poller.Resume()
    case prob, more := <-t.poller.Epoch():
      if !more {
        return
      }
      t.l.Println(t.getTopic())
      t.sendPubsub[t.getTopic()]<- &pb.PollPeersRequest{
        Probability: prob,
      }
    case summary, more := <-t.poller.Result():
      if !more {
        return
      }
      t.sendPubsub[t.getTopic()]<- summary
      select {
      case t.pushCmd<- summary:
      default:
      }
		case <-ctx.Done():
      close(t.roleAssigned)
			return
		}
	}
}

func (t *Server) raftAddrInfo() *pb.AddrInfo {
  return  &pb.AddrInfo{
      Id:    t.raft.ID(),
		  Addrs: t.raft.Addrs(),
    }
}

func (t *Server) raftAddrsRequest() *pb.RaftAddrsRequest {
	return &pb.RaftAddrsRequest{
    AddrInfo:	t.raftAddrInfo(),
  }
}
func (t *Server) raftAddrsResponse() *pb.RaftAddrsResponse {
  t.l.Println("TODO get raft peers list")
	return &pb.RaftAddrsResponse{Peers: []*pb.AddrInfo{
    t.raftAddrInfo(),
	}}
}

func (t *Server) checkMutable(j *pb.Job, peer string) error {
	expiry := uint64(time.Now().Unix() - LockTimeoutSeconds)
  p := j.Lock.GetPeer()
	if !(p == "" || p == peer || j.Lock.Created < expiry) {
    return fmt.Errorf("access denied for %q; job %s acquired by %q", peer, j.GetId(), j.Lock.GetPeer())
  }
  return nil
}

func (t *Server) getMutableJob(jid string, peer string) (*pb.State, *pb.Job, error) {
	s, err := t.raft.Get()
	if err != nil {
		return nil, nil, fmt.Errorf("raft.Get(): %w", err)
	}
	j, ok := s.Jobs[jid]
  if !ok {
    return nil, nil, fmt.Errorf("job not found")
  }
  if err := t.checkMutable(j, peer); err != nil {
    return nil, nil, err
  }
  return s, j, nil
}

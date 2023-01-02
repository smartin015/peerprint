package server

import (
  "context"
  "log"
  "time"
  "fmt"
  "google.golang.org/protobuf/proto"
  pb "github.com/smartin015/peerprint/p2pgit/proto"
	"github.com/smartin015/peerprint/p2pgit/transport"
	"github.com/smartin015/peerprint/p2pgit/storage"
)

const (
  PeerPrintProtocol = "peerprint@0.0.1"
  DefaultTopic = "default"
  DefaultGrantTTL = 5 * time.Minute
  SyncPeriod = 5 * time.Second
  TargetAdminCount = 3
)

type sublog struct {
  n string
  l *log.Logger
}
func (l *sublog) log(prefix string, args []interface{}, suffix string) {
  l.l.Println(fmt.Sprintf("%v %s(%s): ", time.Now().Format(time.RFC3339), prefix, l.n) + fmt.Sprintf(args[0].(string), args[1:]...))
}
func (l *sublog) Info(args ...interface{}) {
  l.log("\033[0mI", args, "\033[0m")
}
func (l *sublog) Error(args ...interface{}) {
  l.log("\033[31mE", args, "\033[0m")
}



type Opts struct {
  AccessionDelay time.Duration
  StatusPeriod time.Duration
}

type Server struct {
  t transport.Interface
  s storage.Interface
  mirror string
  logger *log.Logger
  l *sublog

  // Tickers for periodic network activity
  publishStatusTicker *time.Ticker
  status *pb.PeerStatus

  // Data for specific states
  handshake *handshake
  listener *listener
  electable *electable
  leader *leader

}

func New(t transport.Interface, s storage.Interface, opts *Opts, l *log.Logger) *Server {
  srv := &Server{
    t: t,
    s: s,
    logger: l,
    l: &sublog{l: l, n: "Base"},
    publishStatusTicker: time.NewTicker(opts.StatusPeriod),
    status: &pb.PeerStatus{
      Type: pb.PeerType_UNKNOWN_PEER_TYPE, // Unknown until handshake is complete
    },
  }
  srv.handshake = &handshake{
    base: srv,
    accessionDelay: opts.AccessionDelay,
    l: &sublog{l: l, n: "Handshake"},
  }
  srv.listener = &listener{
    base: srv,
    l: &sublog{l: l, n: "Listener"},
  }
  srv.electable = &electable{
    base: srv,
    l: &sublog{l: l, n: "Electable"},
  }
  srv.leader = &leader{
    base: srv,
    ticker: time.NewTicker(Heartbeat),
    l: &sublog{l: l, n: "Leader"},
  }

  if err := s.SetPubKey(t.ID(), t.PubKey()); err != nil {
    panic(fmt.Errorf("Failed to store our pubkey: %w", err))
  }

  return srv
}

func (s *Server) GetService() *PeerPrintService {
  return &PeerPrintService{
    base: s,
  }
}

func (s *Server) Ready() bool {
  return s.status.Type != pb.PeerType_UNKNOWN_PEER_TYPE
}

func (s *Server) sendStatus() error {
  return s.t.Publish(StatusTopic, s.status)
}

func (s *Server) partialSync() {
  n, err := s.t.GetRandomNeighbor()
  if err != nil {
    s.l.Error("GetRandomNeighbor: %w", err)
    return
  }
  rep := &pb.GetStateResponse{}
  if err := s.t.Call(n, "GetState", &pb.GetStateRequest{}, rep); err != nil {
    s.l.Error("GetState: %w", err)
    return
  }
  cnt := 0
  nerr := 0
  for _, r := range rep.Records {
    if err := s.s.SetSignedRecord(r); err != nil {
      s.l.Error("SetSignedRecord: %w", err)
      nerr += 1
    } else {
      cnt += 1
    }
  }
  for _, g := range rep.Grants {
    if err := s.s.SetSignedGrant(g); err != nil {
      s.l.Error("SetSignedGrant: %w", err)
      nerr += 1
    } else {
      cnt += 1
    }
  }
}

func (s *Server) sign(m proto.Message) (*pb.Signature, error) {
  if b, err := proto.Marshal(m); err != nil {
    return nil, fmt.Errorf("sign() marshal error: %w", err)
  } else if sig, err := s.t.Sign(b); err != nil {
    return nil, fmt.Errorf("sign() crypto error: %w", err)
  } else {
    return &pb.Signature{
      Signer: s.t.ID(),
      Data: sig,
    }, nil
  }
}

func (s *Server) verify(m proto.Message, sig *pb.Signature) (bool, error) {
  k, err := s.s.GetPubKey(sig.Signer)
  if err != nil {
    return false, fmt.Errorf("verify() get key error: %w", err)
  }
  b, err := proto.Marshal(m)
  if err != nil {
    return false, fmt.Errorf("verify() marshal error: %w", err)
  }

  return k.Verify(b, sig.Data)
}

func (s *Server) storeGrant(peer string, g *pb.Grant, sig *pb.Signature) bool {
  if peerAdmin, err := s.s.IsAdmin(peer); err != nil {
    s.l.Error("IsAdmin error: %w", err)
    return false
  } else if !peerAdmin {
    s.l.Info("Ignoring grant from non-admin peer %s", peer)
    return false
  }
  if err := s.s.SetSignedGrant(&pb.SignedGrant{
    Grant: g,
    Signature: sig,
  }); err != nil {
    s.l.Error("SetSignedGrant error: %w", err)
    return false
  }
  s.l.Info("Stored grant (from %s)", peer)
  return true
}

func (s *Server) storeRecord(peer string, r *pb.Record, sig *pb.Signature) bool {
  if peerAdmin, err := s.s.IsAdmin(peer); err != nil {
    s.l.Error("IsAdmin error: %w", err)
    return false
  } else if !peerAdmin {
    s.l.Info("Ignoring grant from non-admin peer %s", peer)
    return false
  }
  if err := s.s.SetSignedRecord(&pb.SignedRecord{
    Record: r,
    Signature: sig,
  }); err != nil {
    s.l.Error("SetSignedRecord error: %w", err)
    return false
  }
  s.l.Info("Stored record (from %s)", peer)
  return true
}


func (s *Server) Run(ctx context.Context) {
  s.t.Run(ctx)
  s.l.Info("Running server main loop")

  s.handshake.Init()
  for {
    switch s.status.Type {
    case pb.PeerType_UNKNOWN_PEER_TYPE:
      s.handshake.Step(ctx)
    case pb.PeerType_LEADER:
      s.leader.Step(ctx)
    case pb.PeerType_ELECTABLE:
      s.electable.Step(ctx)
    case pb.PeerType_LISTENER:
      s.listener.Step(ctx)
    default:
      panic("Unknown status type")
    }
  }
}

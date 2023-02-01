package driver

import (
  "google.golang.org/protobuf/proto"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/crawl"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/cmd"
	"github.com/libp2p/go-libp2p/core/peer"
  "context"
  "fmt"
  "time"
  "errors"
  "strings"
)

const (
  MaxAddrPerPeer = 30
)

var (
  ErrWatchdog = errors.New("Watchdog timeout")
  ErrContext = errors.New("Context canceled")
)

type Driver struct {
  s server.Interface
  st storage.Interface
  t transport.Interface
  c *crawl.Crawler
  l *pplog.Sublog
}

func NewDriver(srv server.Interface, st storage.Interface, t transport.Interface, l *pplog.Sublog) *Driver {
  return &Driver{
    s: srv,
    st: st,
    t: t,
    c: nil,
    l: l,
  }
}

func (d *Driver) Loop(ctx context.Context, repPath, pushPath string, watchdog time.Duration) error {
  cmdRecv := make(chan proto.Message, 5)
  errChan := make(chan error, 5)
  cmdSend, cmdPush, destroyComm := cmd.New(repPath, pushPath, cmdRecv, errChan)
  defer destroyComm()
  wdt := time.NewTimer(watchdog)
  if watchdog== 0 {
    wdt.Stop()
  }
  for {
    select {
    case m := <-d.s.OnUpdate():
      cmdPush<- m
    case e := <-errChan:
      d.l.Error(e.Error())
    case c := <-cmdRecv:
      rep, err := d.handleCommand(ctx, c)
      if err != nil {
        cmdSend<- &pb.Error{Reason: err.Error()}
      } else {
        cmdSend<- rep
      }
    case <-wdt.C:
      d.l.Error("Watchdog timeout, exiting")
      return ErrWatchdog
    case <- ctx.Done():
      d.l.Info("Context completed, exiting")
      return ErrContext
    }
    if watchdog > 0 {
      wdt.Reset(watchdog)
    }
  }
}

func (d *Driver) handleCommand(ctx context.Context, c proto.Message) (proto.Message, error) {
  switch v := c.(type) {
  case *pb.HealthCheck:
    return &pb.HealthCheck{}, nil
  case *pb.GetID:
    return &pb.IDResponse{
      Id: d.s.ID(),
    }, nil
  case *pb.Record:
    return d.s.IssueRecord(v, true)
  case *pb.Completion:
    return d.s.IssueCompletion(v, true)
  case *pb.CrawlPeers:
    if v.RestartCrawl || d.c == nil {
      d.c = crawl.NewCrawler(d.t.GetPeerAddresses(), d.crawlPeer)
    }
    ctx2, _ := context.WithTimeout(ctx, time.Duration(v.TimeoutMillis) * time.Millisecond)
    remaining, errs := d.c.Step(ctx2, v.BatchSize)
    for _, err := range errs {
      d.l.Error(err)
    }
    return &pb.CrawlResult{Remaining: int32(remaining)}, nil
  default:
    return nil, fmt.Errorf("Unrecognized command")
  }
}

func (d *Driver) handleGetPeersResponse(rep *pb.GetPeersResponse) []*peer.AddrInfo {
  ais := []*peer.AddrInfo{}
  naddr := 0
  for _, a := range rep.Addresses {
    if naddr >= MaxAddrPerPeer {
      break
    }
    if r, err := transport.ProtoToPeerAddrInfo(a); err != nil {
      d.l.Error("ProtoToPeerAddrInfo: %v", err)
    } else {
      ais = append(ais, r)
      naddr++
    }
  }
  return ais
}

func (d *Driver) crawlPeer(ctx context.Context, ai *peer.AddrInfo) ([]*peer.AddrInfo, error) {
  if err := d.st.LogPeerCrawl(ai.ID.String(), d.c.Started.Unix()); err != nil {
    if !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
      // Only log as error if it's not because we already inserted - which 
      // is expected.
      return nil, fmt.Errorf("LogPeerCrawl: %v", err)
    }
    return []*peer.AddrInfo{}, nil
  }

  rep := &pb.GetPeersResponse{}
  d.t.AddTempPeer(ai)
  if err := d.t.Call(ctx, ai.ID, "GetPeers", &pb.GetPeersRequest{}, rep); err != nil {
    return nil, fmt.Errorf("GetPeers of %s: %v", ai.ID.String(), err)
  } else {
    return d.handleGetPeersResponse(rep), nil
  }
}

package poll

import (
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
  "time"
  "context"
)

const (
	DefaultMaxPoll         = 100
  DefaultPollPeriod  = 60 * time.Second
  DefaultPollTimeout = 5 * time.Second
)

type Poller interface {
  Pause()
  Resume()
  Update(status *pb.PeerStatus)
  Epoch() (<-chan float64)
  Result() (<-chan *pb.PeersSummary)
}

type PollerImpl struct {
  ctx    context.Context
  epoch            chan float64
  result           chan *pb.PeersSummary
  ticker           *time.Ticker
	polling          chan struct{}
	pollResult       []*pb.PeerStatus
  peersSummary      *pb.PeersSummary

  maxPoll int
  pollPeriod time.Duration
  pollTimeout time.Duration
}

func New(ctx context.Context) *PollerImpl {
  t:= time.NewTicker(DefaultPollTimeout)
  t.Stop()
  pi := &PollerImpl{
    ctx: ctx,
    epoch: make(chan float64),
    result: make(chan *pb.PeersSummary),
    ticker: t,
    polling: nil,
    pollResult: []*pb.PeerStatus{},
    peersSummary: &pb.PeersSummary{},
    maxPoll: DefaultMaxPoll,
    pollPeriod: DefaultPollPeriod,
    pollTimeout: DefaultPollTimeout,
  }
  go pi.loop()
  return pi
}

func (p *PollerImpl) Epoch() (<-chan float64) {
  return p.epoch
}

func (p *PollerImpl) Result() (<-chan *pb.PeersSummary) {
  return p.result
}

func (p *PollerImpl) Pause() {
  p.ticker.Stop()
}

func (p *PollerImpl) Resume() {
  p.ticker.Reset(p.pollPeriod)
}

func (p *PollerImpl) Cleanup() {

}

func (p *PollerImpl) Update(status *pb.PeerStatus) {
	if p.polling != nil {
    p.pollResult = append(p.pollResult, status)
    if len(p.pollResult) >= p.maxPoll {
      close(p.polling)
    }
  }
}

func (p *PollerImpl) loop() {
  for {
    select {
      case <-p.ticker.C:
        if p.polling == nil {
          ctx2, cancel := context.WithTimeout(p.ctx, p.pollTimeout)
          defer cancel()
          go p.pollPeersSync(ctx2)
          p.ticker.Reset(p.pollPeriod)
        }
      case <-p.ctx.Done():
        return
    }
  }
}


func (p *PollerImpl) pollPeersSync(ctx context.Context) {
  prob := 0.9 // TODO adjust dynamically based on last poll performance
	p.pollResult = nil
  p.polling = make(chan struct{})

  p.epoch <- prob // Trigger upstream request for peers
  select {
  case <-p.polling: // Closed
  case <-ctx.Done():
  }
  p.polling = nil

  // Assuming received number of peers is statistical mode
  // len(pollResult) = floor((pop + 1)*prob)
  pop := int64(float64(len(p.pollResult)) / prob)
  p.peersSummary = &pb.PeersSummary{
    PeerEstimate: pop,
    Variance: float64(pop) * prob * (1 - prob),
  }
  select {
  case p.result <- p.peersSummary:
  default:
  }
}

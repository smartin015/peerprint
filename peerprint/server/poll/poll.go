package poll

import (
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
  "time"
  "context"
  "sync"
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
  done             chan struct{}
  result           chan *pb.PeersSummary
  ticker           *time.Ticker
	polling          bool
	pollResult       []*pb.PeerStatus
  peersSummary      *pb.PeersSummary
  m                 sync.RWMutex

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
    done: make(chan struct{}),
    result: make(chan *pb.PeersSummary),
    ticker: t,
    polling: false,
    pollResult: []*pb.PeerStatus{},
    peersSummary: &pb.PeersSummary{},
    maxPoll: DefaultMaxPoll,
    pollPeriod: DefaultPollPeriod,
    pollTimeout: DefaultPollTimeout,
    m: sync.RWMutex{},
  }
  go pi.loop()
  return pi
}

func (p *PollerImpl) isPolling() bool {
  p.m.RLock()
  v := p.polling
  p.m.RUnlock()
  return v
}

func (p *PollerImpl) SetPollPeriod(d time.Duration) {
  p.m.Lock()
  defer p.m.Unlock()
  p.pollPeriod = d
}

func (p *PollerImpl) getPollPeriod() time.Duration {
  p.m.RLock()
  v := p.pollPeriod
  p.m.RUnlock()
  return v
}

func (p *PollerImpl) SetPollTimeout(d time.Duration) {
  p.m.Lock()
  defer p.m.Unlock()
  p.pollTimeout = d
}

func (p *PollerImpl) getPollTimeout() time.Duration {
  p.m.RLock()
  v := p.pollTimeout
  p.m.RUnlock()
  return v
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
  p.ticker.Reset(p.getPollPeriod())
}

func (p *PollerImpl) Cleanup() {

}

func (p *PollerImpl) Update(status *pb.PeerStatus) {
	if p.isPolling() {
    p.pollResult = append(p.pollResult, status)
    if len(p.pollResult) >= p.maxPoll {
      p.done <- struct{}{}
    }
  }
}

func (p *PollerImpl) loop() {
  for {
    select {
      case <-p.ticker.C:
        if !p.isPolling() {
          ctx2, cancel := context.WithTimeout(p.ctx, p.getPollTimeout())
          defer cancel()
          go p.pollPeersSync(ctx2)
          p.ticker.Reset(p.getPollPeriod())
        }
      case <-p.ctx.Done():
        return
    }
  }
}


func (p *PollerImpl) pollPeersSync(ctx context.Context) {
  prob := 0.9 // TODO adjust dynamically based on last poll performance
  p.m.Lock()
  p.polling = true
	p.pollResult = nil
  p.m.Unlock()

  p.epoch <- prob // Trigger upstream request for peers
  select {
  case <-p.done:
  case <-ctx.Done():
  }
  p.m.Lock()
  p.polling = false
  p.m.Unlock()

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

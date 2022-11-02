package poll

import (
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
  "sync"
  "time"
  "context"
)

const (
	MaxPoll         = 100
  PollPeriod  = 5 * time.Second
  PollTimeout = 60 * time.Second
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
	polling          *sync.Cond
	pollResult       []*pb.PeerStatus
  peersSummary      *pb.PeersSummary
}

func New(ctx context.Context) Poller {
  t:= time.NewTicker(PollTimeout)
  t.Stop()
  pi := &PollerImpl{
    ctx: ctx,
    epoch: make(chan float64),
    ticker: t,
    polling: nil,
    pollResult: []*pb.PeerStatus{},
    peersSummary: &pb.PeersSummary{},
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
  p.ticker.Reset(PollPeriod)
}

func (p *PollerImpl) Cleanup() {

}

func (p *PollerImpl) Update(status *pb.PeerStatus) {
	if p.polling != nil {
    p.pollResult = append(p.pollResult, status)
    if len(p.pollResult) >= MaxPoll {
      p.polling.Broadcast()
    }
  }
}

func (p *PollerImpl) loop() {
  for {
    select {
      case <-p.ticker.C:
        if p.polling == nil {
          ctx2, _ := context.WithTimeout(p.ctx, PollPeriod)
          go p.pollPeersSync(ctx2)
          p.ticker.Reset(PollTimeout)
        }
      case <-p.ctx.Done():
        return
    }
  }
}


func (p *PollerImpl) pollPeersSync(ctx context.Context) {
  prob := 0.9 // TODO adjust dynamically based on last poll performance
	p.pollResult = nil
  l := &sync.Mutex{}
  l.Lock()
  p.polling = sync.NewCond(l)
  defer func() { p.polling = nil }()
  p.epoch <- prob

	await := make(chan bool)
	go func() {
		p.polling.Wait()
		await <- true
	}()
	select {
	case <-await:
	case <-ctx.Done():
	}

  // Assuming the received number of peers is the mode - most commonly occurring value,
  // len(pollResult) = floor((pop + 1)*prob)
  pop := int64(float64(len(p.pollResult)) / prob)
  p.peersSummary = &pb.PeersSummary{
    PeerEstimate: pop,
    Variance: float64(pop) * prob * (1 - prob),
  }
  p.result<- p.peersSummary
}

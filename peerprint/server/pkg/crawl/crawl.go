package crawl

import (
	"github.com/libp2p/go-libp2p/core/peer"
  "sync"
  "context"
  "time"
)


// Note: GetPeersResponse only contains peers which have not yet been crawled.
// It's the responsibilty of the implementer of crawlPeerFn to strip already
// crawled peers from server replies.
type crawlPeerFn func(ctx context.Context, ai *peer.AddrInfo) ([]*peer.AddrInfo, error)

type Crawler struct {
  Started time.Time
  crawlPeer crawlPeerFn
  next map[string]*peer.AddrInfo
  mut sync.Mutex // Prevent parallel reads/writes to `next`
}

func NewCrawler(start []*peer.AddrInfo, cpf crawlPeerFn) *Crawler {
  c := &Crawler{
    crawlPeer: cpf,
    next: make(map[string]*peer.AddrInfo),
    Started: time.Now(),
  }
  for _, p := range start {
    c.next[p.ID.String()] = p
  }
  return c
}

func (c *Crawler) Step(ctx context.Context, maxConn int64) (int, []error) {
  var wg sync.WaitGroup
  ncon := int64(0)
  c.mut.Lock()
  errs := []error{}
  for _, p := range c.next {
    ncon += 1
    if ncon > maxConn {
      continue
    }
    defer delete(c.next, p.ID.String())
    wg.Add(1)
    go func(p *peer.AddrInfo) {
      defer wg.Done()
      rep, err := c.crawlPeer(ctx, p)
      if err != nil {
        errs = append(errs, err)
      }
      for _, a := range rep {
        c.mut.Lock()
        if _, ok := c.next[a.ID.String()]; !ok {
          c.next[a.ID.String()] = a
        }
        c.mut.Unlock()
      }
    }(p)
  }
  c.mut.Unlock()
  wg.Wait()
  return len(c.next), errs
}

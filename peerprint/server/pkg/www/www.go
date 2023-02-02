package www

import (
  "sync"
  "io/fs"
  "context"
  "time"
  "net"
  "net/http"
  "encoding/json"
  "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "embed"
)

const (
  DBReadTimeout = 5*time.Second
)

//go:embed static
var static embed.FS

type webserver struct {
  l *log.Sublog
  s server.Interface
  st storage.Interface
}

func New(logger *log.Sublog, srv server.Interface, st storage.Interface) *webserver {
  return &webserver {
    l: logger,
    s: srv,
    st: st,
  }
}

func (s *webserver) handleGetEvents(w http.ResponseWriter, r *http.Request) {
  cur := make(chan storage.DBEvent, 5)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer storage.HandlePanic()
    defer wg.Done()
    for v := range cur {
      if data, err := json.Marshal(v); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
        return
      } else {
        w.Write(data)
        w.Write([]byte("\n"))
      }
    }
  }()
  ctx, _ := context.WithTimeout(context.Background(), DBReadTimeout)
  if err := s.st.GetEvents(ctx, cur, 1000); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleGetTimeline(w http.ResponseWriter, r *http.Request) {
  cur := make(chan *storage.DataPoint, 5)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer storage.HandlePanic()
    defer wg.Done()
    for v := range cur {
      if data, err := json.Marshal(v); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
        return
      } else {
        w.Write(data)
        w.Write([]byte("\n"))
      }
    }
  }()
  ctx, _ := context.WithTimeout(context.Background(), DBReadTimeout)
  if err := s.st.GetPeerTimeline(ctx, cur); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleGetPeerLogs(w http.ResponseWriter, r *http.Request) {
  cur := make(chan *storage.TimeProfile, 5)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
    defer storage.HandlePanic()
    defer wg.Done()
    for v := range cur {
      if data, err := json.Marshal(v); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
        return
      } else {
        w.Write(data)
        w.Write([]byte("\n"))
      }
    }
  }()
  ctx, _ := context.WithTimeout(context.Background(), DBReadTimeout)
  if err := s.st.GetPeerTracking(ctx, cur); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleServerSummary(w http.ResponseWriter, r *http.Request) {
  data, err := json.Marshal(s.s.GetSummary())
  if err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write(data)
}

func (s *webserver) handleStorageSummary(w http.ResponseWriter, r *http.Request) {
  summary, errs := s.st.GetSummary()
  for _, e := range errs {
    s.l.Error("handleStorageSummary: %v", e)
  }
  data, err := json.Marshal(summary)
  if err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write(data)
}

func (s *webserver) Serve(addr string, ctx context.Context, liveDir string) {
  if liveDir == "" {
    subFS, err := fs.Sub(static, "static")
    if err != nil {
      panic(err)
    }
    http.Handle("/", http.FileServer(http.FS(subFS)))
  } else {
    http.Handle("/", http.FileServer(http.Dir(liveDir)))
    s.l.Info("Serving www assets from %s", liveDir)
  }

  http.HandleFunc("/timeline", s.handleGetTimeline)
  http.HandleFunc("/peerLogs", s.handleGetPeerLogs)
  http.HandleFunc("/events", s.handleGetEvents)
  http.HandleFunc("/serverSummary", s.handleServerSummary)
  http.HandleFunc("/storageSummary", s.handleStorageSummary)

  l, err := net.Listen("tcp", addr)
  if err != nil {
      panic(err)
  }
  defer l.Close()

  s.l.Info("Starting status HTTP server on %s\n", l.Addr().(*net.TCPAddr).String())
  if err := http.Serve(l, nil); err != nil {
    s.l.Fatal(err)
  }
}

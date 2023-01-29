package www

import (
  "sync"
  "context"
  "time"
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

func (s *webserver) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal([]string{"hello", "world", "testing","json","transfer"})
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
	w.Write(data)
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
	data, err := json.Marshal(s.st.GetSummary())
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
	w.Write(data)
}

func (s *webserver) Serve(addr string, ctx context.Context) {
	s.l.Info("Starting status HTTP server at %s\n", addr)

	fileServer := http.FileServer(http.FS(static))
	http.Handle("/", fileServer)
	http.HandleFunc("/history", s.handleGetHistory)
	http.HandleFunc("/events", s.handleGetEvents)
	http.HandleFunc("/serverSummary", s.handleServerSummary)
	http.HandleFunc("/storageSummary", s.handleStorageSummary)
	if err := http.ListenAndServe(addr, nil); err != nil {
		s.l.Fatal(err)
	}
}

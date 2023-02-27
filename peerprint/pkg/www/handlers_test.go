package www

import (
  "testing"
  "log"
  "bytes"
  "strings"
  "net/url"
  "net/http"
  "path/filepath"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
)

type testResponseWriter struct {
  status int
  Buf bytes.Buffer
}
func (w *testResponseWriter) Header() http.Header {
  return make(map[string][]string) // Swallow header changes
}
func (w *testResponseWriter) Write(data []byte) (int, error) {
  w.Buf.Write(data)
  return 0, nil
}
func (w *testResponseWriter) WriteHeader(status int) {
  w.status = status
}

func testRequest(query string) *http.Request {
  if v, err := url.ParseQuery(query); err != nil {
    panic(err)
  } else {
    return &http.Request{
      Form: v,
    }
  }
}

func testRW() *testResponseWriter{
  return &testResponseWriter{}
}

func testWebserver(t *testing.T, withLAN bool) *webserver {
  dir := t.TempDir()
  d := driver.NewTestDriverImpl(t,withLAN)
  return New(pplog.New("www", log.Default()), d, &Opts{
    ConfigPath: filepath.Join(dir, "config.yaml"),
    CookieStoreKey: []byte("testcookies"),
  })
}

func TestHandleIndexAndLogin(t *testing.T) {
  s := testWebserver(t, false)
  w := testRW()
  s.handleIndex(w, testRequest(""))
  if got := w.Buf.String(); !strings.HasPrefix(got, "<html>") {
    t.Errorf("Expected HTML, got %q", got)
  }

  w.Buf.Reset()
  s.handleLogin(w, testRequest(""))
  if got := w.Buf.String(); !strings.HasPrefix(got, "<html>") {
    t.Errorf("Expected HTML, got %q", got)
  }
}

func TestWithLANConnection(t *testing.T) {
  // Testing with a connection is pretty heavy, require generating x509 certs
  // for RPC command server, starting up libp2p transport, etc.
  // We run these read-only subtests with a single initialized server to reduce testing load
  s := testWebserver(t, true)

  t.Run("testGetConn", func(t *testing.T) {
    w := testRW()
    s.handleGetConn(w, testRequest(""))
    if got := w.Buf.String(); !strings.Contains(got, "\"network\":\"LAN\"") {
      t.Errorf("Expected LAN connection JSON, got %q", got)
    }
  })

  t.Run("testGetEvents", func(t *testing.T) {
    w := testRW()
    s.handleGetEvents(w, testRequest("instance=LAN"))
    if got := w.Buf.String(); got != "" {
      t.Errorf("Expected empty, got %q", got)
    }
  })

  t.Run("testGetRegistry", func(t *testing.T) {
    w := testRW()
    s.handleGetRegistry(w, testRequest(""))
    if got := w.Buf.String(); got != "" {
      t.Errorf("Expected empty, got %q", got)
    }
  })

  t.Run("testLobby", func(t *testing.T) {
    w := testRW()
    s.handleGetLobby(w, testRequest(""))
    if got := w.Buf.String(); len(got) < 100 {
      t.Errorf("Expected some JSON, got %q", got)
    }
  })

  t.Run("testPrinterLocations", func(t *testing.T) {
    w := testRW()
    s.handleGetPrinterLocations(w, testRequest("instance=LAN"))
    if got := w.Buf.String(); got != "" {
      t.Errorf("Expected empty, got %q", got)
    }
  })

  t.Run("testTimeline", func(t *testing.T) {
    w := testRW()
    s.handleGetTimeline(w, testRequest("instance=LAN"))
    if got := w.Buf.String(); got != "" {
      t.Errorf("Expected empty, got %q", got)
    }
  })

  t.Run("testPeerLogs", func(t *testing.T) {
    w := testRW()
    s.handleGetPeerLogs(w, testRequest("instance=LAN"))
    if got := w.Buf.String(); got != "" {
      t.Errorf("Expected empty, got %q", got)
    }
  })

  t.Run("testGetCredentials", func(t *testing.T) {
    w := testRW()
    s.handleGetCredentials(w, testRequest(""))
    if got := w.Buf.String(); got != "[]" {
      t.Errorf("Expected empty list, got %q", got)
    }
  })

  t.Run("testServerSummary", func(t *testing.T) {
    w := testRW()
    s.handleServerSummary(w, testRequest("instance=LAN"))
    if got := w.Buf.String(); !strings.Contains(got, "Connection") {
      t.Errorf("Expected server status JSON with 'Connection', got %q", got)
    }
  })

  t.Run("testStorageSummary", func(t *testing.T) {
    w := testRW()
    s.handleStorageSummary(w, testRequest("instance=LAN"))
    if got := w.Buf.String(); !strings.Contains(got, "LAN.sqlite3") {
      t.Errorf("Expected storage JSON with LAN.sqlite3, got %q", got)
    }
  })
}

func TestHandleSyncLobby(t *testing.T) {
  t.Skip("TODO");
}

func TestHandleConnNewDelete(t *testing.T) {
  t.Skip("TODO");
  // NewConn
  // DeleteConn
}

func TestHandleSetPrinterStatus(t *testing.T) {
  t.Skip("TODO");
}

func TestHandleRegistryNewDelete(t *testing.T) {
  t.Skip("TODO");
  // NewRegistry
  // DeleteRegistry
}

func TestHandleNewPassword(t *testing.T) {
  t.Skip("TODO");
}

func TestHandleRoot(t *testing.T) {
  t.Skip("TODO");
}

func TestHandleRemoveCredentials(t *testing.T) {
  t.Skip("TODO");
}

package www

import (
  "strconv"
  "strings"
  "sync"
  "context"
  "html/template"
  "time"
  "net/http"
  "encoding/json"
  "google.golang.org/protobuf/encoding/protojson"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/google/uuid"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
)

const (
  DBReadTimeout = 5*time.Second
)

func (s *webserver) handleIndex(w http.ResponseWriter, r *http.Request) {
  tmpl, err := template.ParseFS(s.f, "*.html")
  if err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  if err := tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
}

func (s *webserver) handleGetEvents(w http.ResponseWriter, r *http.Request) {
  streamingReadInstance[storage.DBEvent](s, w, r, func(ctx context.Context, n *driver.Instance, cur chan storage.DBEvent) error {
    return n.St.GetEvents(ctx, cur, 1000)
  })
}

func (s *webserver) handleSyncLobby(w http.ResponseWriter, r *http.Request) {
  d, err := strconv.Atoi(r.FormValue("seconds"))
  if err != nil {
    w.WriteHeader(400)
    w.Write([]byte("failed to parse seconds"))
    return
  }
  dt := time.Duration(d) * time.Second
  go func() {
    if err := s.d.RLocal.Run(dt); err != nil {
      s.l.Error("Local sync: %s", err.Error())
    }
  }()
  go func() {
    if err := s.d.RWorld.Run(dt); err != nil {
      s.l.Error("Global sync: %s", err.Error())
    }
  }()
  w.Write([]byte("ok"))
}

func (s *webserver) handleGetLobby(w http.ResponseWriter, r *http.Request) {
  cur := make(chan *pb.Network, 5)

  if v, err := json.Marshal(s.d.RLocal.Counters); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  } else {
    w.Write(v)
    w.Write([]byte("\n"))
  }
  if v, err := json.Marshal(s.d.RWorld.Counters); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  } else {
    w.Write(v)
    w.Write([]byte("\n"))
  }

  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
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
  if err := s.d.RLocal.DB.GetRegistry(ctx, cur, storage.LobbyTable, false); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  if err := s.d.RWorld.DB.GetRegistry(ctx, cur, storage.LobbyTable, true); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleGetRegistry(w http.ResponseWriter, r *http.Request) {
  cur := make(chan *pb.Network, 5)
  var wg sync.WaitGroup
  wg.Add(1)
  go func() {
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
  if err := s.d.RLocal.DB.GetRegistry(ctx, cur, storage.RegistryTable, false); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  if err := s.d.RWorld.DB.GetRegistry(ctx, cur, storage.RegistryTable, true); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  wg.Wait()
}

func (s *webserver) handleGetPrinterLocations(w http.ResponseWriter, r *http.Request) {
  streamingReadInstance[*pb.Location](s, w, r, func(ctx context.Context, n *driver.Instance, cur chan *pb.Location) error {
    return n.St.GetPrinterLocations(ctx, time.Now().Unix() - 60*60*24, cur)
  })
}

func (s *webserver) handleGetTimeline(w http.ResponseWriter, r *http.Request) {
  streamingReadInstance[*storage.DataPoint](s, w, r, func(ctx context.Context, n *driver.Instance, cur chan *storage.DataPoint) error {
    return n.St.GetPeerTimeline(ctx, cur)
  })
}

func (s *webserver) handleGetConn(w http.ResponseWriter, r *http.Request) {
  v := s.d.GetConfigs(true)
  for _, c := range v {
    j := &protojson.MarshalOptions{EmitUnpopulated: true}
    if data, err := j.Marshal(c); err != nil {
      w.WriteHeader(500)
      w.Write([]byte(err.Error()))
      return
    } else {
      w.Write(data)
      w.Write([]byte("\n"))
    }
    }
}

func (s *webserver) handleGetPeerLogs(w http.ResponseWriter, r *http.Request) {
  streamingReadInstance[*storage.TimeProfile](s, w, r, func(ctx context.Context, n *driver.Instance, cur chan *storage.TimeProfile) error {
    return n.St.GetPeerTracking(ctx, cur)
  })
}

func (s *webserver) handleServerSummary(w http.ResponseWriter, r *http.Request) {
  readInstance[*server.Summary](s, w, r, func(n *driver.Instance) (*server.Summary, error) {
    summary := n.S.GetSummary()
    return summary, nil
  })
}

func (s *webserver) handleStorageSummary(w http.ResponseWriter, r *http.Request) {
  readInstance[*storage.Summary](s, w, r, func(n *driver.Instance) (*storage.Summary, error) {
    summary, errs := n.St.GetSummary()
    for _, e := range errs {
      s.l.Error("handleStorageSummary: %v", e)
    }
    return summary, nil
  })
}

func (s *webserver) handleDeleteConn(w http.ResponseWriter, r *http.Request) {
  if v := r.PostFormValue("network"); v == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing network"))
      return
  } else {
    req := &pb.DisconnectRequest{Network: v}
    if _, err := s.d.Command.Disconnect(r.Context(), req); err != nil {
      w.WriteHeader(500)
      w.Write([]byte(err.Error()))
    }
    w.Write([]byte("ok"))
  }
}

func (s *webserver) handleSetPrinterStatus(w http.ResponseWriter, r *http.Request) {
  vs := make(map[string]string)
  for _, k := range []string{"network", "name", "active_record", "active_unit", "status", "profile", "latitude", "longitude"} {
    if v := r.PostFormValue(k); v == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing form item: " + k))
      return
    } else {
      vs[k] = v
    }
  }
  n := s.getInstance(r, w)
  if n == nil {
    w.WriteHeader(400)
    w.Write([]byte("Instance no found: " + vs["network"]))
    return
  }

  lonF, _ := strconv.ParseFloat(vs["longitude"], 64)
  latF, _ := strconv.ParseFloat(vs["latitude"], 64)
  if _, err := n.St.SetPeerStatus(vs["network"], &pb.PrinterStatus{
    Name: vs["name"],
    ActiveRecord: vs["active_record"],
    ActiveUnit: vs["active_unit"],
    Status: vs["status"],
    Profile: vs["profile"],
    Location: &pb.Location{
            Latitude: latF,
            Longitude: lonF,
    },
  }); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write([]byte("ok"))
}

func (s *webserver) handleNewConn(w http.ResponseWriter, r *http.Request) {
  vs := make(map[string]string)
  for _, k := range []string{"network", "rendezvous", "psk", "local"} {
    if v := r.PostFormValue(k); v == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing form item: " + k))
      return
    } else {
      vs[k] = v
    }
  }
  net := vs["network"]

  req := &pb.ConnectRequest{
    Network: net,
    Addr: get(r, "addr", "/ip4/0.0.0.0/tcp/0"),
    Rendezvous: vs["rendezvous"],
    Psk: vs["psk"],
    Local: vs["local"] == "true",
    DbPath: get(r, "db_path", net + ".sqlite3"),
    PrivkeyPath: get(r, "privkey_path", net + ".priv"),
    PubkeyPath: get(r, "pubkey_path", net + ".pub"),
    DisplayName: get(r, "display_name", "anonymous"),
    ConnectTimeout: 0,
    SyncPeriod: 60*5,
    MaxRecordsPerPeer: 100,
    MaxTrackedPeers: 100,
  }
  if _, err := s.d.Command.Connect(r.Context(), req); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write([]byte("ok"))
}

func (s *webserver) handleDeleteRegistry(w http.ResponseWriter, r *http.Request) {
  if u := r.PostFormValue("uuid"); u == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing uuid"))
      return
    } else if l := r.PostFormValue("local"); l == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing local"))
      return
    } else {
      req := &pb.StopAdvertisingRequest{Uuid: u, Local: l == "true"}
      if _, err := s.d.Command.StopAdvertising(r.Context(), req); err != nil {
        w.WriteHeader(500)
        w.Write([]byte(err.Error()))
      }
      w.Write([]byte("ok"))
  }
}


func (s *webserver) handleNewRegistry(w http.ResponseWriter, r *http.Request) {
  vs := make(map[string]string)
  for _, k := range []string{"name", "rendezvous", "local"} {
    if v := r.PostFormValue(k); v == "" {
      w.WriteHeader(400)
      w.Write([]byte("Missing form item: " + k))
      return
    } else {
      vs[k] = v
    }
  }
  tags := []string{}
  for _, s := range strings.Split(get(r, "tags", ""), ",") {
    tags = append(tags, strings.TrimSpace(s))
  }
  links := []string{}
  for _, s := range strings.Split(get(r, "links", ""), ",") {
    links = append(links, strings.TrimSpace(s))
  }
  cfg := &pb.NetworkConfig {
    Uuid: uuid.New().String(),
    Name: vs["name"],
    Description: get(r, "description", ""),
    Tags: tags,
    Links: links,
    Location: get(r, "location", "unknown"),
    Rendezvous: vs["rendezvous"],
    Creator: get(r, "creator", "anonymous"),
    Created: time.Now().Unix(),
  }
  if err := s.d.Command.ResolveRegistry(vs["local"] == "true").DB.UpsertConfig(cfg, []byte(""), storage.RegistryTable); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write([]byte("ok"))
}

func (s *webserver) handleNewPassword(w http.ResponseWriter, r *http.Request) {
  p := r.PostFormValue("password")
  if err := s.d.SetAdminPassAndSalt(p); err != nil {
    w.WriteHeader(500)
    w.Write([]byte(err.Error()))
  }
  w.Write([]byte("ok"))
}

func (s *webserver) handleRoot(w http.ResponseWriter, r *http.Request) {
  if r.Method != "GET" {
    w.WriteHeader(404)
    return
  }
  if r.URL.Path == "/" {
    s.handleIndex(w, r)
  } else {
    s.fsh.ServeHTTP(w, r)
  }
}

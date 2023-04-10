package driver

import (
  "path"
  "path/filepath"
	"github.com/libp2p/go-libp2p/core/peer"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/smartin015/peerprint/p2pgit/pkg/transport"
  "github.com/smartin015/peerprint/p2pgit/pkg/storage"
  "github.com/smartin015/peerprint/p2pgit/pkg/crawl"
  pplog "github.com/smartin015/peerprint/p2pgit/pkg/log"
  "github.com/smartin015/peerprint/p2pgit/pkg/server"
  "context"
	lp2p_crypto "github.com/libp2p/go-libp2p/core/crypto"
  "github.com/smartin015/peerprint/p2pgit/pkg/crypto"
	"github.com/libp2p/go-libp2p/core/pnet"
  "fmt"
  "time"
  "strings"
)


type Instance struct {
  name string
  S server.Interface
  St storage.Interface
  t transport.Interface
  c *crawl.Crawler
  l *pplog.Sublog
  cancel context.CancelFunc
}

func safeJoin(base string, tail string) string {
  return filepath.Join(base, filepath.FromSlash(path.Clean("/"+strings.Trim(tail, "/"))))
}

func NewInstance(v *pb.ConnectRequest, baseDir string, l *pplog.Sublog) (*Instance, error) {
  var st storage.Interface
  var err error
  st, err = storage.NewSqlite3(safeJoin(baseDir, v.DbPath))
  if err != nil {
    return nil, fmt.Errorf("Error initializing DB: %w", err)
  }
  storage.SetPanicHandler(st)

	if v.Rendezvous == "" {
		return nil, fmt.Errorf("rendezvous must be specified!")
	}

  var kpriv lp2p_crypto.PrivKey
  var kpub lp2p_crypto.PubKey
  if v.PrivkeyPath == "" && v.PubkeyPath == "" {
    l.Warning("WARNING: generating ephemeral key pair; this will change on restart")
    kpriv, kpub, err = crypto.GenKeyPair()
    if err != nil {
      panic(fmt.Errorf("Error generating ephemeral keys: %w", err))
    }
  } else {
    kpriv, kpub, err = crypto.LoadOrGenerateKeys(
      safeJoin(baseDir, v.PrivkeyPath),
      safeJoin(baseDir, v.PubkeyPath))
    if err != nil {
      panic(fmt.Errorf("Error loading keys: %w", err))
    }
  }

  var psk pnet.PSK
  if v.Psk == "" && !v.Local {
    l.Warning("\n\n\n ================= WARNING =================\n\n" +
      "No PSK path is set - your GLOBAL network will be INSECURE!\n" +
      "It is STRONGLY RECOMMENDED to specify a PSK in your connection\n" +
      "or else anybody can become a node in your network\n" +
      "\n ================= WARNING =================\n\n\n")
  } else {
    psk = crypto.LoadPSK(v.Psk)
  }

  ctx := context.Background()
  t, err := transport.New(&transport.Opts{
    Addr: v.Addr,
    Rendezvous: v.Rendezvous,
    Local: v.Local,
    ConnectOnDiscover: true,
    PrivKey: kpriv,
    PubKey: kpub,
    PSK: psk,
    ConnectTimeout: time.Duration(v.ConnectTimeout) * time.Second,
    Topics: []string{server.DefaultTopic},
    ExtraBootstrapPeers: v.ExtraBootstrapPeers,
    ExtraRelayPeers: v.ExtraRelayPeers,
  }, ctx, l)
  if err != nil {
    return nil, fmt.Errorf("Error initializing transport layer: %w", err)
  }

  id, err := peer.IDFromPublicKey(kpub)
  name := id.Pretty()
  name = name[len(name)-4:]
	st.SetId(id.String())

  s := server.New(t, st, &server.Opts{
    SyncPeriod: time.Duration(v.SyncPeriod) * time.Second,
    DisplayName: v.DisplayName,
    MaxRecordsPerPeer: v.MaxRecordsPerPeer,
    MaxTrackedPeers: v.MaxTrackedPeers,
  }, pplog.New(name, l))

  return &Instance{
    name: v.Network,
    S: s,
    St: st,
    t: t,
    c: nil,
    l: l,
  }, nil
}

func (i *Instance) Run(ctx context.Context) {
  ctx2, cancel := context.WithCancel(ctx)
  i.cancel = cancel
  i.S.Run(ctx2)
}

func (i *Instance) Destroy() {
  if i.cancel != nil {
    i.cancel()
  }
  i.t.Destroy()
  i.St.Close()
}

func (d *Instance) crawlPeer(ctx context.Context, ai *peer.AddrInfo) ([]*peer.AddrInfo, error) {
  if err := d.St.LogPeerCrawl(ai.ID.String(), d.c.Started.Unix()); err != nil {
    if !strings.HasPrefix(err.Error(), "UNIQUE constraint failed") {
      // Only log as error if it's not because we already inserted - which 
      // is expected.
      return nil, fmt.Errorf("LogPeerCrawl: %v", err)
    }
    return []*peer.AddrInfo{}, nil
  }

  // Peer status is not essential in crawling, but if successfully retrieved
  // can provide info to users about printer locations etc.
  stat := &pb.PeerStatus{}
  if err := d.t.Call(ctx, ai.ID, "GetStatus", &pb.GetStatusRequest{}, stat); err != nil {
    d.l.Warning("GetStatus: %v", err)
  } else if err := d.St.SetPeerStatus(d.name, stat); err != nil {
    d.l.Warning("SetPeerStatus: %v", err)
  }

  rep := &pb.GetPeersResponse{}
  d.t.AddTempPeer(ai)
  if err := d.t.Call(ctx, ai.ID, "GetPeers", &pb.GetPeersRequest{}, rep); err != nil {
    return nil, fmt.Errorf("GetPeers of %s: %v", ai.ID.String(), err)
  } else {
    return d.handleGetPeersResponse(rep), nil
  }
}


func (d *Instance) handleGetPeersResponse(rep *pb.GetPeersResponse) []*peer.AddrInfo {
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

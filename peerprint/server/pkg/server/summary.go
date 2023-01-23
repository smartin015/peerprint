package server

import (
	"github.com/libp2p/go-libp2p/core/peer"

)

type Summary struct {
  ID string
  Peers []peer.AddrInfo
  Connection string
  LastSyncStart int64
  LastSyncEnd int64
  LastMessage int64
}

func (s *Server) GetSummary() *Summary {
  return &Summary{
    ID: s.ID(),
    Peers: s.t.GetPeerAddresses(),
    Connection: s.connStr,
    LastSyncStart: s.lastSyncStart.Unix(),
    LastSyncEnd: s.lastSyncEnd.Unix(),
    LastMessage: s.lastMsg.Unix(),
  }
}

// package server implements handlers for peerprint service
package server

import (
	"fmt"
	pb "github.com/smartin015/peerprint/pubsub/proto"
	"github.com/smartin015/peerprint/pubsub/raft"
)

type ServerState struct {
	r *raft.RaftImpl
	s *pb.State // Use this when raft not initialized
}

func NewServerState() *ServerState {
	return &ServerState{}
}

func (s *ServerState) Set(r *raft.RaftImpl, st *pb.State) {
	s.r = r
	s.s = st
}

func (s *ServerState) Get() (*pb.State, error) {
	if s.r != nil {
		return s.r.GetState()
	}
	if s.s != nil {
		return s.s, nil
	}
	return nil, fmt.Errorf("No state available to satisfy ServerState.Get()")
}

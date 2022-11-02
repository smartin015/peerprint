package raft

import (
  "testing"
  "bytes"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
  "google.golang.org/protobuf/proto"
)

func TestMarshalUnmarshal(t *testing.T) {
  s := &pb.State{
    Jobs: make(map[string]*pb.Job),
  }
  s.Jobs["asdf"] = &pb.Job{}

  ms := MarshableState{State: s}

  buf := new(bytes.Buffer)
  if err := ms.Marshal(buf); err != nil {
    t.Fatalf("Marshal error %v", err)
  }

  ms2 := MarshableState{State: &pb.State{}}
  if err := ms2.Unmarshal(bytes.NewReader(buf.Bytes())); err != nil {
    t.Fatalf("Unmarshal error %v", err)
  }

  if !proto.Equal(ms.State, ms2.State) {
    t.Errorf("Unmarshal from marshal: want %v got %v", ms, ms2)
  }
}

func TestUnmarshalError(t *testing.T) {
  ms := MarshableState{State: &pb.State{}}
  if err := ms.Unmarshal(bytes.NewReader([]byte{1,2,3})); err == nil {
    t.Errorf("Expected unmarshal error")
  }
}



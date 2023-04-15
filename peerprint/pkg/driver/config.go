package driver

import (
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
)

type DriverConfig struct {
  Connections map[string]*pb.ConnectRequest
}

func NewConfig() *DriverConfig {
  d := &DriverConfig{
    Connections: make(map[string]*pb.ConnectRequest),
  }
  return d
}


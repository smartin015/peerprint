package discovery

import (
  "context"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/event"
	"github.com/libp2p/go-libp2p-core/peerstore"
  ma "github.com/multiformats/go-multiaddr"
)

//https://pkg.go.dev/github.com/libp2p/go-libp2p@v0.23.2/core/host#Host
type MockHost struct {
}
func (m *MockHost) ID() peer.ID { 
  id, err := peer.Decode("12D3KooWNgjdBgmgRyY42Eo2Jg3rGyvaepU4QDkEMjy4WtF3Ad9V")
  if err != nil {
    panic(err)
  }
  return id
}
func (m *MockHost) Peerstore() peerstore.Peerstore { return nil }
func (m *MockHost) Addrs() []ma.Multiaddr { return nil }
func (m *MockHost) Network() network.Network { return nil }
func (m *MockHost) Mux() protocol.Switch { return nil }
func (m *MockHost) Connect(ctx context.Context, pi peer.AddrInfo) error { return nil }
func (m *MockHost) SetStreamHandler(pid protocol.ID, handler network.StreamHandler) {}
func (m *MockHost) SetStreamHandlerMatch(protocol.ID, func(string) bool, network.StreamHandler) {}
func (m *MockHost) RemoveStreamHandler(pid protocol.ID) {}
func (m *MockHost) NewStream(ctx context.Context, p peer.ID, pids ...protocol.ID) (network.Stream, error) { return nil, nil }
func (m *MockHost) Close() error { return nil }
func (m *MockHost) ConnManager() connmgr.ConnManager { return nil }
func (m *MockHost) EventBus() event.Bus { return nil }

// GetConnections is a testing method that returns the number of connected peers attempted
func (m *MockHost) GetConnections() []string {
  return nil
}

func NewMockHost() *MockHost {
  return &MockHost{}
}

package registry_generator

import (
  "fmt"
  "github.com/google/uuid"
  "time"
  "path/filepath"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
  "gopkg.in/yaml.v3"
  "crypto/rand"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
  "os"
)

func GenKeyPairFile(privkeyFile, pubkeyFile string) (crypto.PrivKey, crypto.PubKey, error) {
		priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
		if err != nil {
			return nil, nil, fmt.Errorf("Generating keypair error: %w", err)
		}

		data, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return nil, nil, fmt.Errorf("Marshal private key: %w", err)
		}
		if err := os.WriteFile(privkeyFile, data, 0644); err != nil {
			return nil, nil, fmt.Errorf("Write %s: %w", privkeyFile, err)
		}

		data, err = crypto.MarshalPublicKey(pub)
		if err != nil {
			return nil, nil, fmt.Errorf("Marshal public key: %w", err)
		}
		if err := os.WriteFile(pubkeyFile, data, 0644); err != nil {
			return nil, nil, fmt.Errorf("Write %s: %w", pubkeyFile, err)
		}
    return priv, pub, nil
}

func writeAsYaml(reg *pb.Registry, dest string) error {
  ydata, err := yaml.Marshal(reg)
  if err != nil {
    return err
  }
  return os.WriteFile(dest, ydata, 0644)
}

func genRegistry(trustedPeers []crypto.PubKey) (*pb.Registry, error) {
  tp := []string{}
  for _, p := range trustedPeers {
    id, err := peer.IDFromPublicKey(p)
    if err != nil {
      return nil, err
    }
    tp = append(tp, id.String())
  }

  q := &pb.Queue{
    Name: "TODO: Name this queue",
    Desc: "TODO: Provide a useful description for why users should join this queue",
    Url: "TODO: Provide a working URL with more information about the queue",
    Rendezvous: uuid.New().String(),
    TrustedPeers: tp,
  }

  r := &pb.Registry{
    Created: uint64(time.Now().Unix()),
    Url: "TODO: Provide a useful URL linking to registry maintainer details",
    Queues: []*pb.Queue{q},
  }
  return r, nil
}

func GenRegistryFiles(num_peers int, dest_dir string) error {
  pubks := []crypto.PubKey{}
  for i := 0; i < num_peers; i++ {
    _, pub, err := GenKeyPairFile(
      filepath.Join(dest_dir, fmt.Sprintf("trusted_peer_%d.priv", i)),
      filepath.Join(dest_dir, fmt.Sprintf("trusted_peer_%d.pub", i)))
    if err != nil {
      return err
    }
    pubks = append(pubks, pub)
  }
  // TODO file locations
  reg, err := genRegistry(pubks)
  if err != nil {
    return err
  }
  return writeAsYaml(reg, filepath.Join(dest_dir, "registry.yaml"))
}

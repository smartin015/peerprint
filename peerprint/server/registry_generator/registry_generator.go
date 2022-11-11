package registry_generator

import (
  "fmt"
  "github.com/google/uuid"
  "time"
  "path/filepath"
	pb "github.com/smartin015/peerprint/peerprint_server/proto"
  "github.com/ghodss/yaml"
  "crypto/rand"
	"github.com/libp2p/go-libp2p/core/crypto"
  "google.golang.org/protobuf/encoding/protojson"
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

func LoadKeys(privkeyFile string, pubkeyFile string) (crypto.PrivKey, crypto.PubKey, error) {
		data, err := os.ReadFile(privkeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("Read %s: %w", privkeyFile, err)
		}
		priv, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, nil, fmt.Errorf("UnmarshalPrivateKey: %w", err)
		}

		data, err = os.ReadFile(pubkeyFile)
		if err != nil {
			return nil, nil, fmt.Errorf("Read %s: %w", pubkeyFile, err)
		}
		pub, err := crypto.UnmarshalPublicKey(data)
		if err != nil {
			return nil, nil, fmt.Errorf("UnmarshalPublicKey: %w", err)
		}
		return priv, pub, nil

}

func writeAsYaml(reg *pb.Registry, dest string) error {
  // We must convert the proto to JSON before converting
  // to yaml, otherwise field names will not be correctly
  // specified.
  jdata, err := protojson.Marshal(reg)
  if err != nil {
    return err
  }
  ydata, err := yaml.JSONToYAML(jdata)
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

func LoadRegistry(path string) (*pb.Registry, error) {
	data, err := os.ReadFile(path)
  if err != nil {
    return nil, err
  }
  j, err := yaml.YAMLToJSON(data)
	if err != nil {
    return nil, err
	}
	reg := &pb.Registry{}
	err = protojson.Unmarshal(j, reg)
	if err != nil {
    return nil, err
	}
  return reg, nil
}

func GenRegistryFiles(num_peers int, dest_dir string) error {
  pubks := []crypto.PubKey{}
  for i := 0; i < num_peers; i++ {
    _, pub, err := GenKeyPairFile(
      filepath.Join(dest_dir, fmt.Sprintf("trusted_peer_%d.priv", i+1)),
      filepath.Join(dest_dir, fmt.Sprintf("trusted_peer_%d.pub", i+1)))
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

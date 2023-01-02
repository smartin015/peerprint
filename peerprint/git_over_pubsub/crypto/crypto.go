package crypto

import (
  "fmt"
  "crypto/rand"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/pnet"
  "os"
	b64 "encoding/base64"
)

func GenPSKFile(pskFile string) error {
	// https://github.com/libp2p/specs/blob/master/pnet/Private-Networks-PSK-V1.md
	f, err := os.OpenFile(pskFile, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString("/key/swarm/psk/1.0.0/\n") // This is a PSK
	f.WriteString("/base64/\n") // Encoded with base64

  // Key must be exactly 32 bytes per the spec
	token := make([]byte, 32)
  rand.Read(token)
  f.WriteString(b64.StdEncoding.EncodeToString(token))
  return nil
}

func LoadPSKFile(pskFile string) (pnet.PSK, error) {
	f, err := os.Open(pskFile)
  if err != nil {
    return nil, err
  }
	return pnet.DecodeV1PSK(f)
}

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

func fileExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func LoadOrGenerateKeys(privkeyFile string, pubkeyFile string) (crypto.PrivKey, crypto.PubKey, error) {
	privEx := fileExists(privkeyFile)
	pubEx := fileExists(pubkeyFile)
	if pubEx != privEx {
		return nil, nil, fmt.Errorf("Partial existance of public/private keys, cannot continue: (public %v, private %v)", pubEx, privEx)
	}

	if privEx && pubEx {
		return LoadKeys(privkeyFile, pubkeyFile)
	} else {
		return GenKeyPairFile(privkeyFile, pubkeyFile)
	}
}

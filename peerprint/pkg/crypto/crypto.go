package crypto

import (
  "path/filepath"
  "fmt"
  "crypto/rand"
  "crypto/x509"
  "crypto/tls"
  "crypto/sha256"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/pnet"
  "os"
)

func LoadPSK(phrase string) pnet.PSK {
  hash := sha256.New()
  hash.Write([]byte(phrase))
	return pnet.PSK(hash.Sum(nil))
}

func GenKeyPair() (crypto.PrivKey, crypto.PubKey, error) {
	return crypto.GenerateEd25519Key(rand.Reader)
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

func NewTLSConfig(certsDir, rootCert, serverCert, serverKey string) (*tls.Config, error) {
  scp := filepath.Join(certsDir, serverCert)
  skp := filepath.Join(certsDir, serverKey)
  cert, err := tls.LoadX509KeyPair(scp, skp)
  if err != nil {
    return nil, fmt.Errorf("Load server cert from %s, %s: %w", scp, skp, err)
  }

  rcp := filepath.Join(certsDir, rootCert)
  rcp_pem, err := os.ReadFile(rcp)
  if err != nil {
    return nil, fmt.Errorf("read rcp file %w", err)
  }

  ccp := x509.NewCertPool()
  if ok := ccp.AppendCertsFromPEM(rcp_pem); !ok {
    return nil, fmt.Errorf("failed to parse any root certificates from %s", rcp_pem)
  }

  return &tls.Config{
    Certificates: []tls.Certificate{cert},
    InsecureSkipVerify: true,
    RootCAs: ccp,
    ClientCAs: ccp,
    ClientAuth: tls.RequireAndVerifyClientCert,
  }, nil
}

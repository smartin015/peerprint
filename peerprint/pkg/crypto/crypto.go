package crypto

import (
  "math/big"
  "bytes"
  "time"
  "path/filepath"
  "net"
  "fmt"
  "crypto/rsa"
  "crypto/rand"
  "crypto/x509"
  "crypto/tls"
  "crypto/x509/pkix"
  "encoding/pem"
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

func WritePEM(cert []byte, certPrivKey *rsa.PrivateKey, certDest, keyDest string) error {
  certPEM := new(bytes.Buffer)
  if err := pem.Encode(certPEM, &pem.Block{
    Type:  "CERTIFICATE",
    Bytes: cert,
  }); err != nil {
    return fmt.Errorf("encode certificate: %w", err)
  }

  certPrivKeyPEM := new(bytes.Buffer)
  if err := pem.Encode(certPrivKeyPEM, &pem.Block{
    Type:  "RSA PRIVATE KEY",
    Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
  }); err != nil {
    return fmt.Errorf("encode private key: %w", err)
  }

  if err := os.WriteFile(certDest, certPEM.Bytes(), 0644); err != nil {
    return fmt.Errorf("write cert: %w", err)
  }
  if err := os.WriteFile(keyDest, certPrivKeyPEM.Bytes(), 0600); err != nil {
    return fmt.Errorf("write cert: %w", err)
  }

  return nil
}

func SelfSignedCACert(cname string) (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
    Subject: pkix.Name{
      CommonName: cname, 
    },
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
  caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
  if err != nil {
    return nil, nil, nil, err
  }
  caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
  if err != nil {
    return nil, nil, nil, err
  }

  return ca, caBytes, caPrivKey, nil
}

func CASignedCert(ca *x509.Certificate, caPrivKey *rsa.PrivateKey, cname string) (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
  cert := &x509.Certificate{
    SerialNumber: big.NewInt(1658),
    Subject: pkix.Name{
      CommonName: cname, 
    },
    IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
    NotBefore:    time.Now(),
    NotAfter:     time.Now().AddDate(10, 0, 0),
    SubjectKeyId: []byte{1, 2, 3, 4, 6},
    ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
    KeyUsage:     x509.KeyUsageDigitalSignature,
  }
  certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
  if err != nil {
    return nil, nil, nil, err
  }
  certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
  if err != nil {
    return nil, nil, nil, err
  }
  return cert, certBytes, certPrivKey, nil
}

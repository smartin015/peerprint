package config

import (
  "crypto/sha256"
  "crypto/subtle"
  pb "github.com/smartin015/peerprint/p2pgit/pkg/proto"
  "github.com/go-webauthn/webauthn/webauthn"
  "gopkg.in/yaml.v3"
  "crypto/rand"
  "fmt"
  "os"
)

type DriverConfig struct {
  WebAuthnId [64]byte `yaml:",flow"`
  Connections map[string]*pb.ConnectRequest
  AdminSalt [128]byte `yaml:",flow"`
  AdminPassHash [32]byte `yaml:",flow"`
  Credentials []webauthn.Credential
}

func New() *DriverConfig {
  d := &DriverConfig{
    Connections: make(map[string]*pb.ConnectRequest),
  }
  if err := d.SetPassword("changeme"); err != nil {
    panic(err)
  }
  return d
}

func (d *DriverConfig) Write(dest string) error {
  data, err := yaml.Marshal(d)
  if err != nil {
    return fmt.Errorf("marshal YAML: %w", err)
  }

  f, err := os.Create(dest)
  if err != nil {
    return err
  }
  defer f.Close()
  f.Write(data)
  return nil
}

func (d *DriverConfig) Read(src string) error {
  data, err := os.ReadFile(src)
  if err != nil {
    return err
  }
  if err := yaml.Unmarshal(data, d); err != nil {
    return fmt.Errorf("parse YAML: %w", err)
  }
  return nil
}


func (u *DriverConfig) WebAuthnID() []byte {
  for _, b := range u.WebAuthnId {
    if b != 0 {
      return u.WebAuthnId[:]
    }
  }
  rand.Read(u.WebAuthnId[:]) // Generate ID if unset
  return u.WebAuthnId[:]
}
func (u *DriverConfig) WebAuthnName() string {
  return "admin"
}
func (u *DriverConfig) WebAuthnDisplayName() string {
  return "Administrator"
}
func (u *DriverConfig) WebAuthnCredentials() []webauthn.Credential {
  return u.Credentials
}
func (u *DriverConfig) WebAuthnIcon() string {
  return ""
}

func (u *DriverConfig) PasswordMatches(pass string) bool {
  passwordHash := sha256.Sum256(append([]byte(pass), u.AdminSalt[:]...))
	passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], u.AdminPassHash[:]) == 1)
	return passwordMatch
}

func (u *DriverConfig) SetPassword(pass string) error {
  if _, err := rand.Read(u.AdminSalt[:]); err != nil {
    return err
  }
  hash := sha256.Sum256(append([]byte(pass), u.AdminSalt[:]...))
  u.AdminPassHash = hash
  return nil
}

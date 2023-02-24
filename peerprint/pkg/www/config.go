package www

import (
  "crypto/sha256"
  "crypto/subtle"
  "github.com/go-webauthn/webauthn/webauthn"
  "crypto/rand"
)

const (
  DefaultPassword = "changeme"
)

type WWWConfig struct {
  WebAuthnId [64]byte `yaml:",flow"`
  AdminSalt [128]byte `yaml:",flow"`
  AdminPassHash [32]byte `yaml:",flow"`
  Credentials []webauthn.Credential
}

func NewConfig() *WWWConfig {
  d := &WWWConfig{}
  if err := d.SetPassword(DefaultPassword); err != nil {
    panic(err)
  }
  return d
}

func (u *WWWConfig) WebAuthnID() []byte {
  for _, b := range u.WebAuthnId {
    if b != 0 {
      return u.WebAuthnId[:]
    }
  }
  rand.Read(u.WebAuthnId[:]) // Generate ID if unset
  return u.WebAuthnId[:]
}
func (u *WWWConfig) WebAuthnName() string {
  return "admin"
}
func (u *WWWConfig) WebAuthnDisplayName() string {
  return "Administrator"
}
func (u *WWWConfig) WebAuthnCredentials() []webauthn.Credential {
  return u.Credentials
}
func (u *WWWConfig) WebAuthnIcon() string {
  return ""
}

func (u *WWWConfig) PasswordMatches(pass string) bool {
  passwordHash := sha256.Sum256(append([]byte(pass), u.AdminSalt[:]...))
	passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], u.AdminPassHash[:]) == 1)
	return passwordMatch
}

func (u *WWWConfig) SetPassword(pass string) error {
  if _, err := rand.Read(u.AdminSalt[:]); err != nil {
    return err
  }
  hash := sha256.Sum256(append([]byte(pass), u.AdminSalt[:]...))
  u.AdminPassHash = hash
  return nil
}

package www

import (
  "testing"
  "github.com/go-webauthn/webauthn/webauthn"
)

func TestDefaultPassword(t *testing.T) {
  c := NewConfig()
  if !c.PasswordMatches(DefaultPassword) {
    t.Errorf("want PasswordMatches(DefaultPassword) = true; got false")
  }
}

func TestSetPassword(t *testing.T) {
  c := NewConfig()
  c.SetPassword("testpass")
  if c.PasswordMatches(DefaultPassword) {
    t.Errorf("still matches default password after change")
  }
  if !c.PasswordMatches("testpass") {
    t.Errorf("want PasswordMatches(testpass) = true; got false")
  }
}

func isZeroByteSlice(bb []byte) bool {
  for _, b := range bb {
    if b != 0 {
      return false
    }
  }
  return true
}

func TestWebAuthnInterface(t *testing.T) {
  c := NewConfig()
  if got := c.WebAuthnID(); isZeroByteSlice(got) {
    t.Errorf("got zero byte slice for ID")
  }
  if c.WebAuthnName() != "admin" {
    t.Errorf("want admin name")
  }
  if c.WebAuthnDisplayName() != "Administrator" {
    t.Errorf("want Administrator display name")
  }
  if len(c.WebAuthnCredentials()) != 0 {
    t.Errorf("somehow initialized with credentials; expecting none")
  }
  c.Credentials = append(c.Credentials, webauthn.Credential{})
  if len(c.WebAuthnCredentials()) != 1 {
    t.Errorf("expected single added credential")
  }
  if c.WebAuthnIcon() != "" {
    t.Errorf("Expected empty icon")
  }
}

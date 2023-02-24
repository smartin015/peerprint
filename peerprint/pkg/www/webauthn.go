package www

import (
  "github.com/go-webauthn/webauthn/protocol"
  "fmt"
  "net/http"
)

const (
	AuthCookieKey = "authenticated"
)

func (s *webserver) BeginRegistration(w http.ResponseWriter, r *http.Request) {
  options, session, err := s.w.BeginRegistration(s.cfg)
  if err != nil {
    ErrorResponse(w, err)
  } else {
    s.authSession = session
    JSONResponse(w, options)
  }
}

func (s *webserver) FinishRegistration(w http.ResponseWriter, r *http.Request) {
  response, err := protocol.ParseCredentialCreationResponseBody(r.Body)
  if err != nil {
    ErrorResponse(w, err)
    return
  }
  credential, err := s.w.CreateCredential(s.cfg, *s.authSession, response)
  if err != nil {
    ErrorResponse(w, err)
    return
  }
  if err := s.RegisterCredentials(credential); err != nil {
    ErrorResponse(w, err)
    return
  }
  s.l.Info("New credentials registered - ID %s", credential.Descriptor().CredentialID.String())
  JSONResponse(w, "OK")
}

func (s *webserver) setLoginCookie(w http.ResponseWriter, r *http.Request) error {
  session, err := s.cs.Get(r, "auth")
  if err != nil {
    return err
  }
  session.Values[AuthCookieKey] = true
  session.Save(r, w)
	s.l.Info("Saved login cookie")
  return nil
}

func (s *webserver) Logout(w http.ResponseWriter, r *http.Request) {
  session, err := s.cs.Get(r, "auth")
  if err != nil {
    ErrorResponse(w, err)
    return
  }
  session.Values[AuthCookieKey] = false
  session.Save(r, w)
  s.l.Info("Saved logout cookie")
  http.Redirect(w, r, "/login", 303) // 303 "see other"
}

func (s *webserver) BeginLogin(w http.ResponseWriter, r *http.Request) {
	// TODO prevent if not TLS

	pass := r.PostFormValue("password")
	if !s.cfg.PasswordMatches(pass) {
		http.Error(w, "Invalid Password", http.StatusUnauthorized)
		return
	}

  if len(s.cfg.Credentials) == 0 {
    if err := s.setLoginCookie(w, r); err != nil {
      ErrorResponse(w, err)
    } else {
      JSONResponse(w, "OK")
    }
  } else {
    options, session, err := s.w.BeginLogin(s.cfg)
    if err != nil {
      ErrorResponse(w, err)
      return
    } else {
      s.authSession = session
      JSONResponse(w, options)
    }
  }
}

func (s *webserver) FinishLogin(w http.ResponseWriter, r *http.Request) {
  _, err := s.w.FinishLogin(s.cfg, *s.authSession, r)
  if err != nil {
    ErrorResponse(w, err)
    return
  } else if err := s.setLoginCookie(w, r); err != nil {
    ErrorResponse(w, err)
    return
  }
  JSONResponse(w, "welcome")
}

func (s *webserver) WithAuth(next http.HandlerFunc) http.HandlerFunc {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    session, err := s.cs.Get(r, "auth")
    if err != nil {
      ErrorResponse(w, fmt.Errorf("Get session cookie: %w", err))
      return
    }
    if authed, ok := session.Values[AuthCookieKey].(bool); !ok || !authed {
			s.l.Warning("Redirect from auth - authed, ok = %v, %v", authed, ok)
      http.Redirect(w, r, "/login", 303) // 303 "see other"
      return
    }
    next.ServeHTTP(w, r)
  })
}

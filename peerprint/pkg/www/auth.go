package www

import (
  "net/http"
  "github.com/smartin015/peerprint/p2pgit/pkg/driver"
  "crypto/sha256"
  "crypto/subtle"
)

var (
  username="admin"
)

// https://www.alexedwards.net/blog/basic-authentication-in-go
func basicAuth(d *driver.Driver, next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Extract the username and password from the request 
    // Authorization header. If no Authentication header is present 
    // or the header value is invalid, then the 'ok' return value 
    // will be false.
		username, password, ok := r.BasicAuth()
		if ok {
      // Calculate SHA-256 hashes for the provided and expected
      // usernames and passwords.
      wantPassHash, salt := d.GetAdminPassAndSalt()
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256(append([]byte(password), salt...))
			expectedUsernameHash := sha256.Sum256([]byte(username))

      // Use the subtle.ConstantTimeCompare() function to check if 
      // the provided username and password hashes equal the  
      // expected username and password hashes. ConstantTimeCompare
      // will return 1 if the values are equal, or 0 otherwise. 
      // Importantly, we should to do the work to evaluate both the 
      // username and password before checking the return values to 
      // avoid leaking information.
			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], wantPassHash[:]) == 1)

      // If the username and password are correct, then call
      // the next handler in the chain. Make sure to return 
      // afterwards, so that none of the code below is run.
			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

    // If the Authentication header is not present, is invalid, or the
    // username or password is wrong, then set a WWW-Authenticate 
    // header to inform the client that we expect them to use basic
    // authentication and send a 401 Unauthorized response.
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}



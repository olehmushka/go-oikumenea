package transport

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Authenticator gates hermenea's API on the OIKUMENEA_HERMENEA_TOKEN shared secret (the trigger trust
// direction; D-Hermenea). /status and /debug pass through so health/readiness probes stay open. Fails
// CLOSED: a missing/empty configured token rejects every API request. Constant-time comparison.
type Authenticator struct{ token string }

// NewAuthenticator builds the middleware with the configured shared secret.
func NewAuthenticator(token string) *Authenticator { return &Authenticator{token: token} }

// Handle is the wrouter.RequestHandlerMiddleware.
func (a *Authenticator) Handle(rw http.ResponseWriter, r *http.Request, next http.Handler) {
	if strings.HasPrefix(r.URL.Path, "/status") || strings.HasPrefix(r.URL.Path, "/debug") {
		next.ServeHTTP(rw, r)
		return
	}
	raw := bearerToken(r)
	if a.token == "" || raw == "" || subtle.ConstantTimeCompare([]byte(raw), []byte(a.token)) != 1 {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusUnauthorized)
		_, _ = rw.Write([]byte(`{"errorCode":"CUSTOM_CLIENT","errorName":"Hermenea:Unauthorized","parameters":{}}`))
		return
	}
	next.ServeHTTP(rw, r)
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

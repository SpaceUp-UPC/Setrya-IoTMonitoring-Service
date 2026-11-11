package rest

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var (
	authNow    = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	authSecret = []byte("0123456789abcdef0123456789abcdef-test-secret")
)

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

// sign arma un JWT como los de IAM (jjwt): header.payload.firma en base64url.
func sign(alg string, newHash func() hash.Hash, secret []byte, claims string) string {
	signingInput := b64(fmt.Sprintf(`{"alg":%q}`, alg)) + "." + b64(claims)
	mac := hmac.New(newHash, secret)
	mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func claims(sub string, exp time.Time) string {
	return fmt.Sprintf(`{"sub":%q,"iat":%d,"exp":%d}`, sub, authNow.Unix(), exp.Unix())
}

func TestVerifyJWT(t *testing.T) {
	valid := claims("ana@example.com", authNow.Add(time.Hour))

	for alg, h := range map[string]func() hash.Hash{"HS256": sha256.New, "HS384": sha512.New384, "HS512": sha512.New} {
		sub, err := verifyJWT(sign(alg, h, authSecret, valid), authSecret, authNow)
		if err != nil || sub != "ana@example.com" {
			t.Errorf("%s: sub=%q err=%v", alg, sub, err)
		}
	}

	good := sign("HS256", sha256.New, authSecret, valid)
	bad := map[string]string{
		"wrong secret":   sign("HS256", sha256.New, []byte("another-secret-another-secret-123"), valid),
		"expired":        sign("HS256", sha256.New, authSecret, claims("ana@example.com", authNow.Add(-time.Second))),
		"no exp":         sign("HS256", sha256.New, authSecret, `{"sub":"ana@example.com"}`),
		"no sub":         sign("HS256", sha256.New, authSecret, fmt.Sprintf(`{"exp":%d}`, authNow.Add(time.Hour).Unix())),
		"not yet valid":  sign("HS256", sha256.New, authSecret, fmt.Sprintf(`{"sub":"a","exp":%d,"nbf":%d}`, authNow.Add(time.Hour).Unix(), authNow.Add(time.Minute).Unix())),
		"alg none":       b64(`{"alg":"none"}`) + "." + b64(valid) + ".",
		"unknown alg":    sign("RS256", sha256.New, authSecret, valid),
		"tampered claim": good[:len(b64(`{"alg":"HS256"}`))+1] + b64(claims("admin@example.com", authNow.Add(time.Hour))) + good[len(good)-43:],
		"two segments":   "a.b",
		"garbage":        "not-a-token",
		"empty":          "",
	}
	for name, token := range bad {
		if _, err := verifyJWT(token, authSecret, authNow); err == nil {
			t.Errorf("%s: token must be rejected", name)
		}
	}
}

func TestRequireJWT(t *testing.T) {
	var subject string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subject = SubjectFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	h := RequireJWT(authSecret, func() time.Time { return authNow }, next)
	token := sign("HS256", sha256.New, authSecret, claims("ana@example.com", authNow.Add(time.Hour)))

	cases := []struct {
		name, path, header string
		want               int
	}{
		{"health is public", "/health", "", http.StatusOK},
		{"openapi is public", "/openapi.yaml", "", http.StatusOK},
		{"api without token", "/api/v1/spaces/s/alerts", "", http.StatusUnauthorized},
		{"api with basic auth", "/api/v1/spaces/s/alerts", "Basic abc", http.StatusUnauthorized},
		{"api with invalid token", "/api/v1/spaces/s/alerts", "Bearer nope", http.StatusUnauthorized},
		{"api with valid token", "/api/v1/spaces/s/alerts", "Bearer " + token, http.StatusOK},
	}
	for _, c := range cases {
		subject = ""
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		if c.header != "" {
			req.Header.Set("Authorization", c.header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: got %d want %d", c.name, rec.Code, c.want)
		}
		if c.name == "api with valid token" && subject != "ana@example.com" {
			t.Errorf("subject = %q", subject)
		}
	}
}

func TestCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	preflight := func(h http.Handler, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/readings", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	any := CORS([]string{"*"}, next)
	if rec := preflight(any, "https://app.example"); rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("wildcard preflight: %d %v", rec.Code, rec.Header())
	}

	listed := CORS([]string{"https://setrya-frontend.vercel.app"}, next)
	if rec := preflight(listed, "https://setrya-frontend.vercel.app"); rec.Code != http.StatusNoContent ||
		rec.Header().Get("Access-Control-Allow-Origin") != "https://setrya-frontend.vercel.app" {
		t.Errorf("listed origin: %d %v", rec.Code, rec.Header())
	}
	// Un origen no permitido no recibe cabeceras CORS y la petición sigue su curso.
	if rec := preflight(listed, "https://evil.example"); rec.Header().Get("Access-Control-Allow-Origin") != "" || rec.Code != http.StatusTeapot {
		t.Errorf("unlisted origin: %d %v", rec.Code, rec.Header())
	}
}

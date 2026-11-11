package rest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"hash"
	"net/http"
	"strings"
	"time"
)

type subjectKey struct{}

// SubjectFromContext devuelve el usuario (claim sub) del token validado.
func SubjectFromContext(ctx context.Context) string {
	s, _ := ctx.Value(subjectKey{}).(string)
	return s
}

var errInvalidToken = errors.New("invalid token")

// hmacAlgorithms son los algoritmos que usa IAM: jjwt elige HS256, HS384 o
// HS512 según el largo del secreto. "none" y los asimétricos se rechazan.
var hmacAlgorithms = map[string]func() hash.Hash{
	"HS256": sha256.New,
	"HS384": sha512.New384,
	"HS512": sha512.New,
}

// verifyJWT valida la firma y la vigencia de un JWT firmado por IAM con el
// secreto compartido y devuelve su claim sub.
func verifyJWT(token string, secret []byte, now time.Time) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return "", errInvalidToken
	}
	newHash, ok := hmacAlgorithms[header.Alg]
	if !ok {
		return "", errInvalidToken
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", errInvalidToken
	}
	mac := hmac.New(newHash, secret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return "", errInvalidToken
	}

	var claims struct {
		Sub string   `json:"sub"`
		Exp *float64 `json:"exp"`
		Nbf *float64 `json:"nbf"`
	}
	if err := decodeSegment(parts[1], &claims); err != nil {
		return "", errInvalidToken
	}
	unix := float64(now.Unix())
	if claims.Exp == nil || unix >= *claims.Exp {
		return "", errInvalidToken
	}
	if claims.Nbf != nil && unix < *claims.Nbf {
		return "", errInvalidToken
	}
	if claims.Sub == "" {
		return "", errInvalidToken
	}
	return claims.Sub, nil
}

func decodeSegment(segment string, dst any) error {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

// RequireJWT exige un token Bearer de IAM en las rutas /api/. El resto
// (/health, /openapi.yaml) queda público.
func RequireJWT(secret []byte, now func() time.Time, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(w, http.StatusUnauthorized, ErrorResource{Error: "missing bearer token"})
			return
		}
		subject, err := verifyJWT(strings.TrimSpace(token), secret, now())
		if err != nil {
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			writeJSON(w, http.StatusUnauthorized, ErrorResource{Error: "invalid or expired token"})
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), subjectKey{}, subject)))
	})
}

// CORS permite que la aplicación web llame a la API desde otro origen.
// allowed es la lista de orígenes permitidos; "*" permite cualquiera (la API
// se autentica con el header Authorization, no con cookies).
func CORS(allowed []string, next http.Handler) http.Handler {
	any := false
	set := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		if o == "*" {
			any = true
		}
		set[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (any || set[origin]) {
			if any {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

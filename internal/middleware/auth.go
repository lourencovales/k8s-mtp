package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"

	"git.assilvestrar.club/lourenco/k8s-mtp/internal/config"
)

type AuthMiddleware struct {
	IssuerURL string `json:"issuer_url"`
	ClientID  string `json:"client_id"`
	Logger    *slog.Logger
	Enabled   bool `json:"enabled"`
	keySet    map[string]any
	mu        sync.RWMutex
}

type jwksResp struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type ctxKey struct{}

var userCtxKey = &ctxKey{}

func NewAuthMiddleware(cfg *config.Config, logger *slog.Logger) *AuthMiddleware {
	resp, err := http.Get(cfg.DexIssuerURL + "/.well-known/jwks.json")
	if err != nil {
		logger.Error("unable to get Dex jwks token")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		logger.Error("response from Dex wasn't 200")
		return nil
	}
	defer resp.Body.Close()

	var jwksR jwksResp
	err = json.NewDecoder(resp.Body).Decode(&jwksR)
	if err != nil {
		logger.Error("problem decoding json body of the JWT token response")
		return nil
	}

	a := &AuthMiddleware{
		IssuerURL: cfg.DexIssuerURL,
		ClientID:  cfg.DexClientID,
		Logger:    logger,
		Enabled:   cfg.AuthEnabled,
	}

	a.keySet = make(map[string]any)
	for _, key := range jwksR.Keys {
		pk, err := key.publicKey()
		if err != nil {
			continue
		}
		a.keySet[key.KID] = pk
	}

	return a
}

func (a *AuthMiddleware) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.Enabled {
			ctx := context.WithValue(r.Context(), userCtxKey, "anonymous")
			r = r.WithContext(ctx)
			next(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		tokenString, _ := strings.CutPrefix(token, "Bearer ")
		tokenParsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			a.mu.RLock()
			key := a.keySet[kid]
			a.mu.RUnlock()
			return key, nil
		}, jwt.WithIssuer(a.IssuerURL), jwt.WithAudience(a.ClientID))
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		claims, _ := tokenParsed.Claims.(jwt.MapClaims)
		sub, _ := claims.GetSubject()
		ctx := context.WithValue(r.Context(), userCtxKey, sub)
		r = r.WithContext(ctx)
		next(w, r)
	}
}

func UserFromContext(ctx context.Context) string {
	user, _ := ctx.Value(userCtxKey).(string)
	return user
}

func (j *jwk) publicKey() (any, error) {
	n, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, fmt.Errorf("unable to decode the rsa modulus")
	}
	e, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, fmt.Errorf("unable to decode the rsa exponent")
	}

	ei := 0
	for _, b := range e {
		ei = ei<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: ei}, nil
}

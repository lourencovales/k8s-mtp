package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateRSAKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key, &key.PublicKey
}

func signJWT(t *testing.T, key *rsa.PrivateKey, claims jwt.RegisteredClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-kid"
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestAuth_ValidToken(t *testing.T) {
	priv, pub := generateRSAKeys(t)
	claims := jwt.RegisteredClaims{
		Issuer:    "https://dex.test",
		Subject:   "testuser",
		Audience:  jwt.ClaimStrings{"k8s-mtp"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tokenStr := signJWT(t, priv, claims)

	auth := &AuthMiddleware{
		IssuerURL: "https://dex.test",
		ClientID:  "k8s-mtp",
		Enabled:   true,
		keySet:    map[string]any{"test-kid": pub},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		user := UserFromContext(r.Context())
		if user != "testuser" {
			t.Errorf("expected `testuser`, got %q", user)
		}
	}
	auth.Auth(next)(w, req)

	if !nextCalled {
		t.Error("next handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAuth_WrongIssuer(t *testing.T) {
	priv, pub := generateRSAKeys(t)
	claims := jwt.RegisteredClaims{
		Issuer:    "https://wrong.test",
		Subject:   "testuser",
		Audience:  jwt.ClaimStrings{"k8s-mtp"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tokenStr := signJWT(t, priv, claims)

	auth := &AuthMiddleware{
		IssuerURL: "https://dex.test",
		ClientID:  "k8s-mtp",
		Enabled:   true,
		keySet:    map[string]any{"test-kid": pub},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		user := UserFromContext(r.Context())
		if user != "testuser" {
			t.Errorf("expected `testuser`, got %q", user)
		}
	}
	auth.Auth(next)(w, req)

	if nextCalled {
		t.Error("next handler was called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	priv, pub := generateRSAKeys(t)
	claims := jwt.RegisteredClaims{
		Issuer:    "https://dex.test",
		Subject:   "testuser",
		Audience:  jwt.ClaimStrings{"k8s-mtp"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	}
	tokenStr := signJWT(t, priv, claims)

	auth := &AuthMiddleware{
		IssuerURL: "https://dex.test",
		ClientID:  "k8s-mtp",
		Enabled:   true,
		keySet:    map[string]any{"test-kid": pub},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	w := httptest.NewRecorder()

	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		user := UserFromContext(r.Context())
		if user != "testuser" {
			t.Errorf("expected `testuser`, got %q", user)
		}
	}
	auth.Auth(next)(w, req)

	if nextCalled {
		t.Error("next handler was called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_MissingToken(t *testing.T) {
	_, pub := generateRSAKeys(t)
	auth := &AuthMiddleware{
		IssuerURL: "https://dex.test",
		ClientID:  "k8s-mtp",
		Enabled:   true,
		keySet:    map[string]any{"test-kid": pub},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		user := UserFromContext(r.Context())
		if user != "testuser" {
			t.Errorf("expected `testuser`, got %q", user)
		}
	}
	auth.Auth(next)(w, req)

	if nextCalled {
		t.Error("next handler was called")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuth_Disabled(t *testing.T) {
	auth := &AuthMiddleware{
		Enabled: false,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	nextCalled := false
	next := func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		user := UserFromContext(r.Context())
		if user != "anonymous" {
			t.Errorf("expected `anonymous`, got %q", user)
		}
	}
	auth.Auth(next)(w, req)

	if !nextCalled {
		t.Error("next handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

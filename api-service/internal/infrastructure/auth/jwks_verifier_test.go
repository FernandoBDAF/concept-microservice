package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
)

func genRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func jwkFor(kid string, pub *rsa.PublicKey) jwksKey {
	return jwksKey{
		Kty: "RSA",
		Use: "sig",
		Alg: "RS256",
		Kid: kid,
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

// jwksTestServer serves a swappable key set and counts fetches.
type jwksTestServer struct {
	server *httptest.Server
	mu     sync.Mutex
	keys   []jwksKey
	hits   int32
}

func newJWKSTestServer(t *testing.T, keys ...jwksKey) *jwksTestServer {
	t.Helper()
	s := &jwksTestServer{keys: keys}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&s.hits, 1)
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		s.mu.Lock()
		doc := jwksDocument{Keys: s.keys}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *jwksTestServer) setKeys(keys ...jwksKey) {
	s.mu.Lock()
	s.keys = keys
	s.mu.Unlock()
}

func (s *jwksTestServer) hitCount() int32 { return atomic.LoadInt32(&s.hits) }

func newTestVerifier(t *testing.T, url string) *JWKSVerifier {
	t.Helper()
	return NewJWKSVerifier(config.AuthConfig{URL: url, Timeout: 2 * time.Second}, zap.NewNop())
}

func signAccessToken(t *testing.T, key *rsa.PrivateKey, kid string, mutate func(jwt.MapClaims)) string {
	t.Helper()
	claims := jwt.MapClaims{
		"userId":    "u1",
		"email":     "a@b.com",
		"role":      "user",
		"tokenType": "ACCESS_TOKEN",
		"jti":       "j1",
		"exp":       time.Now().Add(15 * time.Minute).Unix(),
		"iat":       time.Now().Unix(),
	}
	if mutate != nil {
		mutate(claims)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestJWKSVerifier_ValidTokenRoundTrip(t *testing.T) {
	key := genRSAKey(t)
	srv := newJWKSTestServer(t, jwkFor("kid-1", &key.PublicKey))
	v := newTestVerifier(t, srv.server.URL)

	// No eager fetch: the unknown kid on first use triggers the refresh.
	claims, err := v.Verify(context.Background(), signAccessToken(t, key, "kid-1", nil))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != "u1" || claims.Email != "a@b.com" || claims.Role != "user" {
		t.Errorf("claims not mapped: %+v", claims)
	}
	if srv.hitCount() != 1 {
		t.Errorf("expected exactly one JWKS fetch, got %d", srv.hitCount())
	}

	// Second verification uses the cache — no extra fetch.
	if _, err := v.Verify(context.Background(), signAccessToken(t, key, "kid-1", nil)); err != nil {
		t.Fatalf("cached Verify: %v", err)
	}
	if srv.hitCount() != 1 {
		t.Errorf("expected cached key to be used, got %d fetches", srv.hitCount())
	}
}

func TestJWKSVerifier_KidRotationTriggersRefresh(t *testing.T) {
	key1 := genRSAKey(t)
	key2 := genRSAKey(t)
	srv := newJWKSTestServer(t, jwkFor("kid-1", &key1.PublicKey))
	v := newTestVerifier(t, srv.server.URL)
	v.minInterval = 0 // rotation test: allow back-to-back refreshes

	if _, err := v.Verify(context.Background(), signAccessToken(t, key1, "kid-1", nil)); err != nil {
		t.Fatalf("Verify with kid-1: %v", err)
	}

	// Rotate: auth-service now publishes (and signs with) kid-2.
	srv.setKeys(jwkFor("kid-1", &key1.PublicKey), jwkFor("kid-2", &key2.PublicKey))

	claims, err := v.Verify(context.Background(), signAccessToken(t, key2, "kid-2", nil))
	if err != nil {
		t.Fatalf("Verify after rotation: %v", err)
	}
	if claims.UserID != "u1" {
		t.Errorf("claims not mapped after rotation")
	}
	if srv.hitCount() != 2 {
		t.Errorf("expected unknown kid to trigger exactly one extra fetch, got %d total", srv.hitCount())
	}
}

func TestJWKSVerifier_UnknownKidRefreshIsRateLimited(t *testing.T) {
	key1 := genRSAKey(t)
	rogue := genRSAKey(t)
	srv := newJWKSTestServer(t, jwkFor("kid-1", &key1.PublicKey))
	v := newTestVerifier(t, srv.server.URL) // default 30s min interval

	if _, err := v.Verify(context.Background(), signAccessToken(t, key1, "kid-1", nil)); err != nil {
		t.Fatalf("priming Verify: %v", err)
	}

	// A flood of tokens with a kid the JWKS does not contain must NOT
	// hammer auth-service: refresh already happened <30s ago.
	for i := 0; i < 5; i++ {
		_, err := v.Verify(context.Background(), signAccessToken(t, rogue, "kid-rogue", nil))
		if err == nil {
			t.Fatalf("expected verification failure for unknown kid")
		}
	}
	if srv.hitCount() != 1 {
		t.Errorf("expected refresh rate-limiting to hold fetches at 1, got %d", srv.hitCount())
	}
}

func TestJWKSVerifier_BadSignatureRejected(t *testing.T) {
	key := genRSAKey(t)
	imposter := genRSAKey(t)
	srv := newJWKSTestServer(t, jwkFor("kid-1", &key.PublicKey))
	v := newTestVerifier(t, srv.server.URL)

	// Signed by the wrong private key but claiming a known kid.
	if _, err := v.Verify(context.Background(), signAccessToken(t, imposter, "kid-1", nil)); err == nil {
		t.Fatalf("expected bad signature to be rejected")
	}
}

func TestJWKSVerifier_ExpiredTokenRejected(t *testing.T) {
	key := genRSAKey(t)
	srv := newJWKSTestServer(t, jwkFor("kid-1", &key.PublicKey))
	v := newTestVerifier(t, srv.server.URL)

	expired := signAccessToken(t, key, "kid-1", func(c jwt.MapClaims) {
		c["exp"] = time.Now().Add(-time.Minute).Unix()
	})
	if _, err := v.Verify(context.Background(), expired); err == nil {
		t.Fatalf("expected expired token to be rejected")
	}
}

func TestJWKSVerifier_RefreshTokenRejected(t *testing.T) {
	key := genRSAKey(t)
	srv := newJWKSTestServer(t, jwkFor("kid-1", &key.PublicKey))
	v := newTestVerifier(t, srv.server.URL)

	refreshTok := signAccessToken(t, key, "kid-1", func(c jwt.MapClaims) {
		c["tokenType"] = "REFRESH_TOKEN"
	})
	if _, err := v.Verify(context.Background(), refreshTok); !errors.Is(err, ErrInvalidTokenUse) {
		t.Fatalf("expected ErrInvalidTokenUse for refresh token, got %v", err)
	}
}

func TestJWKSVerifier_HS256Rejected(t *testing.T) {
	key := genRSAKey(t)
	srv := newJWKSTestServer(t, jwkFor("kid-1", &key.PublicKey))
	v := newTestVerifier(t, srv.server.URL)

	// During JWT_ALGORITHM rollout auth-service may still mint HS256; local
	// verification is RS256-only BY DESIGN (introspection is the fallback,
	// not an HS256 local verify).
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": "u1", "tokenType": "ACCESS_TOKEN",
		"exp": time.Now().Add(15 * time.Minute).Unix(),
	})
	tok.Header["kid"] = "kid-1"
	signed, err := tok.SignedString([]byte("shared-secret"))
	if err != nil {
		t.Fatalf("sign hs256: %v", err)
	}
	if _, err := v.Verify(context.Background(), signed); err == nil {
		t.Fatalf("expected HS256 token to be rejected by RS256-only verifier")
	}
}

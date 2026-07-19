package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/fernandobarroso/microservices/api-service/internal/config"
)

// JWKS local verification (ADR-009.1): auth-service signs RS256 with kid
// headers and serves /.well-known/jwks.json; api-service verifies tokens
// locally against a cached key set. Per-request HTTP introspection (the
// auth-service SPOF proven live twice) remains only behind
// API_AUTH_STRICT_INTROSPECTION=true.
//
// During the JWT_ALGORITHM rollout auth-service may still issue HS256
// tokens; those fail local verification by design (RS256 only here) — the
// introspection flag is the escape hatch, not an HS256 local verify.

const (
	jwksPath = "/.well-known/jwks.json"
	// minRefreshInterval rate-limits unknown-kid refreshes so a flood of
	// bad tokens cannot hammer auth-service (the whole point of ADR-009.1
	// is to stop depending on it per-request).
	minRefreshInterval = 30 * time.Second
	// backgroundRefreshInterval keeps the cache fresh for planned key
	// rotation without any unknown-kid misses.
	backgroundRefreshInterval = time.Hour
)

// Claims are the access-token claims auth-service's TokenService embeds
// (userId/email/role/jti/tokenType).
type Claims struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenType string `json:"tokenType"`
	jwt.RegisteredClaims
}

type jwksDocument struct {
	Keys []jwksKey `json:"keys"`
}

type jwksKey struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

var (
	ErrUnknownKeyID    = errors.New("token kid not present in cached JWKS")
	ErrInvalidTokenUse = errors.New("token is not an access token")
)

// JWKSVerifier verifies RS256 JWTs locally against a cached JWKS fetched
// from auth-service. Refresh triggers: startup (best-effort), hourly
// background, and on unknown kid (min-interval 30s, single-flight).
type JWKSVerifier struct {
	url        string
	httpClient *http.Client
	log        *zap.Logger

	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey

	refreshMu   sync.Mutex
	lastAttempt time.Time
	minInterval time.Duration // minRefreshInterval; overridable in tests
}

// NewJWKSVerifier derives the JWKS URL from the existing auth-service base
// URL config (API_AUTH_URL + /.well-known/jwks.json).
func NewJWKSVerifier(cfg config.AuthConfig, log *zap.Logger) *JWKSVerifier {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &JWKSVerifier{
		url:         strings.TrimRight(cfg.URL, "/") + jwksPath,
		httpClient:  &http.Client{Timeout: timeout},
		log:         log.Named("jwks_verifier"),
		keys:        map[string]*rsa.PublicKey{},
		minInterval: minRefreshInterval,
	}
}

// Start eagerly fetches the JWKS (non-fatal: auth-service may still be
// booting) and refreshes hourly until ctx is done. Run as a goroutine.
func (v *JWKSVerifier) Start(ctx context.Context) {
	if err := v.refresh(ctx, false); err != nil {
		v.log.Warn("initial JWKS fetch failed; will retry on demand", zap.Error(err))
	}

	ticker := time.NewTicker(backgroundRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := v.refresh(ctx, true); err != nil {
				v.log.Warn("background JWKS refresh failed", zap.Error(err))
			}
		}
	}
}

// refresh fetches and swaps the key set. Single-flight; unless force, it
// honors minRefreshInterval between ATTEMPTS (success or failure) so a
// degraded auth-service is not hammered by unknown-kid traffic.
func (v *JWKSVerifier) refresh(ctx context.Context, force bool) error {
	v.refreshMu.Lock()
	defer v.refreshMu.Unlock()

	if !force && time.Since(v.lastAttempt) < v.minInterval {
		return fmt.Errorf("jwks refresh rate-limited (min interval %s)", v.minInterval)
	}
	v.lastAttempt = time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return fmt.Errorf("failed to build jwks request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jwks fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("failed to decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := rsaKeyFromJWK(k)
		if err != nil {
			v.log.Warn("skipping unparseable JWK", zap.String("kid", k.Kid), zap.Error(err))
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return errors.New("jwks contained no usable RSA keys")
	}

	v.mu.Lock()
	v.keys = keys
	v.mu.Unlock()

	v.log.Info("JWKS refreshed", zap.Int("keys", len(keys)))
	return nil
}

func rsaKeyFromJWK(k jwksKey) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("bad modulus: %w", err)
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("bad exponent: %w", err)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nb),
		E: int(new(big.Int).SetBytes(eb).Int64()),
	}, nil
}

func (v *JWKSVerifier) keyFor(kid string) (*rsa.PublicKey, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	k, ok := v.keys[kid]
	return k, ok
}

// Verify validates an RS256 access token locally and returns its claims.
// Unknown kid triggers one rate-limited JWKS refresh (key rotation path).
func (v *JWKSVerifier) Verify(ctx context.Context, tokenString string) (*Claims, error) {
	claims := &Claims{}
	keyfunc := func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token has no kid header")
		}
		if key, ok := v.keyFor(kid); ok {
			return key, nil
		}
		// Unknown kid: likely rotation — refresh once (rate-limited).
		if err := v.refresh(ctx, false); err != nil {
			v.log.Debug("unknown-kid JWKS refresh not performed", zap.Error(err))
		}
		if key, ok := v.keyFor(kid); ok {
			return key, nil
		}
		return nil, fmt.Errorf("%w: %q", ErrUnknownKeyID, kid)
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, keyfunc,
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenType != "ACCESS_TOKEN" {
		return nil, ErrInvalidTokenUse
	}
	return claims, nil
}

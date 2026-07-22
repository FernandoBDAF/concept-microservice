package httpapi

import (
	"context"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"example.com/api-publisher/internal/auth"
)

// ctxKey is an unexported context key type so values set here cannot collide
// with anything else in the request context.
type ctxKey string

const (
	userIDKey   ctxKey = "user_id"
	userRoleKey ctxKey = "user_role"
)

// UserID returns the authenticated user id set by LocalAuth, or "".
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

// UserRole returns the authenticated user role set by LocalAuth, or "".
func UserRole(ctx context.Context) string {
	v, _ := ctx.Value(userRoleKey).(string)
	return v
}

// bearerToken extracts the Bearer token from the Authorization header,
// writing the 401 itself when the header is missing or malformed.
func bearerToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeError(w, http.StatusUnauthorized, "authorization header is required")
		return "", false
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		writeError(w, http.StatusUnauthorized, "invalid authorization header format")
		return "", false
	}
	return parts[1], true
}

// DevAuthBypass skips all token verification and stamps a fixed dev user id
// on every request. It exists ONLY so the bootstrap smoke can run without an
// auth service (config.AuthConfig.Disabled / AUTH_DISABLED=true). Never use
// it anywhere real — it authenticates nobody.
func DevAuthBypass(log *zap.Logger) func(http.Handler) http.Handler {
	log.Warn("AUTH DISABLED — every request is treated as authenticated (dev/bootstrap only)")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), userIDKey, "dev-user")
			ctx = context.WithValue(ctx, userRoleKey, "dev")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LocalAuth validates RS256 JWTs locally against the cached JWKS. This is the
// DEFAULT (and only) auth path in this template: no per-request hop to the
// auth service. On success it puts the user id and role in the request
// context (read them with UserID / UserRole).
//
// Seam: if you need revocation-strict checks or must accept a legacy token
// algorithm during a migration, wrap or replace this with an HTTP
// introspection call to your auth service (see CONTRACTS.md §3).
func LocalAuth(verifier *auth.JWKSVerifier, log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(w, r)
			if !ok {
				return
			}

			claims, err := verifier.Verify(r.Context(), token)
			if err != nil {
				log.Warn("local token verification failed", zap.Error(err))
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userRoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

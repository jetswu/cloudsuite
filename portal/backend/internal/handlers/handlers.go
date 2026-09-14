package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog/log"
)

// ctxKey is an unexported type for context keys.
type ctxKey string

const claimsKey ctxKey = "claims"

// Claims extracted from verified ID token.
type Claims struct {
	Sub               string   `json:"sub"`
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
}

// Handlers holds shared dependencies.
type Handlers struct {
	verifier *oidc.IDTokenVerifier
}

// New creates a Handlers with the given OIDC verifier.
func New(verifier *oidc.IDTokenVerifier) *Handlers {
	return &Handlers{verifier: verifier}
}

// Health responds GET /api/health.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Me responds GET /api/me with the caller's verified claims.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(claimsKey).(*Claims)
	if !ok || claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, claims)
}

// RequireAuth verifies the Bearer JWT in the Authorization header and
// injects verified claims into the request context.
func (h *Handlers) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		raw, ok := strings.CutPrefix(authz, "Bearer ")
		if !ok || raw == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}

		idToken, err := h.verifier.Verify(r.Context(), raw)
		if err != nil {
			log.Debug().Err(err).Msg("token verify failed")
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			return
		}

		claims := &Claims{}
		if err := idToken.Claims(claims); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "failed to parse claims"})
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("write json")
	}
}

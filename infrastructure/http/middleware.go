package httpinfra

import (
	"context"
	"net/http"
	"strings"

	domain "github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const tokenContextKey contextKey = "token"

// BearerAuthMiddleware validates the Authorization: Bearer <token> header.
func BearerAuthMiddleware(tokens domain.TokenRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				jsonError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			plaintext := strings.TrimPrefix(auth, "Bearer ")

			all, err := tokens.FindAll(r.Context())
			if err != nil {
				jsonError(w, http.StatusInternalServerError, "token lookup failed")
				return
			}

			var matched *domain.Token
			for i := range all {
				t := &all[i]
				if t.Revoked {
					continue
				}
				if bcrypt.CompareHashAndPassword([]byte(t.Hash), []byte(plaintext)) == nil {
					matched = t
					break
				}
			}
			if matched == nil {
				jsonError(w, http.StatusUnauthorized, "invalid or revoked token")
				return
			}

			ctx := context.WithValue(r.Context(), tokenContextKey, matched)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jdholdren/seymour/internal/seymour"
)

type ctxKey int

const userIDCtxKey ctxKey = iota

// sessionUserID reads and decodes the session cookie, returning the user ID
// and whether a valid session was present.
func (s Server) sessionUserID(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}

	var sess session
	if err := s.secureCookie.Decode(sessionCookie, cookie.Value, &sess); err != nil {
		return "", false
	}

	return sess.UserID, true
}

// requireAuth rejects requests without a valid session, otherwise attaches
// the session's user ID to the request context.
func (s Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := s.sessionUserID(r)
		if !ok {
			sErr := seymour.E("unauthorized", http.StatusUnauthorized)
			if err := writeJSON(w, sErr.Status, sErr); err != nil {
				slog.Error("error writing response", "error", err)
			}
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDCtxKey, userID)))
	})
}

// userIDFromContext returns the user ID that requireAuth attached to the request context.
func userIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(userIDCtxKey).(string)
	return id
}

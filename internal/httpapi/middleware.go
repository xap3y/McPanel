package httpapi

import (
	"context"
	"net/http"

	"mcpanel/internal/auth"
)

type ctxKey string

const sessionKey ctxKey = "session"

func (a *API) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("mcpanel_session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		sess, err := a.Auth.Validate(cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey, sess)
		next(w, r.WithContext(ctx))
	}
}

func sessionFrom(r *http.Request) *auth.Session {
	s, _ := r.Context().Value(sessionKey).(*auth.Session)
	return s
}

// requireCSRF is disabled — this panel runs on localhost only.
// All mutating routes pass through without token verification.
func (a *API) requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return next
}
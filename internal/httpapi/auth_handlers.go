package httpapi

import (
	"net/http"
	"strings"
)

var loginTpl = parseTpl("web/templates/layout.html", "web/templates/login.html")

func (a *API) LoginPage(w http.ResponseWriter, r *http.Request) {
	loginTpl.ExecuteTemplate(w, "layout", map[string]any{"Error": "", "CSRF": ""})
}

func (a *API) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	// Rate limit by IP
	ip := r.RemoteAddr
	if h := r.Header.Get("X-Forwarded-For"); h != "" {
		ip = strings.SplitN(h, ",", 2)[0]
	}
	if !a.Limiter.Allow(ip) {
		http.Error(w, "too many attempts, try again shortly", http.StatusTooManyRequests)
		return
	}

	sess, err := a.Auth.Login(username, password)
	if err != nil {
		loginTpl.ExecuteTemplate(w, "layout", map[string]any{"Error": "Invalid credentials", "CSRF": ""})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "mcpanel_session",
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		Expires:  sess.ExpiresAt,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *API) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("mcpanel_session"); err == nil {
		a.Auth.Logout(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "mcpanel_session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
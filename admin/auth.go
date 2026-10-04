package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"

	"uiserver/admin/components"
)

const sessionCookie = "admin_session"

// User identifies the signed-in operator. ID and Role are only populated
// when RBAC is active (Config.DB set) — in the simple/no-DB mode, Role is
// the zero value and every permission check short-circuits to "allowed"
// (see App.hasPermission).
type User struct {
	ID       int
	Username string
	Role     Role
}

// Authenticator checks credentials. SimpleAuthenticator below is a demo
// implementation; swap in one backed by a real user store for production use.
type Authenticator interface {
	Authenticate(username, password string) (*User, error)
}

// SimpleAuthenticator is a demo Authenticator backed by a fixed
// username->password map.
type SimpleAuthenticator map[string]string

func (a SimpleAuthenticator) Authenticate(username, password string) (*User, error) {
	if want, ok := a[username]; ok && want == password {
		return &User{Username: username}, nil
	}
	return nil, fmt.Errorf("invalid username or password")
}

// sessionStore is a mutex-guarded in-memory token->User map. Sessions are
// lost on restart; this is demo-grade, not a production session store.
type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]*User
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]*User)}
}

func (s *sessionStore) create(u *User) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = u
	return token, nil
}

func (s *sessionStore) lookup(token string) *User {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[token]
}

func (s *sessionStore) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

type userCtxKey struct{}

// CurrentUser returns the signed-in User for this request, or nil if none.
func CurrentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userCtxKey{}).(*User)
	return u
}

// loadSession attaches the signed-in User (if any) to the request context.
func (a *App) loadSession(r *http.Request) *http.Request {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return r
	}
	user := a.sessions.lookup(cookie.Value)
	if user == nil {
		return r
	}
	return r.WithContext(context.WithValue(r.Context(), userCtxKey{}, user))
}

// requireAuth redirects unauthenticated requests to /login: a plain redirect
// for normal navigation, an HX-Redirect header for htmx requests (so the
// browser performs the redirect instead of htmx swapping the login page
// into the current content target).
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if CurrentUser(r) == nil {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusOK)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// authenticate verifies credentials, preferring the DB-backed Users table
// (and its Role) when RBAC is active; Config.Authenticator is only used in
// the simple/no-DB mode, since roles have to live in the same database the
// Configurator and resource data already do.
func (a *App) authenticate(username, password string) (*User, error) {
	if a.rbac != nil {
		return a.rbac.verifyPassword(username, password)
	}
	return a.auth.Authenticate(username, password)
}

func (a *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if CurrentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.writeHTML(w, components.LoginPage(a.title, ""))
}

func (a *App) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := a.authenticate(username, password)
	if err != nil {
		a.writeHTML(w, components.LoginPage(a.title, "Invalid username or password."))
		return
	}

	token, err := a.sessions.create(user)
	if err != nil {
		a.writeHTML(w, components.LoginPage(a.title, "Could not start session, please try again."))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("HX-Redirect", "/")
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		a.sessions.delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("HX-Redirect", "/login")
	w.WriteHeader(http.StatusOK)
}

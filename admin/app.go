// Package admin is a small, generic admin-tool engine built on elem-go and
// htmx. Register Resources (a Field list + a DataProvider) to get a full
// Data/Create/Edit CRUD UI, and GenericPages for anything else; App wires
// routing, auth, and the master-page/content-area composition.
package admin

import (
	"net/http"

	"github.com/chasefleming/elem-go"

	"uiserver/admin/components"
)

// Config configures a new App.
type Config struct {
	Title         string
	Authenticator Authenticator
}

type navEntry struct {
	key, label, href string
}

// App is the admin engine: a route registry (built from registered
// Resources and GenericPages) plus the auth/session plumbing shared by all
// of them.
type App struct {
	mux       *http.ServeMux
	title     string
	auth      Authenticator
	sessions  *sessionStore
	resources map[string]*Resource
	navOrder  []navEntry
}

// New creates an App. Call RegisterResource/RegisterPage to add content,
// then serve app.Handler().
func New(cfg Config) *App {
	a := &App{
		mux:       http.NewServeMux(),
		title:     cfg.Title,
		auth:      cfg.Authenticator,
		sessions:  newSessionStore(),
		resources: make(map[string]*Resource),
	}

	a.mux.HandleFunc("GET /", a.handleRoot)
	a.mux.HandleFunc("GET /login", a.handleLoginPage)
	a.mux.HandleFunc("POST /login", a.handleLoginSubmit)
	a.mux.HandleFunc("POST /logout", a.handleLogout)

	return a
}

// RegisterPage adds a blank-canvas GenericPage to the nav and routes it at
// GET /page/{key}.
func (a *App) RegisterPage(p *GenericPage) {
	href := "/page/" + p.Key
	a.navOrder = append(a.navOrder, navEntry{key: p.Key, label: p.Label, href: href})

	a.mux.HandleFunc("GET "+href, requireAuth(func(w http.ResponseWriter, r *http.Request) {
		a.render(w, r, p.Key, p.Render(r))
	}))
}

// RegisterResource adds a Resource's full Data/Create/Edit CRUD UI to the
// nav and routes it under /r/{key}/...
func (a *App) RegisterResource(res *Resource) {
	a.resources[res.Key] = res
	href := "/r/" + res.Key
	a.navOrder = append(a.navOrder, navEntry{key: res.Key, label: res.Label, href: href})

	a.mux.HandleFunc("GET "+href, requireAuth(a.handleList(res)))
	a.mux.HandleFunc("GET "+href+"/new", requireAuth(a.handleNew(res)))
	a.mux.HandleFunc("POST "+href, requireAuth(a.handleCreate(res)))
	a.mux.HandleFunc("GET "+href+"/{id}/edit", requireAuth(a.handleEdit(res)))
	a.mux.HandleFunc("PUT "+href+"/{id}", requireAuth(a.handleUpdate(res)))
	a.mux.HandleFunc("DELETE "+href+"/{id}", requireAuth(a.handleDelete(res)))
}

// Handler returns the App as an http.Handler, ready for
// http.ListenAndServe.
func (a *App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mux.ServeHTTP(w, a.loadSession(r))
	})
}

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	if len(a.navOrder) == 0 {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, a.navOrder[0].href, http.StatusSeeOther)
}

// render writes content as a bare htmx fragment when the request came from
// htmx (HX-Request: true — a nav click, pagination, a form submit, ...),
// or wraps it in the full MasterPage shell for direct navigation/refreshes.
func (a *App) render(w http.ResponseWriter, r *http.Request, activeKey string, content elem.Node) {
	if r.Header.Get("HX-Request") == "true" {
		a.writeHTML(w, content)
		return
	}
	a.writeHTML(w, components.MasterPage(a.title, a.navItems(), activeKey, usernameOf(CurrentUser(r)), content))
}

func (a *App) writeHTML(w http.ResponseWriter, node elem.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(node.Render()))
}

func (a *App) navItems() []components.NavItem {
	items := make([]components.NavItem, 0, len(a.navOrder))
	for _, n := range a.navOrder {
		items = append(items, components.NavItem{Key: n.key, Label: n.label, Href: n.href})
	}
	return items
}

func usernameOf(u *User) string {
	if u == nil {
		return ""
	}
	return u.Username
}

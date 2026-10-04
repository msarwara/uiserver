// Package admin is a small, generic admin-tool engine built on elem-go and
// htmx. Register Resources (a Field list + a DataProvider) to get a full
// Data/Create/Edit CRUD UI, and GenericPages for anything else; App wires
// routing, auth, and the master-page/content-area composition. When
// Config.DB is set, resource schemas persist in SQLite and a built-in
// Configurator page lets fields/resources be edited at runtime.
package admin

import (
	"database/sql"
	"fmt"
	"net/http"
	"sync"

	"github.com/chasefleming/elem-go"

	"uiserver/admin/components"
)

// Config configures a new App.
type Config struct {
	Title         string
	Authenticator Authenticator
	// DB enables schema/data persistence: resources with a nil Provider get
	// an auto-provisioned SQLProvider table, resource definitions persist
	// across restarts, and the Configurator page is registered. Leaving it
	// nil keeps the simplest in-process (non-persistent) behavior.
	DB *sql.DB
}

type navEntry struct {
	key, label, href string
}

// App is the admin engine: a route registry (built from registered
// Resources and GenericPages) plus the auth/session plumbing shared by all
// of them. Resources can be added after Handler() starts serving (the
// Configurator does exactly this), so the registry is mutex-guarded and
// routes look resources up by key on every request rather than being
// registered per-resource.
type App struct {
	mux      *http.ServeMux
	title    string
	auth     Authenticator
	sessions *sessionStore
	db       *sql.DB
	schema   *schemaStore
	rbac     *rbacStore

	mu        sync.RWMutex
	resources map[string]*Resource
	navOrder  []navEntry
}

// New creates an App. Call RegisterResource/RegisterPage to add content,
// then serve app.Handler().
func New(cfg Config) (*App, error) {
	a := &App{
		mux:       http.NewServeMux(),
		title:     cfg.Title,
		auth:      cfg.Authenticator,
		sessions:  newSessionStore(),
		db:        cfg.DB,
		resources: make(map[string]*Resource),
	}

	if cfg.DB != nil {
		store, err := newSchemaStore(cfg.DB)
		if err != nil {
			return nil, fmt.Errorf("admin: initializing schema store: %w", err)
		}
		a.schema = store

		rbac, err := newRBACStore(cfg.DB)
		if err != nil {
			return nil, fmt.Errorf("admin: initializing RBAC store: %w", err)
		}
		a.rbac = rbac
	}

	a.mux.HandleFunc("GET /", a.handleRoot)
	a.mux.HandleFunc("GET /login", a.handleLoginPage)
	a.mux.HandleFunc("POST /login", a.handleLoginSubmit)
	a.mux.HandleFunc("POST /logout", a.handleLogout)

	a.mux.HandleFunc("GET /r/{resource}", requireAuth(a.requirePermission(ActionView, a.handleList)))
	a.mux.HandleFunc("GET /r/{resource}/new", requireAuth(a.requirePermission(ActionCreate, a.handleNew)))
	a.mux.HandleFunc("POST /r/{resource}", requireAuth(a.requirePermission(ActionCreate, a.handleCreate)))
	a.mux.HandleFunc("GET /r/{resource}/{id}/edit", requireAuth(a.requirePermission(ActionEdit, a.handleEdit)))
	a.mux.HandleFunc("PUT /r/{resource}/{id}", requireAuth(a.requirePermission(ActionEdit, a.handleUpdate)))
	a.mux.HandleFunc("DELETE /r/{resource}/{id}", requireAuth(a.requirePermission(ActionDelete, a.handleDelete)))

	if cfg.DB != nil {
		a.registerConfigurator()
		a.registerUsersAndPermissions()
		a.registerAPI()
	}

	return a, nil
}

// RegisterPage adds a blank-canvas GenericPage to the nav and routes it at
// GET /page/{key}.
func (a *App) RegisterPage(p *GenericPage) {
	href := "/page/" + p.Key

	a.mu.Lock()
	a.navOrder = append(a.navOrder, navEntry{key: p.Key, label: p.Label, href: href})
	a.mu.Unlock()

	a.mux.HandleFunc("GET "+href, requireAuth(func(w http.ResponseWriter, r *http.Request) {
		a.render(w, r, p.Key, p.Render(r))
	}))
}

// RegisterResource adds a Resource's full Data/Create/Edit CRUD UI to the
// nav, under /r/{key}/... (routes are generic — see New — so this never
// touches the mux). If a DB is configured: the first time a given key is
// registered, res's Fields/Label/PageSize/Seed seed the schema store and
// (when Provider is nil) a new table; on every later registration of the
// same key, the persisted definition is loaded and overrides what res
// specifies — a Configurator edit outlives the process, a Go-code edit
// after that point does not.
func (a *App) RegisterResource(res *Resource) error {
	if err := validIdentifier(res.Key); err != nil {
		return fmt.Errorf("resource key: %w", err)
	}
	for _, f := range res.Fields {
		if f.Name == "id" {
			continue // the implicit primary key; valid to list for display, not a new column
		}
		if err := validIdentifier(f.Name); err != nil {
			return fmt.Errorf("field %q: %w", f.Name, err)
		}
	}

	if a.schema != nil {
		persisted, ok, err := a.schema.get(res.Key)
		if err != nil {
			return fmt.Errorf("admin: loading persisted resource %q: %w", res.Key, err)
		}
		if ok {
			res.Label = persisted.Label
			res.Fields = persisted.Fields
			res.PageSize = persisted.PageSize
		} else if err := a.firstTimeProvision(res); err != nil {
			return fmt.Errorf("admin: provisioning resource %q: %w", res.Key, err)
		}
	}

	if res.Provider == nil {
		if a.db == nil {
			return fmt.Errorf("admin: resource %q has no Provider and no DB is configured", res.Key)
		}
		res.Provider = NewSQLProvider(a.db, res.Key)
	}

	a.addResource(res)
	return nil
}

// firstTimeProvision creates res's table (when it has no custom Provider),
// seeds it, and persists its definition — all in one transaction, so a
// failure partway through leaves neither the table nor the schema store
// changed.
func (a *App) firstTimeProvision(res *Resource) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if res.Provider == nil {
		if err := createResourceTable(tx, res.Key, res.Fields); err != nil {
			return err
		}
		if err := insertSeedRows(tx, res.Key, res.Seed); err != nil {
			return err
		}
	}

	position, err := a.schema.nextPosition(tx)
	if err != nil {
		return err
	}
	if err := a.schema.upsertResource(tx, res, position); err != nil {
		return err
	}

	return tx.Commit()
}

func (a *App) addResource(res *Resource) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.resources[res.Key] = res
	a.navOrder = append(a.navOrder, navEntry{key: res.Key, label: res.Label, href: "/r/" + res.Key})
}

// removeResource drops a resource from the registry and nav. Its table and
// persisted schema rows are the caller's responsibility (see
// App.deleteResource in configurator.go) — this only updates the
// in-process routing/nav state.
func (a *App) removeResource(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.resources, key)
	for i, n := range a.navOrder {
		if n.key == key {
			a.navOrder = append(a.navOrder[:i], a.navOrder[i+1:]...)
			break
		}
	}
}

func (a *App) getResource(key string) (*Resource, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	res, ok := a.resources[key]
	return res, ok
}

// allResources returns every registered resource, regardless of Provider,
// in nav order — used by the permission matrix, since view/create/edit/
// delete grants apply to a resource's data no matter what backs it.
func (a *App) allResources() []*Resource {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]*Resource, 0, len(a.navOrder))
	for _, n := range a.navOrder {
		if res, ok := a.resources[n.key]; ok {
			out = append(out, res)
		}
	}
	return out
}

// sqlResources returns every registered resource backed by the
// auto-provisioned SQLProvider — the ones the Configurator can structurally
// edit (see package doc) — in nav order.
func (a *App) sqlResources() []*Resource {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]*Resource, 0, len(a.navOrder))
	for _, n := range a.navOrder {
		if res, ok := a.resources[n.key]; ok {
			if _, isSQL := res.Provider.(*SQLProvider); isSQL {
				out = append(out, res)
			}
		}
	}
	return out
}

// Handler returns the App as an http.Handler, ready for
// http.ListenAndServe.
func (a *App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mux.ServeHTTP(w, a.loadSession(r))
	})
}

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	var first string
	if len(a.navOrder) > 0 {
		first = a.navOrder[0].href
	}
	a.mu.RUnlock()

	if first == "" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, first, http.StatusSeeOther)
}

// render writes content as a bare htmx fragment when the request came from
// htmx (HX-Request: true — a nav click, pagination, a form submit, ...),
// or wraps it in the full MasterPage shell for direct navigation/refreshes.
func (a *App) render(w http.ResponseWriter, r *http.Request, activeKey string, content elem.Node) {
	if r.Header.Get("HX-Request") == "true" {
		a.writeHTML(w, content)
		return
	}
	user := CurrentUser(r)
	a.writeHTML(w, components.MasterPage(a.title, a.navItems(user), activeKey, usernameOf(user), content))
}

func (a *App) writeHTML(w http.ResponseWriter, node elem.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(node.Render()))
}

func (a *App) writeHTMLStatus(w http.ResponseWriter, status int, node elem.Node) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(node.Render()))
}

// navItems lists the nav entries visible to u: every GenericPage, every
// resource u can at least ActionView, the Configurator (if
// canAccessConfigurator) and Users & Permissions (if canManageUsers). With
// no DB (RBAC inactive), hasPermission always returns true, so nothing is
// filtered — today's open-nav behavior.
func (a *App) navItems(u *User) []components.NavItem {
	a.mu.RLock()
	entries := make([]navEntry, len(a.navOrder))
	copy(entries, a.navOrder)
	a.mu.RUnlock()

	items := make([]components.NavItem, 0, len(entries))
	for _, n := range entries {
		switch n.key {
		case configuratorKey:
			if !a.canAccessConfigurator(u) {
				continue
			}
		case usersPermissionsKey:
			if !a.canManageUsers(u) {
				continue
			}
		default:
			if _, isResource := a.getResource(n.key); isResource && !a.hasPermission(u, n.key, ActionView) {
				continue
			}
		}
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

package admin

import (
	"net/http"

	"uiserver/admin/components"
)

// Action is one of the four things a Resource's data can be used for; the
// permission grants (role-level and per-user override) are keyed by
// resourceKey + Action.
type Action string

const (
	ActionView   Action = "view"
	ActionCreate Action = "create"
	ActionEdit   Action = "edit"
	ActionDelete Action = "delete"
)

// Actions lists every action, in the order the permission matrix UI shows
// its columns.
var Actions = []Action{ActionView, ActionCreate, ActionEdit, ActionDelete}

func (a *App) canAccessConfigurator(u *User) bool {
	return u != nil && (u.Role.IsSystemAdmin || u.Role.CanAccessConfigurator)
}

func (a *App) canManageUsers(u *User) bool {
	return u != nil && (u.Role.IsSystemAdmin || u.Role.CanManageUsers)
}

// hasPermission reports whether u may perform action on resourceKey. With
// no DB configured, RBAC is inactive and everything is allowed — today's
// simple/no-DB behavior. An explicit per-user override (grant or deny)
// always beats the user's role; absent an override, the role's grant
// decides.
func (a *App) hasPermission(u *User, resourceKey string, action Action) bool {
	if a.rbac == nil {
		return true
	}
	if u == nil {
		return false
	}
	if u.Role.IsSystemAdmin {
		return true
	}
	if allowed, ok := a.rbac.userOverride(u.ID, resourceKey, action); ok {
		return allowed
	}
	return a.rbac.roleGrant(u.Role.ID, resourceKey, action)
}

// requirePermission wraps a resource/API handler so it 403s unless the
// current user may perform action on the request's {resource} path value.
func (a *App) requirePermission(action Action, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("resource")
		if !a.hasPermission(CurrentUser(r), key, action) {
			a.renderForbidden(w, r)
			return
		}
		next(w, r)
	}
}

// requireCanManageUsers wraps a Users & Permissions handler so it 403s
// unless the current user's role can manage users.
func (a *App) requireCanManageUsers(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.canManageUsers(CurrentUser(r)) {
			a.renderForbidden(w, r)
			return
		}
		next(w, r)
	}
}

func (a *App) renderForbidden(w http.ResponseWriter, r *http.Request) {
	content := components.Card(components.Flash("error", "You don't have permission to do that."))
	user := CurrentUser(r)

	if r.Header.Get("HX-Request") == "true" {
		a.writeHTMLStatus(w, http.StatusForbidden, content)
		return
	}
	a.writeHTMLStatus(w, http.StatusForbidden,
		components.MasterPage(a.title, a.navItems(user), "", usernameOf(user), content))
}

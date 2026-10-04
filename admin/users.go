package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"uiserver/admin/components"
)

const usersPermissionsKey = "users-permissions"

// registerUsersAndPermissions adds the built-in "Users & Permissions" nav
// entry and routes: Roles CRUD + per-role permission grants, Users CRUD +
// per-user permission overrides, and API token issuance. Every route is
// gated on canManageUsers, not just requireAuth.
func (a *App) registerUsersAndPermissions() {
	a.mu.Lock()
	a.navOrder = append(a.navOrder, navEntry{key: usersPermissionsKey, label: "Users & Permissions", href: "/admin-users"})
	a.mu.Unlock()

	wrap := a.requireCanManageUsers

	a.mux.HandleFunc("GET /admin-users", requireAuth(wrap(a.handleUsersList)))
	a.mux.HandleFunc("GET /admin-users/new", requireAuth(wrap(a.handleUserNewForm)))
	a.mux.HandleFunc("POST /admin-users/new", requireAuth(wrap(a.handleUserCreate)))
	a.mux.HandleFunc("GET /admin-users/{id}/edit", requireAuth(wrap(a.handleUserEditForm)))
	a.mux.HandleFunc("PUT /admin-users/{id}", requireAuth(wrap(a.handleUserUpdate)))
	a.mux.HandleFunc("DELETE /admin-users/{id}", requireAuth(wrap(a.handleUserDelete)))
	a.mux.HandleFunc("GET /admin-users/{id}/permissions", requireAuth(wrap(a.handleUserPermissionsForm)))
	a.mux.HandleFunc("POST /admin-users/{id}/permissions", requireAuth(wrap(a.handleUserPermissionsSave)))
	a.mux.HandleFunc("GET /admin-users/{id}/tokens", requireAuth(wrap(a.handleTokensList)))
	a.mux.HandleFunc("POST /admin-users/{id}/tokens", requireAuth(wrap(a.handleTokenCreate)))
	a.mux.HandleFunc("DELETE /admin-users/{id}/tokens/{tokenID}", requireAuth(wrap(a.handleTokenRevoke)))

	a.mux.HandleFunc("GET /admin-roles", requireAuth(wrap(a.handleRolesList)))
	a.mux.HandleFunc("GET /admin-roles/new", requireAuth(wrap(a.handleRoleNewForm)))
	a.mux.HandleFunc("POST /admin-roles/new", requireAuth(wrap(a.handleRoleCreate)))
	a.mux.HandleFunc("GET /admin-roles/{id}/edit", requireAuth(wrap(a.handleRoleEditForm)))
	a.mux.HandleFunc("PUT /admin-roles/{id}", requireAuth(wrap(a.handleRoleUpdate)))
	a.mux.HandleFunc("DELETE /admin-roles/{id}", requireAuth(wrap(a.handleRoleDelete)))
	a.mux.HandleFunc("GET /admin-roles/{id}/permissions", requireAuth(wrap(a.handleRolePermissionsForm)))
	a.mux.HandleFunc("POST /admin-roles/{id}/permissions", requireAuth(wrap(a.handleRolePermissionsSave)))
}

func pathID(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("id"))
}

func (a *App) renderError(w http.ResponseWriter, r *http.Request, msg string) {
	a.render(w, r, usersPermissionsKey, components.Card(components.Flash("error", msg)))
}

func (a *App) roleOptions() ([]components.RoleOption, error) {
	roles, err := a.rbac.listRoles()
	if err != nil {
		return nil, err
	}
	out := make([]components.RoleOption, 0, len(roles))
	for _, role := range roles {
		out = append(out, components.RoleOption{ID: role.ID, Name: role.Name})
	}
	return out, nil
}

// --- Roles ---

func (a *App) handleRolesList(w http.ResponseWriter, r *http.Request) {
	roles, err := a.rbac.listRoles()
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	users, err := a.rbac.listUsers()
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	counts := map[int]int{}
	for _, u := range users {
		counts[u.RoleID]++
	}

	summaries := make([]components.RoleSummary, 0, len(roles))
	for _, role := range roles {
		summaries = append(summaries, components.RoleSummary{
			ID: role.ID, Name: role.Name, CanAccessConfigurator: role.CanAccessConfigurator,
			CanManageUsers: role.CanManageUsers, IsSystemAdmin: role.IsSystemAdmin, UserCount: counts[role.ID],
		})
	}
	a.render(w, r, usersPermissionsKey, components.RoleListPage(summaries))
}

func (a *App) handleRoleNewForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, usersPermissionsKey, components.RoleForm("/admin-roles/new", components.RoleSummary{}, true, ""))
}

func (a *App) handleRoleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Could not read form data.")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	canConfig := r.FormValue("can_access_configurator") == "on"
	canUsers := r.FormValue("can_manage_users") == "on"
	summary := components.RoleSummary{Name: name, CanAccessConfigurator: canConfig, CanManageUsers: canUsers}

	if name == "" {
		a.render(w, r, usersPermissionsKey, components.RoleForm("/admin-roles/new", summary, true, "Name is required."))
		return
	}
	if _, err := a.rbac.createRole(name, canConfig, canUsers); err != nil {
		a.render(w, r, usersPermissionsKey, components.RoleForm("/admin-roles/new", summary, true, err.Error()))
		return
	}

	w.Header().Set("HX-Redirect", "/admin-roles")
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleRoleEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	role, err := a.rbac.getRole(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	summary := components.RoleSummary{
		ID: role.ID, Name: role.Name, CanAccessConfigurator: role.CanAccessConfigurator,
		CanManageUsers: role.CanManageUsers, IsSystemAdmin: role.IsSystemAdmin,
	}
	a.render(w, r, usersPermissionsKey, components.RoleForm(fmt.Sprintf("/admin-roles/%d", id), summary, false, ""))
}

func (a *App) handleRoleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Could not read form data.")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	canConfig := r.FormValue("can_access_configurator") == "on"
	canUsers := r.FormValue("can_manage_users") == "on"
	action := fmt.Sprintf("/admin-roles/%d", id)
	summary := components.RoleSummary{ID: id, Name: name, CanAccessConfigurator: canConfig, CanManageUsers: canUsers}

	if name == "" {
		a.render(w, r, usersPermissionsKey, components.RoleForm(action, summary, false, "Name is required."))
		return
	}
	if err := a.rbac.updateRole(id, name, canConfig, canUsers); err != nil {
		a.render(w, r, usersPermissionsKey, components.RoleForm(action, summary, false, err.Error()))
		return
	}

	w.Header().Set("HX-Redirect", "/admin-roles")
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleRoleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := a.rbac.deleteRole(id); err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	w.Header().Set("HX-Redirect", "/admin-roles")
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleRolePermissionsForm(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := a.rbac.getRole(id); err != nil {
		http.NotFound(w, r)
		return
	}
	grants, err := a.rbac.roleGrants(id)
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}

	rows := permissionRowsForGrants(a.allResources(), grants)
	content := components.RoleGrantMatrix(fmt.Sprintf("/admin-roles/%d/permissions", id), rows, "")
	a.render(w, r, usersPermissionsKey, content)
}

func (a *App) handleRolePermissionsSave(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Could not read form data.")
		return
	}

	grants := map[string]map[Action]bool{}
	for _, res := range a.allResources() {
		for _, action := range Actions {
			if r.PostForm.Get(fmt.Sprintf("grant__%s__%s", res.Key, action)) == "on" {
				if grants[res.Key] == nil {
					grants[res.Key] = map[Action]bool{}
				}
				grants[res.Key][action] = true
			}
		}
	}

	if err := a.rbac.setRoleGrants(id, grants); err != nil {
		a.renderError(w, r, err.Error())
		return
	}

	w.Header().Set("HX-Redirect", "/admin-roles")
	w.WriteHeader(http.StatusOK)
}

func permissionRowsForGrants(resources []*Resource, grants map[string]map[Action]bool) []components.PermissionResourceRow {
	rows := make([]components.PermissionResourceRow, 0, len(resources))
	for _, res := range resources {
		granted := map[string]bool{}
		for action, ok := range grants[res.Key] {
			granted[string(action)] = ok
		}
		rows = append(rows, components.PermissionResourceRow{ResourceKey: res.Key, ResourceLabel: res.Label, Granted: granted})
	}
	return rows
}

// --- Users ---

func (a *App) handleUsersList(w http.ResponseWriter, r *http.Request) {
	users, err := a.rbac.listUsers()
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	summaries := make([]components.UserSummary, 0, len(users))
	for _, u := range users {
		summaries = append(summaries, components.UserSummary{ID: u.ID, Username: u.Username, RoleName: u.RoleName})
	}
	a.render(w, r, usersPermissionsKey, components.UserListPage(summaries))
}

func (a *App) handleUserNewForm(w http.ResponseWriter, r *http.Request) {
	roles, err := a.roleOptions()
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	defaultRoleID := 0
	if len(roles) > 0 {
		defaultRoleID = roles[len(roles)-1].ID
	}
	a.render(w, r, usersPermissionsKey, components.UserForm("/admin-users/new", "", defaultRoleID, roles, true, ""))
}

func (a *App) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Could not read form data.")
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	roleID, _ := strconv.Atoi(r.FormValue("role_id"))

	roles, err := a.roleOptions()
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}

	if username == "" || password == "" {
		a.render(w, r, usersPermissionsKey,
			components.UserForm("/admin-users/new", username, roleID, roles, true, "Username and password are required."))
		return
	}
	if _, err := a.rbac.createUser(username, password, roleID); err != nil {
		a.render(w, r, usersPermissionsKey, components.UserForm("/admin-users/new", username, roleID, roles, true, err.Error()))
		return
	}

	w.Header().Set("HX-Redirect", "/admin-users")
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleUserEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	user, err := a.rbac.getUser(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	roles, err := a.roleOptions()
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	a.render(w, r, usersPermissionsKey,
		components.UserForm(fmt.Sprintf("/admin-users/%d", id), user.Username, user.Role.ID, roles, false, ""))
}

func (a *App) handleUserUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Could not read form data.")
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	roleID, _ := strconv.Atoi(r.FormValue("role_id"))
	action := fmt.Sprintf("/admin-users/%d", id)

	roles, err := a.roleOptions()
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}

	if username == "" {
		a.render(w, r, usersPermissionsKey, components.UserForm(action, username, roleID, roles, false, "Username is required."))
		return
	}
	if err := a.rbac.updateUser(id, username, roleID, password); err != nil {
		a.render(w, r, usersPermissionsKey, components.UserForm(action, username, roleID, roles, false, err.Error()))
		return
	}

	w.Header().Set("HX-Redirect", "/admin-users")
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := a.rbac.deleteUser(id); err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	w.Header().Set("HX-Redirect", "/admin-users")
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleUserPermissionsForm(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := a.rbac.getUser(id); err != nil {
		http.NotFound(w, r)
		return
	}
	overrides, err := a.rbac.userOverrides(id)
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}

	rows := permissionRowsForOverrides(a.allResources(), overrides)
	content := components.UserOverrideMatrix(fmt.Sprintf("/admin-users/%d/permissions", id), rows, "")
	a.render(w, r, usersPermissionsKey, content)
}

func (a *App) handleUserPermissionsSave(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Could not read form data.")
		return
	}

	var overrides []PermissionOverride
	for _, res := range a.allResources() {
		for _, action := range Actions {
			switch r.PostForm.Get(fmt.Sprintf("override__%s__%s", res.Key, action)) {
			case "grant":
				overrides = append(overrides, PermissionOverride{ResourceKey: res.Key, Action: action, Allowed: true})
			case "deny":
				overrides = append(overrides, PermissionOverride{ResourceKey: res.Key, Action: action, Allowed: false})
			}
		}
	}

	if err := a.rbac.setUserOverrides(id, overrides); err != nil {
		a.renderError(w, r, err.Error())
		return
	}

	w.Header().Set("HX-Redirect", "/admin-users")
	w.WriteHeader(http.StatusOK)
}

func permissionRowsForOverrides(resources []*Resource, overrides map[string]map[Action]bool) []components.PermissionResourceRow {
	rows := make([]components.PermissionResourceRow, 0, len(resources))
	for _, res := range resources {
		ov := map[string]string{}
		for action, allowed := range overrides[res.Key] {
			if allowed {
				ov[string(action)] = "grant"
			} else {
				ov[string(action)] = "deny"
			}
		}
		rows = append(rows, components.PermissionResourceRow{ResourceKey: res.Key, ResourceLabel: res.Label, Override: ov})
	}
	return rows
}

// --- API tokens ---

func (a *App) handleTokensList(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	user, err := a.rbac.getUser(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tokens, err := a.rbac.listTokens(id)
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	a.render(w, r, usersPermissionsKey, components.TokenListPage(user.Username, id, toTokenViews(tokens), "", ""))
}

func (a *App) handleTokenCreate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	user, err := a.rbac.getUser(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.renderError(w, r, "Could not read form data.")
		return
	}
	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		label = "Unnamed token"
	}

	raw, err := a.rbac.createToken(id, label)
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	tokens, err := a.rbac.listTokens(id)
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	a.render(w, r, usersPermissionsKey, components.TokenListPage(user.Username, id, toTokenViews(tokens), raw, ""))
}

func (a *App) handleTokenRevoke(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tokenID, err := strconv.Atoi(r.PathValue("tokenID"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	user, err := a.rbac.getUser(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := a.rbac.revokeToken(id, tokenID); err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	tokens, err := a.rbac.listTokens(id)
	if err != nil {
		a.renderError(w, r, err.Error())
		return
	}
	a.render(w, r, usersPermissionsKey, components.TokenListPage(user.Username, id, toTokenViews(tokens), "", ""))
}

func toTokenViews(tokens []APIToken) []components.APITokenView {
	out := make([]components.APITokenView, 0, len(tokens))
	for _, t := range tokens {
		lastUsed := ""
		if t.LastUsedAt != nil {
			lastUsed = *t.LastUsedAt
		}
		out = append(out, components.APITokenView{ID: t.ID, Label: t.Label, CreatedAt: t.CreatedAt, LastUsedAt: lastUsed})
	}
	return out
}

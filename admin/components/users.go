package components

import (
	"fmt"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/chasefleming/elem-go/htmx"
)

// permActions lists the four grantable actions, in the order every
// permission matrix shows its columns. A plain []string (not importing
// admin.Action) keeps this package free of the admin package, same as the
// rest of components.
var permActions = []string{"view", "create", "edit", "delete"}

func actionLabel(a string) string {
	switch a {
	case "view":
		return "View"
	case "create":
		return "Create"
	case "edit":
		return "Edit"
	case "delete":
		return "Delete"
	default:
		return a
	}
}

// --- Roles ---

type RoleSummary struct {
	ID                    int
	Name                  string
	CanAccessConfigurator bool
	CanManageUsers        bool
	IsSystemAdmin         bool
	UserCount             int
}

func RoleListPage(roles []RoleSummary) elem.Node {
	rows := elem.TransformEach(roles, func(role RoleSummary) elem.Node {
		editHref := fmt.Sprintf("/admin-roles/%d/edit", role.ID)
		permHref := fmt.Sprintf("/admin-roles/%d/permissions", role.ID)

		actions := []elem.Node{
			navButton(editHref, "Edit"),
			navButton(permHref, "Permissions"),
		}
		if !role.IsSystemAdmin {
			actions = append(actions, elem.Button(attrs.Props{
				attrs.Class:    "btn btn-sm btn-danger",
				htmx.HXDelete:  editHref[:len(editHref)-len("/edit")],
				htmx.HXTarget:  "body",
				htmx.HXConfirm: fmt.Sprintf("Delete role %q?", role.Name),
			}, elem.Text("Delete")))
		}

		return elem.Tr(nil,
			elem.Td(nil, elem.Text(role.Name)),
			elem.Td(nil, elem.Text(fmt.Sprintf("%d", role.UserCount))),
			elem.Td(attrs.Props{attrs.Class: "row-actions"}, actions...),
		)
	})

	return Card(
		elem.Div(attrs.Props{attrs.Class: "toolbar"},
			elem.H1(nil, elem.Text("Roles")),
			elem.Div(nil,
				navButtonGhost("/admin-users", "Users"),
				navButton("/admin-roles/new", "New role"),
			),
		),
		elem.Table(nil,
			elem.THead(nil, elem.Tr(nil,
				elem.Th(nil, elem.Text("Role")),
				elem.Th(nil, elem.Text("Users")),
				elem.Th(attrs.Props{attrs.Style: "text-align:right"}, elem.Text("Actions")),
			)),
			elem.TBody(nil, rows...),
		),
	)
}

func RoleForm(action string, role RoleSummary, isNew bool, errorMsg string) elem.Node {
	title := "New Role"
	if !isNew {
		title = "Edit Role: " + role.Name
	}

	nameInput := elem.Input(attrs.Props{attrs.Type: "text", attrs.Name: "name", attrs.Value: role.Name, attrs.Required: "true"})
	configCheckbox := attrs.Props{attrs.Type: "checkbox", attrs.Name: "can_access_configurator"}
	usersCheckbox := attrs.Props{attrs.Type: "checkbox", attrs.Name: "can_manage_users"}
	if role.CanAccessConfigurator {
		configCheckbox[attrs.Checked] = "true"
	}
	if role.CanManageUsers {
		usersCheckbox[attrs.Checked] = "true"
	}
	if role.IsSystemAdmin {
		nameInput = elem.Input(attrs.Props{attrs.Type: "text", attrs.Name: "name", attrs.Value: role.Name, attrs.Disabled: "true"})
		configCheckbox[attrs.Disabled] = "true"
		usersCheckbox[attrs.Disabled] = "true"
	}

	method := "post"
	if !isNew {
		method = "put"
	}
	formAttrs := attrs.Props{attrs.Class: "stacked-form", htmx.HXTarget: "#" + ContentTargetID}
	if method == "put" {
		formAttrs[htmx.HXPut] = action
	} else {
		formAttrs[htmx.HXPost] = action
	}

	return Card(
		elem.H1(nil, elem.Text(title)),
		Flash("error", errorMsg),
		elem.Form(formAttrs,
			elem.Div(attrs.Props{attrs.Class: "form-field"},
				elem.Label(nil, elem.Text("Name")),
				nameInput,
			),
			elem.Div(attrs.Props{attrs.Class: "form-field form-field-checkbox"},
				elem.Input(configCheckbox),
				elem.Label(nil, elem.Text("Can access Configurator")),
			),
			elem.Div(attrs.Props{attrs.Class: "form-field form-field-checkbox"},
				elem.Input(usersCheckbox),
				elem.Label(nil, elem.Text("Can manage Users & Permissions")),
			),
			elem.Div(attrs.Props{attrs.Class: "form-actions"},
				elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text("Save")),
				navButtonGhost("/admin-roles", "Cancel"),
			),
		),
	)
}

// --- Users ---

type UserSummary struct {
	ID       int
	Username string
	RoleName string
}

type RoleOption struct {
	ID   int
	Name string
}

func UserListPage(users []UserSummary) elem.Node {
	rows := elem.TransformEach(users, func(u UserSummary) elem.Node {
		editHref := fmt.Sprintf("/admin-users/%d/edit", u.ID)
		tokensHref := fmt.Sprintf("/admin-users/%d/tokens", u.ID)
		permHref := fmt.Sprintf("/admin-users/%d/permissions", u.ID)

		return elem.Tr(nil,
			elem.Td(nil, elem.Text(u.Username)),
			elem.Td(nil, elem.Text(u.RoleName)),
			elem.Td(attrs.Props{attrs.Class: "row-actions"},
				navButton(editHref, "Edit"),
				navButton(permHref, "Permissions"),
				navButton(tokensHref, "API tokens"),
				elem.Button(attrs.Props{
					attrs.Class:    "btn btn-sm btn-danger",
					htmx.HXDelete:  "/admin-users/" + fmt.Sprintf("%d", u.ID),
					htmx.HXTarget:  "body",
					htmx.HXConfirm: fmt.Sprintf("Delete user %q?", u.Username),
				}, elem.Text("Delete")),
			),
		)
	})

	return Card(
		elem.Div(attrs.Props{attrs.Class: "toolbar"},
			elem.H1(nil, elem.Text("Users")),
			elem.Div(nil,
				navButtonGhost("/admin-roles", "Roles"),
				navButton("/admin-users/new", "New user"),
			),
		),
		elem.Table(nil,
			elem.THead(nil, elem.Tr(nil,
				elem.Th(nil, elem.Text("Username")),
				elem.Th(nil, elem.Text("Role")),
				elem.Th(attrs.Props{attrs.Style: "text-align:right"}, elem.Text("Actions")),
			)),
			elem.TBody(nil, rows...),
		),
	)
}

func UserForm(action string, username string, roleID int, roles []RoleOption, isNew bool, errorMsg string) elem.Node {
	title := "New User"
	passwordLabel := "Password"
	if !isNew {
		title = "Edit User: " + username
		passwordLabel = "New password (leave blank to keep current)"
	}

	options := make([]elem.Node, 0, len(roles))
	for _, r := range roles {
		optAttrs := attrs.Props{attrs.Value: fmt.Sprintf("%d", r.ID)}
		if r.ID == roleID {
			optAttrs[attrs.Selected] = "true"
		}
		options = append(options, elem.Option(optAttrs, elem.Text(r.Name)))
	}

	method := "post"
	if !isNew {
		method = "put"
	}
	formAttrs := attrs.Props{attrs.Class: "stacked-form", htmx.HXTarget: "#" + ContentTargetID}
	if method == "put" {
		formAttrs[htmx.HXPut] = action
	} else {
		formAttrs[htmx.HXPost] = action
	}

	passwordAttrs := attrs.Props{attrs.Type: "password", attrs.Name: "password"}
	if isNew {
		passwordAttrs[attrs.Required] = "true"
	}

	return Card(
		elem.H1(nil, elem.Text(title)),
		Flash("error", errorMsg),
		elem.Form(formAttrs,
			elem.Div(attrs.Props{attrs.Class: "form-field"},
				elem.Label(nil, elem.Text("Username")),
				elem.Input(attrs.Props{attrs.Type: "text", attrs.Name: "username", attrs.Value: username, attrs.Required: "true"}),
			),
			elem.Div(attrs.Props{attrs.Class: "form-field"},
				elem.Label(nil, elem.Text(passwordLabel)),
				elem.Input(passwordAttrs),
			),
			elem.Div(attrs.Props{attrs.Class: "form-field"},
				elem.Label(nil, elem.Text("Role")),
				elem.Select(attrs.Props{attrs.Name: "role_id"}, options...),
			),
			elem.Div(attrs.Props{attrs.Class: "form-actions"},
				elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text("Save")),
				navButtonGhost("/admin-users", "Cancel"),
			),
		),
	)
}

// --- Permission matrices ---

// PermissionResourceRow is one resource's row in a permission matrix.
// Granted is used by RoleGrantMatrix (boolean per action); Override is
// used by UserOverrideMatrix (per action: "" inherit, "grant", "deny").
type PermissionResourceRow struct {
	ResourceKey   string
	ResourceLabel string
	Granted       map[string]bool
	Override      map[string]string
}

// RoleGrantMatrix renders a resource x action checkbox grid for editing a
// role's permission grants. Rows are resourceKey-keyed (resources aren't
// added/removed from this form), so each cell's name is deterministic —
// no row-id bookkeeping needed.
func RoleGrantMatrix(saveAction string, rows []PermissionResourceRow, errorMsg string) elem.Node {
	headerCells := []elem.Node{elem.Th(nil, elem.Text("Resource"))}
	for _, act := range permActions {
		headerCells = append(headerCells, elem.Th(nil, elem.Text(actionLabel(act))))
	}

	bodyRows := elem.TransformEach(rows, func(row PermissionResourceRow) elem.Node {
		cells := []elem.Node{elem.Td(nil, elem.Text(row.ResourceLabel))}
		for _, act := range permActions {
			cbAttrs := attrs.Props{attrs.Type: "checkbox", attrs.Name: "grant__" + row.ResourceKey + "__" + act}
			if row.Granted[act] {
				cbAttrs[attrs.Checked] = "true"
			}
			cells = append(cells, elem.Td(nil, elem.Input(cbAttrs)))
		}
		return elem.Tr(nil, cells...)
	})

	return Card(
		elem.H1(nil, elem.Text("Permissions")),
		Flash("error", errorMsg),
		elem.Form(attrs.Props{htmx.HXPost: saveAction, htmx.HXTarget: "#" + ContentTargetID},
			elem.Table(nil, elem.THead(nil, elem.Tr(nil, headerCells...)), elem.TBody(nil, bodyRows...)),
			elem.Div(attrs.Props{attrs.Class: "form-actions"},
				elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text("Save")),
				navButtonGhost("/admin-roles", "Cancel"),
			),
		),
	)
}

// UserOverrideMatrix renders a resource x action tri-state grid (Inherit /
// Grant / Deny) for a user's permission overrides on top of their role.
func UserOverrideMatrix(saveAction string, rows []PermissionResourceRow, errorMsg string) elem.Node {
	headerCells := []elem.Node{elem.Th(nil, elem.Text("Resource"))}
	for _, act := range permActions {
		headerCells = append(headerCells, elem.Th(nil, elem.Text(actionLabel(act))))
	}

	bodyRows := elem.TransformEach(rows, func(row PermissionResourceRow) elem.Node {
		cells := []elem.Node{elem.Td(nil, elem.Text(row.ResourceLabel))}
		for _, act := range permActions {
			current := row.Override[act]
			name := "override__" + row.ResourceKey + "__" + act
			options := []elem.Node{
				optionNode("", "Inherit", current),
				optionNode("grant", "Grant", current),
				optionNode("deny", "Deny", current),
			}
			cells = append(cells, elem.Td(nil, elem.Select(attrs.Props{attrs.Name: name}, options...)))
		}
		return elem.Tr(nil, cells...)
	})

	return Card(
		elem.H1(nil, elem.Text("Permission overrides")),
		Flash("error", errorMsg),
		elem.Form(attrs.Props{htmx.HXPost: saveAction, htmx.HXTarget: "#" + ContentTargetID},
			elem.Table(nil, elem.THead(nil, elem.Tr(nil, headerCells...)), elem.TBody(nil, bodyRows...)),
			elem.Div(attrs.Props{attrs.Class: "form-actions"},
				elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text("Save")),
				navButtonGhost("/admin-users", "Cancel"),
			),
		),
	)
}

func optionNode(value, label, current string) elem.Node {
	optAttrs := attrs.Props{attrs.Value: value}
	if value == current {
		optAttrs[attrs.Selected] = "true"
	}
	return elem.Option(optAttrs, elem.Text(label))
}

// --- API tokens ---

func TokenListPage(username string, userID int, tokens []APITokenView, newToken string, errorMsg string) elem.Node {
	createHref := fmt.Sprintf("/admin-users/%d/tokens", userID)

	rows := elem.TransformEach(tokens, func(t APITokenView) elem.Node {
		lastUsed := "never"
		if t.LastUsedAt != "" {
			lastUsed = t.LastUsedAt
		}
		revokeHref := fmt.Sprintf("/admin-users/%d/tokens/%d", userID, t.ID)
		return elem.Tr(nil,
			elem.Td(nil, elem.Text(t.Label)),
			elem.Td(nil, elem.Text(t.CreatedAt)),
			elem.Td(nil, elem.Text(lastUsed)),
			elem.Td(attrs.Props{attrs.Class: "row-actions"},
				elem.Button(attrs.Props{
					attrs.Class:    "btn btn-sm btn-danger",
					htmx.HXDelete:  revokeHref,
					htmx.HXTarget:  "#" + ContentTargetID,
					htmx.HXConfirm: "Revoke this token? Anything using it will stop working immediately.",
				}, elem.Text("Revoke")),
			),
		)
	})

	var newTokenBanner elem.Node = elem.None()
	if newToken != "" {
		newTokenBanner = elem.Div(attrs.Props{attrs.Class: "flash flash-success"},
			elem.Text("New token (copy it now — it won't be shown again): "),
			elem.Code(nil, elem.Text(newToken)),
		)
	}

	return Card(
		elem.Div(attrs.Props{attrs.Class: "toolbar"},
			elem.H1(nil, elem.Text("API tokens: "+username)),
			navButtonGhost("/admin-users", "Back to users"),
		),
		Flash("error", errorMsg),
		newTokenBanner,
		elem.Table(nil,
			elem.THead(nil, elem.Tr(nil,
				elem.Th(nil, elem.Text("Label")),
				elem.Th(nil, elem.Text("Created")),
				elem.Th(nil, elem.Text("Last used")),
				elem.Th(attrs.Props{attrs.Style: "text-align:right"}, elem.Text("")),
			)),
			elem.TBody(nil, rows...),
		),
		elem.Form(attrs.Props{
			attrs.Class:   "add-form",
			htmx.HXPost:   createHref,
			htmx.HXTarget: "#" + ContentTargetID,
		},
			elem.Input(attrs.Props{attrs.Type: "text", attrs.Name: "label", attrs.Placeholder: "Label (e.g. \"CI pipeline\")"}),
			elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text("Generate token")),
		),
	)
}

// APITokenView is the display-ready shape of an issued token.
type APITokenView struct {
	ID         int
	Label      string
	CreatedAt  string
	LastUsedAt string
}

func navButton(href, label string) elem.Node {
	return elem.Button(attrs.Props{
		attrs.Class:    "btn btn-sm",
		htmx.HXGet:     href,
		htmx.HXTarget:  "#" + ContentTargetID,
		htmx.HXPushURL: "true",
	}, elem.Text(label))
}

func navButtonGhost(href, label string) elem.Node {
	return elem.A(attrs.Props{
		attrs.Class:    "btn btn-ghost",
		attrs.Href:     href,
		htmx.HXGet:     href,
		htmx.HXTarget:  "#" + ContentTargetID,
		htmx.HXPushURL: "true",
	}, elem.Text(label))
}

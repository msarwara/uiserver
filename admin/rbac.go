package admin

// Role is a named set of capabilities: whether it can reshape resource
// schemas via the Configurator, manage Users/Roles/Permissions, and (via
// the role_permissions grants in rbacStore) which resource actions it's
// granted. IsSystemAdmin is only ever true for the seeded "Admin" role —
// it bypasses the permission grant tables entirely (see App.hasPermission)
// so a misconfigured grant can never lock every admin out.
type Role struct {
	ID                    int
	Name                  string
	CanAccessConfigurator bool
	CanManageUsers        bool
	IsSystemAdmin         bool
}

package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// rbacStore persists roles, users, permission grants, and API tokens.
// Mirrors schemaStore's shape: an execer-taking struct over *sql.DB so
// multi-step writes can run inside one transaction.
type rbacStore struct {
	db *sql.DB
}

// APIToken is one issued (and still live) API token, as shown in the
// wizard — the raw token itself is never stored or shown again after
// creation, only its hash.
type APIToken struct {
	ID         int
	Label      string
	CreatedAt  string
	LastUsedAt *string
}

func newRBACStore(db *sql.DB) (*rbacStore, error) {
	s := &rbacStore{db: db}
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}
	if err := s.bootstrap(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *rbacStore) ensureSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS admin_roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			can_access_configurator INTEGER NOT NULL DEFAULT 0,
			can_manage_users INTEGER NOT NULL DEFAULT 0,
			is_system_admin INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS admin_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role_id INTEGER NOT NULL REFERENCES admin_roles(id)
		)`,
		`CREATE TABLE IF NOT EXISTS admin_role_permissions (
			role_id INTEGER NOT NULL REFERENCES admin_roles(id),
			resource_key TEXT NOT NULL,
			action TEXT NOT NULL,
			PRIMARY KEY (role_id, resource_key, action)
		)`,
		`CREATE TABLE IF NOT EXISTS admin_user_permissions (
			user_id INTEGER NOT NULL REFERENCES admin_users(id),
			resource_key TEXT NOT NULL,
			action TEXT NOT NULL,
			allowed INTEGER NOT NULL,
			PRIMARY KEY (user_id, resource_key, action)
		)`,
		`CREATE TABLE IF NOT EXISTS admin_api_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES admin_users(id),
			token_hash TEXT UNIQUE NOT NULL,
			label TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			last_used_at TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// bootstrap seeds the three default roles and a demo admin/admin user the
// first time admin_roles is empty. Same demo-grade convenience as the
// project's other defaults (in-memory sessions, etc.) — change it before
// any real use.
func (s *rbacStore) bootstrap() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_roles`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	adminID, err := insertRole(tx, "Admin", true, true, true)
	if err != nil {
		return err
	}
	if _, err := insertRole(tx, "Editor", true, false, false); err != nil {
		return err
	}
	if _, err := insertRole(tx, "User", false, false, false); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO admin_users (username, password_hash, role_id) VALUES (?, ?, ?)`,
		"admin", string(hash), adminID); err != nil {
		return err
	}

	return tx.Commit()
}

func insertRole(ex execer, name string, canConfigurator, canManageUsers, isSystemAdmin bool) (int64, error) {
	res, err := ex.Exec(`INSERT INTO admin_roles (name, can_access_configurator, can_manage_users, is_system_admin)
		VALUES (?, ?, ?, ?)`, name, boolToInt(canConfigurator), boolToInt(canManageUsers), boolToInt(isSystemAdmin))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// --- Roles ---

func (s *rbacStore) listRoles() ([]Role, error) {
	rows, err := s.db.Query(`SELECT id, name, can_access_configurator, can_manage_users, is_system_admin
		FROM admin_roles ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Role
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *rbacStore) getRole(id int) (Role, error) {
	row := s.db.QueryRow(`SELECT id, name, can_access_configurator, can_manage_users, is_system_admin
		FROM admin_roles WHERE id = ?`, id)
	return scanRole(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRole(row rowScanner) (Role, error) {
	var r Role
	var canConfig, canUsers, isSystem int64
	if err := row.Scan(&r.ID, &r.Name, &canConfig, &canUsers, &isSystem); err != nil {
		return Role{}, err
	}
	r.CanAccessConfigurator = canConfig != 0
	r.CanManageUsers = canUsers != 0
	r.IsSystemAdmin = isSystem != 0
	return r, nil
}

func (s *rbacStore) createRole(name string, canConfigurator, canManageUsers bool) (Role, error) {
	id, err := insertRole(s.db, name, canConfigurator, canManageUsers, false)
	if err != nil {
		return Role{}, friendlyUniqueError(err, "role name")
	}
	return s.getRole(int(id))
}

func (s *rbacStore) updateRole(id int, name string, canConfigurator, canManageUsers bool) error {
	existing, err := s.getRole(id)
	if err != nil {
		return err
	}
	if existing.IsSystemAdmin {
		return fmt.Errorf("the Admin role's capabilities can't be changed")
	}
	_, err = s.db.Exec(`UPDATE admin_roles SET name = ?, can_access_configurator = ?, can_manage_users = ?
		WHERE id = ?`, name, boolToInt(canConfigurator), boolToInt(canManageUsers), id)
	return friendlyUniqueError(err, "role name")
}

func (s *rbacStore) deleteRole(id int) error {
	role, err := s.getRole(id)
	if err != nil {
		return err
	}
	if role.IsSystemAdmin {
		return fmt.Errorf("the Admin role can't be deleted")
	}

	var userCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_users WHERE role_id = ?`, id).Scan(&userCount); err != nil {
		return err
	}
	if userCount > 0 {
		return fmt.Errorf("reassign this role's %d user(s) first", userCount)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM admin_role_permissions WHERE role_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM admin_roles WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Role permission grants ---

// roleGrant reports whether roleID has an explicit grant for
// resourceKey/action.
func (s *rbacStore) roleGrant(roleID int, resourceKey string, action Action) bool {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM admin_role_permissions WHERE role_id = ? AND resource_key = ? AND action = ?`,
		roleID, resourceKey, string(action)).Scan(&one)
	return err == nil
}

// roleGrants returns every (resourceKey, action) pair roleID is granted,
// for rendering the permission matrix.
func (s *rbacStore) roleGrants(roleID int) (map[string]map[Action]bool, error) {
	rows, err := s.db.Query(`SELECT resource_key, action FROM admin_role_permissions WHERE role_id = ?`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]map[Action]bool{}
	for rows.Next() {
		var key, action string
		if err := rows.Scan(&key, &action); err != nil {
			return nil, err
		}
		if out[key] == nil {
			out[key] = map[Action]bool{}
		}
		out[key][Action(action)] = true
	}
	return out, rows.Err()
}

// setRoleGrants replaces every grant for roleID with exactly the ones in
// grants (resourceKey -> set of granted actions).
func (s *rbacStore) setRoleGrants(roleID int, grants map[string]map[Action]bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM admin_role_permissions WHERE role_id = ?`, roleID); err != nil {
		return err
	}
	for resourceKey, actions := range grants {
		for action, granted := range actions {
			if !granted {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO admin_role_permissions (role_id, resource_key, action) VALUES (?, ?, ?)`,
				roleID, resourceKey, string(action)); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// --- Per-user permission overrides ---

// userOverride reports an explicit per-user override for
// resourceKey/action, if one exists.
func (s *rbacStore) userOverride(userID int, resourceKey string, action Action) (allowed bool, ok bool) {
	var allowedInt int64
	err := s.db.QueryRow(`SELECT allowed FROM admin_user_permissions WHERE user_id = ? AND resource_key = ? AND action = ?`,
		userID, resourceKey, string(action)).Scan(&allowedInt)
	if err != nil {
		return false, false
	}
	return allowedInt != 0, true
}

// userOverrides returns every override for userID: resourceKey -> action ->
// allowed (true=grant, false=deny), for rendering the matrix.
func (s *rbacStore) userOverrides(userID int) (map[string]map[Action]bool, error) {
	rows, err := s.db.Query(`SELECT resource_key, action, allowed FROM admin_user_permissions WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]map[Action]bool{}
	for rows.Next() {
		var key, action string
		var allowed int64
		if err := rows.Scan(&key, &action, &allowed); err != nil {
			return nil, err
		}
		if out[key] == nil {
			out[key] = map[Action]bool{}
		}
		out[key][Action(action)] = allowed != 0
	}
	return out, rows.Err()
}

// PermissionOverride is one explicit per-user grant/deny to persist.
type PermissionOverride struct {
	ResourceKey string
	Action      Action
	Allowed     bool
}

// setUserOverrides replaces every override for userID with exactly the
// ones given (an absent resourceKey/action pair means "inherit the role").
func (s *rbacStore) setUserOverrides(userID int, overrides []PermissionOverride) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM admin_user_permissions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	for _, o := range overrides {
		if _, err := tx.Exec(`INSERT INTO admin_user_permissions (user_id, resource_key, action, allowed) VALUES (?, ?, ?, ?)`,
			userID, o.ResourceKey, string(o.Action), boolToInt(o.Allowed)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --- Users ---

// UserSummary is a user row with its role name resolved, for list views.
type UserSummary struct {
	ID       int
	Username string
	RoleID   int
	RoleName string
}

func (s *rbacStore) listUsers() ([]UserSummary, error) {
	rows, err := s.db.Query(`SELECT u.id, u.username, u.role_id, r.name
		FROM admin_users u JOIN admin_roles r ON r.id = u.role_id ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserSummary
	for rows.Next() {
		var us UserSummary
		if err := rows.Scan(&us.ID, &us.Username, &us.RoleID, &us.RoleName); err != nil {
			return nil, err
		}
		out = append(out, us)
	}
	return out, rows.Err()
}

// getUser loads a user by id, with its Role resolved.
func (s *rbacStore) getUser(id int) (*User, error) {
	row := s.db.QueryRow(`SELECT u.id, u.username, r.id, r.name, r.can_access_configurator, r.can_manage_users, r.is_system_admin
		FROM admin_users u JOIN admin_roles r ON r.id = u.role_id WHERE u.id = ?`, id)
	return scanUser(row)
}

func scanUser(row rowScanner) (*User, error) {
	var u User
	var canConfig, canUsers, isSystem int64
	if err := row.Scan(&u.ID, &u.Username, &u.Role.ID, &u.Role.Name, &canConfig, &canUsers, &isSystem); err != nil {
		return nil, err
	}
	u.Role.CanAccessConfigurator = canConfig != 0
	u.Role.CanManageUsers = canUsers != 0
	u.Role.IsSystemAdmin = isSystem != 0
	return &u, nil
}

func (s *rbacStore) createUser(username, password string, roleID int) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(`INSERT INTO admin_users (username, password_hash, role_id) VALUES (?, ?, ?)`,
		username, string(hash), roleID)
	if err != nil {
		return nil, friendlyUniqueError(err, "username")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.getUser(int(id))
}

// updateUser changes username/role, and the password only if newPassword
// is non-empty.
func (s *rbacStore) updateUser(id int, username string, roleID int, newPassword string) error {
	if newPassword == "" {
		_, err := s.db.Exec(`UPDATE admin_users SET username = ?, role_id = ? WHERE id = ?`, username, roleID, id)
		return friendlyUniqueError(err, "username")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE admin_users SET username = ?, role_id = ?, password_hash = ? WHERE id = ?`,
		username, roleID, string(hash), id)
	return friendlyUniqueError(err, "username")
}

func (s *rbacStore) deleteUser(id int) error {
	user, err := s.getUser(id)
	if err != nil {
		return err
	}
	if user.Role.IsSystemAdmin {
		var systemAdminUsers int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin_users u JOIN admin_roles r ON r.id = u.role_id
			WHERE r.is_system_admin = 1`).Scan(&systemAdminUsers); err != nil {
			return err
		}
		if systemAdminUsers <= 1 {
			return fmt.Errorf("can't delete the last Admin user")
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM admin_user_permissions WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM admin_api_tokens WHERE user_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM admin_users WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *rbacStore) verifyPassword(username, password string) (*User, error) {
	row := s.db.QueryRow(`SELECT u.id, u.username, r.id, r.name, r.can_access_configurator, r.can_manage_users, r.is_system_admin, u.password_hash
		FROM admin_users u JOIN admin_roles r ON r.id = u.role_id WHERE u.username = ?`, username)

	var u User
	var canConfig, canUsers, isSystem int64
	var hash string
	if err := row.Scan(&u.ID, &u.Username, &u.Role.ID, &u.Role.Name, &canConfig, &canUsers, &isSystem, &hash); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid username or password")
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}
	u.Role.CanAccessConfigurator = canConfig != 0
	u.Role.CanManageUsers = canUsers != 0
	u.Role.IsSystemAdmin = isSystem != 0
	return &u, nil
}

// --- API tokens ---

func (s *rbacStore) listTokens(userID int) ([]APIToken, error) {
	rows, err := s.db.Query(`SELECT id, label, created_at, last_used_at FROM admin_api_tokens
		WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []APIToken
	for rows.Next() {
		var t APIToken
		var lastUsed sql.NullString
		if err := rows.Scan(&t.ID, &t.Label, &t.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			t.LastUsedAt = &lastUsed.String
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// createToken generates a new random token for userID, stores only its
// hash, and returns the raw token — the only time it's ever available.
func (s *rbacStore) createToken(userID int, label string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	raw := hex.EncodeToString(buf)

	_, err := s.db.Exec(`INSERT INTO admin_api_tokens (user_id, token_hash, label, created_at) VALUES (?, ?, ?, ?)`,
		userID, hashToken(raw), label, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (s *rbacStore) revokeToken(userID, tokenID int) error {
	_, err := s.db.Exec(`DELETE FROM admin_api_tokens WHERE id = ? AND user_id = ?`, tokenID, userID)
	return err
}

// lookupToken resolves a raw bearer token to its user, stamping
// last_used_at. A token's entropy (32 random bytes) makes a fast hash
// (SHA-256) appropriate here, unlike a password — there's no dictionary to
// attack.
func (s *rbacStore) lookupToken(raw string) (*User, error) {
	hash := hashToken(raw)

	var tokenID, userID int
	if err := s.db.QueryRow(`SELECT id, user_id FROM admin_api_tokens WHERE token_hash = ?`, hash).
		Scan(&tokenID, &userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid API token")
		}
		return nil, err
	}

	user, err := s.getUser(userID)
	if err != nil {
		return nil, err
	}

	_, _ = s.db.Exec(`UPDATE admin_api_tokens SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), tokenID)
	return user, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// friendlyUniqueError turns a SQLite UNIQUE-constraint error into a message
// that names the field, instead of leaking the raw SQL error.
func friendlyUniqueError(err error, field string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("that %s is already in use", field)
	}
	return err
}

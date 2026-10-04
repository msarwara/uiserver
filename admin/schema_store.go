package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// schemaStore persists Resource definitions (label, page size, fields) so
// Configurator edits survive a restart, and so a resource's Go-code
// definition only matters the first time it's ever registered.
type schemaStore struct {
	db *sql.DB
}

func newSchemaStore(db *sql.DB) (*schemaStore, error) {
	s := &schemaStore{db: db}
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *schemaStore) ensureSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS admin_resources (
			key TEXT PRIMARY KEY,
			label TEXT NOT NULL,
			page_size INTEGER NOT NULL DEFAULT 10,
			position INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS admin_fields (
			resource_key TEXT NOT NULL REFERENCES admin_resources(key),
			name TEXT NOT NULL,
			label TEXT NOT NULL,
			type TEXT NOT NULL,
			required INTEGER NOT NULL DEFAULT 0,
			hide_in_grid INTEGER NOT NULL DEFAULT 0,
			hide_in_form INTEGER NOT NULL DEFAULT 0,
			options_json TEXT NOT NULL DEFAULT '[]',
			position INTEGER NOT NULL,
			PRIMARY KEY (resource_key, name)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// get loads a persisted resource definition by key. ok is false (with a nil
// error) if no resource with that key has been persisted yet.
func (s *schemaStore) get(key string) (res *Resource, ok bool, err error) {
	var label string
	var pageSize int
	row := s.db.QueryRow(`SELECT label, page_size FROM admin_resources WHERE key = ?`, key)
	if err := row.Scan(&label, &pageSize); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	fields, err := s.loadFields(key)
	if err != nil {
		return nil, false, err
	}
	return &Resource{Key: key, Label: label, PageSize: pageSize, Fields: fields}, true, nil
}

func (s *schemaStore) loadFields(key string) ([]Field, error) {
	rows, err := s.db.Query(`SELECT name, label, type, required, hide_in_grid, hide_in_form, options_json
		FROM admin_fields WHERE resource_key = ? ORDER BY position`, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fields []Field
	for rows.Next() {
		var f Field
		var typ, optionsJSON string
		var required, hideGrid, hideForm int64
		if err := rows.Scan(&f.Name, &f.Label, &typ, &required, &hideGrid, &hideForm, &optionsJSON); err != nil {
			return nil, err
		}
		f.Type = FieldType(typ)
		f.Required = required != 0
		f.HideInGrid = hideGrid != 0
		f.HideInForm = hideForm != 0
		if err := json.Unmarshal([]byte(optionsJSON), &f.Options); err != nil {
			return nil, fmt.Errorf("decode options for field %q: %w", f.Name, err)
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

// queryRower is satisfied by both *sql.DB and *sql.Tx. nextPosition takes
// one explicitly (rather than always using s.db) so it can run on an
// already-open transaction — with the connection pool capped at one
// connection (see main.go), querying s.db directly while a transaction
// holds that single connection would otherwise deadlock.
type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

// nextPosition returns the nav position to assign to a brand-new resource.
func (s *schemaStore) nextPosition(q queryRower) (int, error) {
	var maxPos sql.NullInt64
	if err := q.QueryRow(`SELECT MAX(position) FROM admin_resources`).Scan(&maxPos); err != nil {
		return 0, err
	}
	if !maxPos.Valid {
		return 0, nil
	}
	return int(maxPos.Int64) + 1, nil
}

// upsertResource writes res's label/page-size/fields. position is only
// applied on first insert (a later save, via ON CONFLICT, leaves the
// resource's original nav position untouched).
func (s *schemaStore) upsertResource(ex execer, res *Resource, position int) error {
	_, err := ex.Exec(`INSERT INTO admin_resources (key, label, page_size, position) VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET label = excluded.label, page_size = excluded.page_size`,
		res.Key, res.Label, res.pageSize(), position)
	if err != nil {
		return err
	}
	return s.replaceFieldRows(ex, res.Key, res.Fields)
}

// deleteResource removes a resource's persisted definition entirely —
// used when the Configurator deletes a resource outright.
func (s *schemaStore) deleteResource(ex execer, key string) error {
	if _, err := ex.Exec(`DELETE FROM admin_fields WHERE resource_key = ?`, key); err != nil {
		return err
	}
	_, err := ex.Exec(`DELETE FROM admin_resources WHERE key = ?`, key)
	return err
}

func (s *schemaStore) replaceFieldRows(ex execer, key string, fields []Field) error {
	if _, err := ex.Exec(`DELETE FROM admin_fields WHERE resource_key = ?`, key); err != nil {
		return err
	}
	for i, f := range fields {
		optsJSON, err := json.Marshal(f.Options)
		if err != nil {
			return err
		}
		_, err = ex.Exec(`INSERT INTO admin_fields
			(resource_key, name, label, type, required, hide_in_grid, hide_in_form, options_json, position)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			key, f.Name, f.Label, string(f.Type),
			boolToInt(f.Required), boolToInt(f.HideInGrid), boolToInt(f.HideInForm),
			string(optsJSON), i)
		if err != nil {
			return err
		}
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

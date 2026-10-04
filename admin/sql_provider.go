package admin

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// tableName is the SQL table backing a resource's data.
func tableName(resourceKey string) string {
	return "res_" + resourceKey
}

// sqlColumnType maps a Field's type to the SQL column type used when
// creating or extending its resource's table.
func sqlColumnType(t FieldType) string {
	switch t {
	case FieldNumber:
		return "REAL"
	case FieldCheckbox:
		return "BOOLEAN"
	default:
		return "TEXT"
	}
}

// execer is satisfied by both *sql.DB and *sql.Tx, so the DDL helpers below
// work whether or not they're part of a larger transaction.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// createResourceTable creates the table backing a new resource: an "id"
// primary key plus one column per field (field "id", if present in fields,
// is skipped since the primary key already covers it).
func createResourceTable(ex execer, resourceKey string, fields []Field) error {
	cols := []string{`"id" INTEGER PRIMARY KEY AUTOINCREMENT`}
	for _, f := range fields {
		if f.Name == "id" {
			continue
		}
		cols = append(cols, quoteIdent(f.Name)+" "+sqlColumnType(f.Type))
	}
	query := fmt.Sprintf(`CREATE TABLE %s (%s)`, quoteIdent(tableName(resourceKey)), strings.Join(cols, ", "))
	_, err := ex.Exec(query)
	return err
}

func addColumn(ex execer, resourceKey string, f Field) error {
	query := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`,
		quoteIdent(tableName(resourceKey)), quoteIdent(f.Name), sqlColumnType(f.Type))
	_, err := ex.Exec(query)
	return err
}

// renameColumn renames a column in place: SQLite moves the column's
// existing data along with it, which is exactly the "migrate existing
// values" behavior a Configurator field rename needs.
func renameColumn(ex execer, resourceKey, oldName, newName string) error {
	query := fmt.Sprintf(`ALTER TABLE %s RENAME COLUMN %s TO %s`,
		quoteIdent(tableName(resourceKey)), quoteIdent(oldName), quoteIdent(newName))
	_, err := ex.Exec(query)
	return err
}

func dropColumn(ex execer, resourceKey, name string) error {
	query := fmt.Sprintf(`ALTER TABLE %s DROP COLUMN %s`,
		quoteIdent(tableName(resourceKey)), quoteIdent(name))
	_, err := ex.Exec(query)
	return err
}

// dropResourceTable deletes a resource's table (and every row in it) —
// used when the Configurator deletes a resource outright.
func dropResourceTable(ex execer, resourceKey string) error {
	_, err := ex.Exec(fmt.Sprintf(`DROP TABLE %s`, quoteIdent(tableName(resourceKey))))
	return err
}

// insertSeedRows inserts a Resource's initial Seed records; used only the
// first time a resource's table is created.
func insertSeedRows(ex execer, resourceKey string, seed []Record) error {
	for _, rec := range seed {
		if _, err := insertRecord(ex, tableName(resourceKey), rec); err != nil {
			return err
		}
	}
	return nil
}

func insertRecord(ex execer, table string, data Record) (sql.Result, error) {
	cols := make([]string, 0, len(data))
	placeholders := make([]string, 0, len(data))
	args := make([]any, 0, len(data))
	for k, v := range data {
		if k == "id" {
			continue
		}
		cols = append(cols, quoteIdent(k))
		placeholders = append(placeholders, "?")
		args = append(args, v)
	}

	var query string
	if len(cols) == 0 {
		query = fmt.Sprintf(`INSERT INTO %s DEFAULT VALUES`, quoteIdent(table))
	} else {
		query = fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`,
			quoteIdent(table), strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	}
	return ex.Exec(query, args...)
}

// SQLProvider is the generic DataProvider auto-provisioned for a resource
// whose Provider is left nil: a real SQL table (see tableName), so field
// renames/adds/removes made through the Configurator are plain DDL rather
// than bespoke data-migration code.
type SQLProvider struct {
	db    *sql.DB
	table string
}

// NewSQLProvider returns a DataProvider backed by resourceKey's table. The
// table must already exist (App creates it on first registration).
func NewSQLProvider(db *sql.DB, resourceKey string) *SQLProvider {
	return &SQLProvider{db: db, table: tableName(resourceKey)}
}

func (p *SQLProvider) List(page, pageSize int) (ListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	var total int
	if err := p.db.QueryRow(`SELECT COUNT(*) FROM ` + quoteIdent(p.table)).Scan(&total); err != nil {
		return ListResult{}, err
	}

	offset := (page - 1) * pageSize
	rows, err := p.db.Query(`SELECT * FROM `+quoteIdent(p.table)+` ORDER BY "id" LIMIT ? OFFSET ?`, pageSize, offset)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	records, err := scanRows(rows)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Records: records, Total: total}, nil
}

func (p *SQLProvider) Get(id string) (Record, error) {
	rows, err := p.db.Query(`SELECT * FROM `+quoteIdent(p.table)+` WHERE "id" = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("record %q not found", id)
	}
	return records[0], nil
}

func (p *SQLProvider) Create(data Record) (Record, error) {
	res, err := insertRecord(p.db, p.table, data)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return p.Get(strconv.FormatInt(id, 10))
}

func (p *SQLProvider) Update(id string, data Record) (Record, error) {
	sets := make([]string, 0, len(data))
	args := make([]any, 0, len(data)+1)
	for k, v := range data {
		if k == "id" {
			continue
		}
		sets = append(sets, quoteIdent(k)+" = ?")
		args = append(args, v)
	}
	if len(sets) == 0 {
		return p.Get(id)
	}
	args = append(args, id)

	query := fmt.Sprintf(`UPDATE %s SET %s WHERE "id" = ?`, quoteIdent(p.table), strings.Join(sets, ", "))
	if _, err := p.db.Exec(query, args...); err != nil {
		return nil, err
	}
	return p.Get(id)
}

func (p *SQLProvider) Delete(id string) error {
	_, err := p.db.Exec(`DELETE FROM `+quoteIdent(p.table)+` WHERE "id" = ?`, id)
	return err
}

// scanRows reads every row into a Record keyed by column name, independent
// of what fields/types a Resource currently declares.
func scanRows(rows *sql.Rows) ([]Record, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []Record
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}

		rec := make(Record, len(cols))
		for i, col := range cols {
			rec[col] = normalizeSQLValue(vals[i])
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// normalizeSQLValue converts driver-returned []byte (common for TEXT
// columns) into a plain string so downstream display/form code only ever
// deals with Go's usual scalar types.
func normalizeSQLValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

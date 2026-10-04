package admin

import (
	"fmt"
	"regexp"
	"strings"
)

// identifierRE matches the names we're willing to use as a SQL table or
// column name: lowercase, starting with a letter, snake_case.
var identifierRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// reservedIdentifiers can't be used as a Field name or Resource key: "id"
// is the primary key every table already has, and the rest are common SQL
// reserved words that would otherwise need extra quoting rules to use safely.
var reservedIdentifiers = map[string]bool{
	"id": true, "select": true, "insert": true, "update": true, "delete": true,
	"from": true, "where": true, "table": true, "drop": true, "alter": true,
	"index": true, "primary": true, "key": true, "order": true, "group": true,
}

// validIdentifier checks that s is safe to interpolate directly into SQL as
// a table or column name. Table/column identifiers can't be passed as query
// parameters, so every Resource.Key and Field.Name must pass this check
// before it's ever used to build a DDL/DML string.
func validIdentifier(s string) error {
	if !identifierRE.MatchString(s) {
		return fmt.Errorf("%q must be lowercase letters, digits, and underscores, starting with a letter", s)
	}
	if reservedIdentifiers[s] {
		return fmt.Errorf("%q is a reserved name", s)
	}
	return nil
}

// quoteIdent double-quotes an identifier for safe interpolation into SQL,
// as defense-in-depth alongside validIdentifier's charset restriction.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

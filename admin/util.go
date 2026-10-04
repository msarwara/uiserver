package admin

import "fmt"

// toString renders any record value as a string for form inputs/HTML.
func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", t)
	}
}

// fieldDisplayValue renders a record value for display/forms, aware of the
// field's declared type. It exists because checkbox values round-trip
// through a real SQL column (SQLite has no native boolean: it may come
// back as a Go bool, or an int64 0/1), so plain toString alone isn't
// enough to reliably recover "true"/"false".
func fieldDisplayValue(f Field, v any) string {
	if f.Type != FieldCheckbox {
		return toString(v)
	}
	switch t := v.(type) {
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int64:
		if t != 0 {
			return "true"
		}
		return "false"
	case nil:
		return "false"
	default:
		return toString(v)
	}
}

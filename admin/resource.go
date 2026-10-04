package admin

// FieldType identifies how a Field is rendered in forms and how its value
// should be treated.
type FieldType string

const (
	FieldText     FieldType = "text"
	FieldTextarea FieldType = "textarea"
	FieldNumber   FieldType = "number"
	FieldEmail    FieldType = "email"
	FieldPassword FieldType = "password"
	FieldDate     FieldType = "date"
	FieldCheckbox FieldType = "checkbox"
	FieldSelect   FieldType = "select"
)

// Option is one choice in a FieldSelect field.
type Option struct {
	Value string
	Label string
}

// Field describes one attribute of a Resource's data model: its type, label,
// validation, and whether it participates in the grid and/or the forms. This
// is the "custom data model" a user plugs in to get a full CRUD UI.
type Field struct {
	Name       string
	Label      string
	Type       FieldType
	Required   bool
	Options    []Option // used when Type == FieldSelect
	HideInGrid bool     // default: shown as a data-grid column
	HideInForm bool     // default: shown in create/edit forms
}

// Record is a generic row of data keyed by field name. A Record returned by
// a DataProvider's List/Get/Create/Update must carry an "id" key.
type Record map[string]any

// ID returns the record's "id" value as a string, or "" if absent.
func (r Record) ID() string {
	if v, ok := r["id"]; ok {
		return toString(v)
	}
	return ""
}

// ListResult is the page of records returned by DataProvider.List, plus the
// total record count across all pages (used to render Pagination).
type ListResult struct {
	Records []Record
	Total   int
}

// DataProvider is the one interface a user implements to back a Resource
// with any storage: a database, an external API, or (for demos/tests) the
// in-memory MemoryProvider in this package.
type DataProvider interface {
	List(page, pageSize int) (ListResult, error)
	Get(id string) (Record, error)
	Create(data Record) (Record, error)
	Update(id string, data Record) (Record, error)
	Delete(id string) error
}

// Resource ties a data model (Fields + Provider) to a URL slug and nav
// label. Registering one Resource with an App yields a full Data/Create/Edit
// CRUD UI for it.
type Resource struct {
	Key      string // URL slug, e.g. "todos"
	Label    string // nav label, e.g. "Todos"
	Fields   []Field
	Provider DataProvider
	PageSize int // defaults to 10 when <= 0
}

func (r *Resource) pageSize() int {
	if r.PageSize > 0 {
		return r.PageSize
	}
	return 10
}

// GridFields returns the fields shown as Data-page grid columns.
func (r *Resource) GridFields() []Field {
	out := make([]Field, 0, len(r.Fields))
	for _, f := range r.Fields {
		if !f.HideInGrid {
			out = append(out, f)
		}
	}
	return out
}

// FormFields returns the fields shown in the Create/Edit forms.
func (r *Resource) FormFields() []Field {
	out := make([]Field, 0, len(r.Fields))
	for _, f := range r.Fields {
		if !f.HideInForm {
			out = append(out, f)
		}
	}
	return out
}

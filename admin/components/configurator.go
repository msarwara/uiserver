package components

import (
	"fmt"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/chasefleming/elem-go/htmx"
)

// ResourceSummary is the subset of a resource shown in the Configurator's
// resource list.
type ResourceSummary struct {
	Key        string
	Label      string
	FieldCount int
}

// FieldRow is one editable row in the Configurator's field editor. Every
// input's name is prefixed with RowID so rows are self-contained: removing
// a row's <tr> client-side (via a no-op htmx delete) drops all of its
// values from the submitted form, with no positional array to misalign.
type FieldRow struct {
	RowID        string
	Original     string // the field's name when the page loaded; "" for a brand-new row
	Name         string
	Label        string
	Type         FormFieldType
	Required     bool
	HideInGrid   bool
	HideInForm   bool
	ReadOnlyName bool   // true for the "id" row: it's the SQL primary key
	OptionsText  string // Select options, one per line ("value:label" or just "label"); only used when Type is Select
}

// FieldTypeOptions lists every field type for the editor's Type <select>.
func FieldTypeOptions() []FormFieldOption {
	return []FormFieldOption{
		{Value: string(FormText), Label: "Text"},
		{Value: string(FormTextarea), Label: "Textarea"},
		{Value: string(FormNumber), Label: "Number"},
		{Value: string(FormEmail), Label: "Email"},
		{Value: string(FormPassword), Label: "Password"},
		{Value: string(FormDate), Label: "Date"},
		{Value: string(FormCheckbox), Label: "Checkbox"},
		{Value: string(FormSelect), Label: "Select"},
	}
}

// ResourceListPage renders the Configurator's landing page: every editable
// resource, plus an "Add new resource" button.
func ResourceListPage(resources []ResourceSummary) elem.Node {
	rows := elem.TransformEach(resources, func(r ResourceSummary) elem.Node {
		href := "/configurator/" + r.Key
		return elem.Tr(nil,
			elem.Td(nil, elem.Text(r.Label)),
			elem.Td(nil, elem.Text(fmt.Sprintf("%d", r.FieldCount))),
			elem.Td(attrs.Props{attrs.Class: "row-actions"},
				elem.Button(attrs.Props{
					attrs.Class:    "btn btn-sm",
					htmx.HXGet:     href,
					htmx.HXTarget:  "#" + ContentTargetID,
					htmx.HXPushURL: "true",
				}, elem.Text("Edit fields")),
				elem.Button(attrs.Props{
					attrs.Class:   "btn btn-sm btn-danger",
					htmx.HXDelete: href,
					htmx.HXTarget: "body",
					htmx.HXConfirm: fmt.Sprintf(
						"Delete %q and all of its data? This cannot be undone.", r.Label),
				}, elem.Text("Delete")),
			),
		)
	})

	newHref := "/configurator/new"
	return Card(
		elem.Div(attrs.Props{attrs.Class: "toolbar"},
			elem.H1(nil, elem.Text("Configurator")),
			elem.Button(attrs.Props{
				attrs.Class:    "btn",
				htmx.HXGet:     newHref,
				htmx.HXTarget:  "#" + ContentTargetID,
				htmx.HXPushURL: "true",
			}, elem.Text("Add new resource")),
		),
		elem.Table(nil,
			elem.THead(nil, elem.Tr(nil,
				elem.Th(nil, elem.Text("Resource")),
				elem.Th(nil, elem.Text("Fields")),
				elem.Th(attrs.Props{attrs.Style: "text-align:right"}, elem.Text("Actions")),
			)),
			elem.TBody(nil, rows...),
		),
	)
}

// NewResourceForm renders the "create a resource" form (Key + Label).
func NewResourceForm(errorMsg string) elem.Node {
	return Card(
		elem.H1(nil, elem.Text("New Resource")),
		Flash("error", errorMsg),
		elem.Form(attrs.Props{
			attrs.Class:   "stacked-form",
			htmx.HXPost:   "/configurator/new",
			htmx.HXTarget: "#" + ContentTargetID,
		},
			elem.Div(attrs.Props{attrs.Class: "form-field"},
				elem.Label(attrs.Props{attrs.For: "key"}, elem.Text("Key")),
				elem.Input(attrs.Props{attrs.ID: "key", attrs.Name: "key", attrs.Type: "text", attrs.Required: "true"}),
				elem.Small(nil, elem.Text("Lowercase letters, digits, underscores — used in the URL and as the table name.")),
			),
			elem.Div(attrs.Props{attrs.Class: "form-field"},
				elem.Label(attrs.Props{attrs.For: "label"}, elem.Text("Label")),
				elem.Input(attrs.Props{attrs.ID: "label", attrs.Name: "label", attrs.Type: "text", attrs.Required: "true"}),
			),
			elem.Div(attrs.Props{attrs.Class: "form-actions"},
				elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text("Create")),
				elem.A(attrs.Props{
					attrs.Class:    "btn btn-ghost",
					attrs.Href:     "/configurator",
					htmx.HXGet:     "/configurator",
					htmx.HXTarget:  "#" + ContentTargetID,
					htmx.HXPushURL: "true",
				}, elem.Text("Cancel")),
			),
		),
	)
}

// FieldEditorPage renders a resource's field editor: one editable row per
// field, an "Add field" button, and Save.
func FieldEditorPage(resource ResourceSummary, rows []FieldRow, errorMsg string) elem.Node {
	rowNodes := make([]elem.Node, 0, len(rows))
	for _, row := range rows {
		rowNodes = append(rowNodes, FieldEditorRow(row))
	}

	addRowHref := "/configurator/" + resource.Key + "/field-row"
	saveHref := "/configurator/" + resource.Key

	return Card(
		elem.H1(nil, elem.Text("Fields: "+resource.Label)),
		Flash("error", errorMsg),
		elem.Form(attrs.Props{
			htmx.HXPost:   saveHref,
			htmx.HXTarget: "#" + ContentTargetID,
		},
			elem.Table(nil,
				elem.THead(nil, elem.Tr(nil,
					elem.Th(nil, elem.Text("Name")),
					elem.Th(nil, elem.Text("Label")),
					elem.Th(nil, elem.Text("Type")),
					elem.Th(nil, elem.Text("Select options")),
					elem.Th(nil, elem.Text("Required")),
					elem.Th(nil, elem.Text("Hide in grid")),
					elem.Th(nil, elem.Text("Hide in form")),
					elem.Th(attrs.Props{attrs.Style: "text-align:right"}, elem.Text("")),
				)),
				elem.TBody(attrs.Props{attrs.ID: "field-rows"}, rowNodes...),
			),
			elem.Div(attrs.Props{attrs.Class: "form-actions"},
				elem.Button(attrs.Props{
					attrs.Type:    "button",
					attrs.Class:   "btn btn-ghost",
					htmx.HXPost:   addRowHref,
					htmx.HXTarget: "#field-rows",
					htmx.HXSwap:   "beforeend",
				}, elem.Text("Add field")),
			),
			elem.Div(attrs.Props{attrs.Class: "form-actions"},
				elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text("Save")),
				elem.A(attrs.Props{
					attrs.Class:    "btn btn-ghost",
					attrs.Href:     "/configurator",
					htmx.HXGet:     "/configurator",
					htmx.HXTarget:  "#" + ContentTargetID,
					htmx.HXPushURL: "true",
				}, elem.Text("Cancel")),
			),
		),
	)
}

// FieldEditorRow renders one self-contained field row (see FieldRow's doc).
// Exported so the "Add field" handler can return a single blank row
// fragment using the exact same markup as the full editor.
func FieldEditorRow(row FieldRow) elem.Node {
	prefix := "row__" + row.RowID + "__"

	nameCell := elem.Input(attrs.Props{attrs.Type: "text", attrs.Name: prefix + "name", attrs.Value: row.Name})
	var removeBtn elem.Node = elem.None()
	if row.ReadOnlyName {
		nameCell = elem.Input(attrs.Props{attrs.Type: "hidden", attrs.Name: prefix + "name", attrs.Value: row.Name})
		nameCell = elem.Div(nil, nameCell, elem.Span(nil, elem.Text(row.Name)))
	} else {
		removeBtn = elem.Button(attrs.Props{
			attrs.Type:    "button",
			attrs.Class:   "btn btn-sm btn-danger",
			htmx.HXDelete: "/configurator/_row",
			htmx.HXTarget: "closest tr",
			htmx.HXSwap:   "outerHTML",
		}, elem.Text("Remove"))
	}

	typeOptions := make([]elem.Node, 0, len(FieldTypeOptions()))
	for _, opt := range FieldTypeOptions() {
		optAttrs := attrs.Props{attrs.Value: opt.Value}
		if opt.Value == string(row.Type) {
			optAttrs[attrs.Selected] = "true"
		}
		typeOptions = append(typeOptions, elem.Option(optAttrs, elem.Text(opt.Label)))
	}

	optionsTextarea := elem.Textarea(attrs.Props{
		attrs.Name:        prefix + "options",
		attrs.Placeholder: "One per line:\nvalue:Label\nor just: Label",
		attrs.Class:       "options-textarea",
	}, elem.Text(row.OptionsText))

	// Only Select and Checkbox fields use Options; rendering the right
	// initial visibility server-side (rather than hiding it with JS after
	// the fact) avoids a flash of the textarea on every page load. A tiny
	// listener (see globalCSS's sibling script in layout.go) keeps it in
	// sync if the admin changes Type afterward, with no server round trip.
	optionsCellAttrs := attrs.Props{attrs.Class: "options-cell"}
	if row.Type != FormSelect && row.Type != FormCheckbox {
		optionsCellAttrs[attrs.Style] = "display:none"
	}

	return elem.Tr(nil,
		elem.Input(attrs.Props{attrs.Type: "hidden", attrs.Name: "row_ids[]", attrs.Value: row.RowID}),
		elem.Input(attrs.Props{attrs.Type: "hidden", attrs.Name: prefix + "original", attrs.Value: row.Original}),
		elem.Td(nil, nameCell),
		elem.Td(nil, elem.Input(attrs.Props{attrs.Type: "text", attrs.Name: prefix + "label", attrs.Value: row.Label})),
		elem.Td(nil, elem.Select(attrs.Props{attrs.Name: prefix + "type"}, typeOptions...)),
		elem.Td(optionsCellAttrs, optionsTextarea),
		elem.Td(nil, checkboxCell(prefix+"required", row.Required)),
		elem.Td(nil, checkboxCell(prefix+"hide_grid", row.HideInGrid)),
		elem.Td(nil, checkboxCell(prefix+"hide_form", row.HideInForm)),
		elem.Td(attrs.Props{attrs.Class: "row-actions"}, removeBtn),
	)
}

func checkboxCell(name string, checked bool) elem.Node {
	a := attrs.Props{attrs.Type: "checkbox", attrs.Name: name}
	if checked {
		a[attrs.Checked] = "true"
	}
	return elem.Input(a)
}

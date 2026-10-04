package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/chasefleming/elem-go/htmx"

	"uiserver/admin/components"
)

func (a *App) handleList(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := parsePage(r)
		result, err := res.Provider.List(page, res.pageSize())
		if err != nil {
			a.render(w, r, res.Key, components.Card(components.Flash("error", err.Error())))
			return
		}
		a.render(w, r, res.Key, dataPageContent(res, result, page))
	}
}

func (a *App) handleNew(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content := formPageContent(res, "New "+res.Label, "/r/"+res.Key, "post", nil, nil, "")
		a.render(w, r, res.Key, content)
	}
}

func (a *App) handleCreate(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			a.render(w, r, res.Key, components.Card(components.Flash("error", "Could not read form data.")))
			return
		}

		data, fieldErrs := parseFormRecord(res, r)
		if len(fieldErrs) > 0 {
			content := formPageContent(res, "New "+res.Label, "/r/"+res.Key, "post", data, fieldErrs, "")
			a.render(w, r, res.Key, content)
			return
		}

		if _, err := res.Provider.Create(data); err != nil {
			content := formPageContent(res, "New "+res.Label, "/r/"+res.Key, "post", data, nil, err.Error())
			a.render(w, r, res.Key, content)
			return
		}

		a.renderList(w, r, res, 1)
	}
}

func (a *App) handleEdit(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		rec, err := res.Provider.Get(id)
		if err != nil {
			a.render(w, r, res.Key, components.Card(components.Flash("error", "Record not found.")))
			return
		}
		action := fmt.Sprintf("/r/%s/%s", res.Key, id)
		content := formPageContent(res, "Edit "+res.Label, action, "put", rec, nil, "")
		a.render(w, r, res.Key, content)
	}
}

func (a *App) handleUpdate(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := r.ParseForm(); err != nil {
			a.render(w, r, res.Key, components.Card(components.Flash("error", "Could not read form data.")))
			return
		}

		action := fmt.Sprintf("/r/%s/%s", res.Key, id)
		data, fieldErrs := parseFormRecord(res, r)
		if len(fieldErrs) > 0 {
			content := formPageContent(res, "Edit "+res.Label, action, "put", data, fieldErrs, "")
			a.render(w, r, res.Key, content)
			return
		}

		if _, err := res.Provider.Update(id, data); err != nil {
			content := formPageContent(res, "Edit "+res.Label, action, "put", data, nil, err.Error())
			a.render(w, r, res.Key, content)
			return
		}

		a.renderList(w, r, res, 1)
	}
}

func (a *App) handleDelete(res *Resource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := res.Provider.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Empty 200 response: htmx swaps this in place of the row (hx-target
		// "closest tr", hx-swap "outerHTML"), which removes it from the table.
		w.WriteHeader(http.StatusOK)
	}
}

func (a *App) renderList(w http.ResponseWriter, r *http.Request, res *Resource, page int) {
	result, err := res.Provider.List(page, res.pageSize())
	if err != nil {
		a.render(w, r, res.Key, components.Card(components.Flash("error", err.Error())))
		return
	}
	a.render(w, r, res.Key, dataPageContent(res, result, page))
}

func parsePage(r *http.Request) int {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// parseFormRecord builds a Record from posted form values, validating
// Required fields. Checkbox fields are read as booleans (present=="on"
// when checked, absent when unchecked); everything else is read as a
// trimmed string.
func parseFormRecord(res *Resource, r *http.Request) (Record, map[string]string) {
	data := Record{}
	errs := map[string]string{}

	for _, f := range res.FormFields() {
		if f.Type == FieldCheckbox {
			data[f.Name] = r.FormValue(f.Name) == "on"
			continue
		}
		val := strings.TrimSpace(r.FormValue(f.Name))
		if f.Required && val == "" {
			errs[f.Name] = "This field is required."
		}
		data[f.Name] = val
	}
	return data, errs
}

func dataPageContent(res *Resource, result ListResult, page int) elem.Node {
	gridFields := toGridFields(res.GridFields())
	rows := make([]components.GridRow, 0, len(result.Records))
	for _, rec := range result.Records {
		rows = append(rows, recordToGridRow(res.GridFields(), rec))
	}

	newHref := "/r/" + res.Key + "/new"
	return components.Card(
		elem.Div(attrs.Props{attrs.Class: "toolbar"},
			elem.H1(nil, elem.Text(res.Label)),
			elem.A(attrs.Props{
				attrs.Class:    "btn",
				attrs.Href:     newHref,
				htmx.HXGet:     newHref,
				htmx.HXTarget:  "#" + components.ContentTargetID,
				htmx.HXPushURL: "true",
			}, elem.Text("New "+res.Label)),
		),
		components.DataTable(gridFields, rows, res.Key),
		components.Pagination(res.Key, page, res.pageSize(), result.Total),
	)
}

func formPageContent(res *Resource, title, action, method string, values Record, fieldErrs map[string]string, generalErr string) elem.Node {
	fields := buildFormFields(res.FormFields(), values, fieldErrs)

	children := []elem.Node{elem.H1(nil, elem.Text(title))}
	if generalErr != "" {
		children = append(children, components.Flash("error", generalErr))
	}
	children = append(children, components.FormCard(action, method, fields, "Save"))

	return components.Card(children...)
}

func toGridFields(fields []Field) []components.GridField {
	out := make([]components.GridField, 0, len(fields))
	for _, f := range fields {
		out = append(out, components.GridField{Name: f.Name, Label: f.Label})
	}
	return out
}

func recordToGridRow(fields []Field, rec Record) components.GridRow {
	values := make(map[string]string, len(fields))
	for _, f := range fields {
		values[f.Name] = toString(rec[f.Name])
	}
	return components.GridRow{ID: rec.ID(), Values: values}
}

func buildFormFields(fields []Field, values Record, fieldErrs map[string]string) []components.FormField {
	out := make([]components.FormField, 0, len(fields))
	for _, f := range fields {
		opts := make([]components.FormFieldOption, 0, len(f.Options))
		for _, o := range f.Options {
			opts = append(opts, components.FormFieldOption{Value: o.Value, Label: o.Label})
		}

		var value string
		if values != nil {
			value = toString(values[f.Name])
		}

		out = append(out, components.FormField{
			Name:     f.Name,
			Label:    f.Label,
			Type:     components.FormFieldType(f.Type),
			Required: f.Required,
			Options:  opts,
			Value:    value,
			Error:    fieldErrs[f.Name],
		})
	}
	return out
}

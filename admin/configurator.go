package admin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"uiserver/admin/components"
)

const configuratorKey = "configurator"

// registerConfigurator adds the built-in "Configurator" nav entry and
// routes. Only called when Config.DB is set — structural field/resource
// editing needs somewhere to persist to and a real table to run DDL
// against, both of which require a DB.
func (a *App) registerConfigurator() {
	a.mu.Lock()
	a.navOrder = append(a.navOrder, navEntry{key: configuratorKey, label: "Configurator", href: "/configurator"})
	a.mu.Unlock()

	a.mux.HandleFunc("GET /configurator", requireAuth(a.handleConfiguratorList))
	a.mux.HandleFunc("GET /configurator/new", requireAuth(a.handleConfiguratorNewForm))
	a.mux.HandleFunc("POST /configurator/new", requireAuth(a.handleConfiguratorCreate))
	a.mux.HandleFunc("DELETE /configurator/_row", requireAuth(a.handleConfiguratorRemoveRow))
	a.mux.HandleFunc("GET /configurator/{resource}", requireAuth(a.handleConfiguratorEdit))
	a.mux.HandleFunc("POST /configurator/{resource}/field-row", requireAuth(a.handleConfiguratorAddRow))
	a.mux.HandleFunc("POST /configurator/{resource}", requireAuth(a.handleConfiguratorSave))
	a.mux.HandleFunc("DELETE /configurator/{resource}", requireAuth(a.handleConfiguratorDelete))
}

func (a *App) handleConfiguratorList(w http.ResponseWriter, r *http.Request) {
	resources := a.sqlResources()
	summaries := make([]components.ResourceSummary, 0, len(resources))
	for _, res := range resources {
		summaries = append(summaries, components.ResourceSummary{
			Key: res.Key, Label: res.Label, FieldCount: len(res.AllFields()),
		})
	}
	a.render(w, r, configuratorKey, components.ResourceListPage(summaries))
}

func (a *App) handleConfiguratorNewForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, r, configuratorKey, components.NewResourceForm(""))
}

func (a *App) handleConfiguratorCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.render(w, r, configuratorKey, components.NewResourceForm("Could not read form data."))
		return
	}

	key := strings.TrimSpace(r.FormValue("key"))
	label := strings.TrimSpace(r.FormValue("label"))

	if err := validIdentifier(key); err != nil {
		a.render(w, r, configuratorKey, components.NewResourceForm(err.Error()))
		return
	}
	if label == "" {
		a.render(w, r, configuratorKey, components.NewResourceForm("Label is required."))
		return
	}
	if _, exists := a.getResource(key); exists {
		a.render(w, r, configuratorKey, components.NewResourceForm(fmt.Sprintf("Resource %q already exists.", key)))
		return
	}

	if err := a.RegisterResource(&Resource{Key: key, Label: label}); err != nil {
		a.render(w, r, configuratorKey, components.NewResourceForm(err.Error()))
		return
	}

	w.Header().Set("HX-Redirect", "/configurator/"+key)
	w.WriteHeader(http.StatusOK)
}

// sqlResourceFromPath resolves {resource} to a Configurator-editable
// resource: one that exists and is backed by the auto-provisioned
// SQLProvider (see the package doc's scope note — a resource with a custom
// Provider isn't guaranteed to tolerate structural changes).
func (a *App) sqlResourceFromPath(w http.ResponseWriter, r *http.Request) (*Resource, bool) {
	res, ok := a.getResource(r.PathValue("resource"))
	if !ok {
		http.NotFound(w, r)
		return nil, false
	}
	if _, isSQL := res.Provider.(*SQLProvider); !isSQL {
		a.render(w, r, configuratorKey, components.Card(
			components.Flash("error", "This resource has a custom data provider and can't be edited here."),
		))
		return nil, false
	}
	return res, true
}

func (a *App) handleConfiguratorEdit(w http.ResponseWriter, r *http.Request) {
	res, ok := a.sqlResourceFromPath(w, r)
	if !ok {
		return
	}
	content := components.FieldEditorPage(resourceSummary(res), fieldRowsFor(res.AllFields()), "")
	a.render(w, r, configuratorKey, content)
}

func (a *App) handleConfiguratorAddRow(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.sqlResourceFromPath(w, r); !ok {
		return
	}
	rowID, err := randomRowID()
	if err != nil {
		http.Error(w, "could not generate a row id", http.StatusInternalServerError)
		return
	}
	a.writeHTML(w, components.FieldEditorRow(components.FieldRow{RowID: rowID, Type: components.FormText}))
}

// handleConfiguratorRemoveRow is a pure UI no-op: the "Remove field" button
// swaps its own <tr> out for this empty response, which is enough to drop
// the row (and, since its hidden row_ids[] marker lives inside it, its
// values) from the editor before Save is ever submitted.
func (a *App) handleConfiguratorRemoveRow(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (a *App) handleConfiguratorSave(w http.ResponseWriter, r *http.Request) {
	res, ok := a.sqlResourceFromPath(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		content := components.FieldEditorPage(resourceSummary(res), fieldRowsFor(res.AllFields()), "Could not read form data.")
		a.render(w, r, configuratorKey, content)
		return
	}

	submitted, err := parseConfiguratorForm(r)
	if err != nil {
		content := components.FieldEditorPage(resourceSummary(res), rowsFromSubmission(submitted), err.Error())
		a.render(w, r, configuratorKey, content)
		return
	}

	if err := a.applyFieldChanges(res, submitted); err != nil {
		content := components.FieldEditorPage(resourceSummary(res), rowsFromSubmission(submitted), err.Error())
		a.render(w, r, configuratorKey, content)
		return
	}

	// Back to the resource list, per the Configurator's own "previous
	// screen" flow (same reasoning as handleConfiguratorCreate landing on
	// the new resource's editor instead of staying put: each action returns
	// to where you'd naturally go next).
	w.Header().Set("HX-Redirect", "/configurator")
	w.WriteHeader(http.StatusOK)
}

// handleConfiguratorDelete drops a resource entirely: its table, its
// persisted schema rows, and its live nav/route entry. There's no undo —
// the UI confirms via hx-confirm before this is ever called.
func (a *App) handleConfiguratorDelete(w http.ResponseWriter, r *http.Request) {
	res, ok := a.sqlResourceFromPath(w, r)
	if !ok {
		return
	}

	if err := a.deleteResource(res); err != nil {
		a.render(w, r, configuratorKey, components.Card(components.Flash("error", err.Error())))
		return
	}

	// Full redirect (not just an in-page swap) because the NavBar — outside
	// #content — also needs to drop this resource.
	w.Header().Set("HX-Redirect", "/configurator")
	w.WriteHeader(http.StatusOK)
}

func (a *App) deleteResource(res *Resource) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := dropResourceTable(tx, res.Key); err != nil {
		return err
	}
	if err := a.schema.deleteResource(tx, res.Key); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	a.removeResource(res.Key)
	return nil
}

func resourceSummary(res *Resource) components.ResourceSummary {
	return components.ResourceSummary{Key: res.Key, Label: res.Label, FieldCount: len(res.AllFields())}
}

func fieldRowsFor(fields []Field) []components.FieldRow {
	rows := make([]components.FieldRow, 0, len(fields))
	for _, f := range fields {
		rows = append(rows, components.FieldRow{
			RowID:        f.Name,
			Original:     f.Name,
			Name:         f.Name,
			Label:        f.Label,
			Type:         components.FormFieldType(f.Type),
			Required:     f.Required,
			HideInGrid:   f.HideInGrid,
			HideInForm:   f.HideInForm,
			ReadOnlyName: f.Name == "id",
			OptionsText:  serializeOptionsText(f.Options),
		})
	}
	return rows
}

func rowsFromSubmission(submitted []submittedField) []components.FieldRow {
	rows := make([]components.FieldRow, 0, len(submitted))
	for _, sf := range submitted {
		rows = append(rows, components.FieldRow{
			RowID:        sf.rowID,
			Original:     sf.Original,
			Name:         sf.Field.Name,
			Label:        sf.Field.Label,
			Type:         components.FormFieldType(sf.Field.Type),
			Required:     sf.Field.Required,
			HideInGrid:   sf.Field.HideInGrid,
			HideInForm:   sf.Field.HideInForm,
			ReadOnlyName: sf.Field.Name == "id",
			// The raw text the user submitted, not a re-serialization of
			// sf.Field.Options — keeps a validation-error redisplay exactly
			// as typed (e.g. a blank line or odd spacing isn't silently
			// cleaned up out from under them).
			OptionsText: sf.OptionsText,
		})
	}
	return rows
}

// submittedField is one row of the field editor as posted: Field is the
// row's new state, Original is the field's name when the page loaded ("" for
// a brand-new row), used to tell add/rename/keep apart.
type submittedField struct {
	rowID       string
	Original    string
	Field       Field
	OptionsText string
}

// serializeOptionsText renders a Field's Options back into the editor's
// one-per-line textarea format: "value:label", or just "value" when the
// value and label are identical (the common case for a simple dropdown).
func serializeOptionsText(opts []Option) string {
	lines := make([]string, 0, len(opts))
	for _, o := range opts {
		if o.Value == o.Label {
			lines = append(lines, o.Value)
		} else {
			lines = append(lines, o.Value+":"+o.Label)
		}
	}
	return strings.Join(lines, "\n")
}

// parseOptionsText reads the editor's one-per-line Select options textarea.
// Each non-blank line is either "value:label" or just "label" (value and
// label become the same string); blank lines are skipped.
func parseOptionsText(text string) []Option {
	var opts []Option
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if value, label, ok := strings.Cut(line, ":"); ok {
			value = strings.TrimSpace(value)
			label = strings.TrimSpace(label)
			if value == "" {
				continue
			}
			if label == "" {
				label = value
			}
			opts = append(opts, Option{Value: value, Label: label})
			continue
		}
		opts = append(opts, Option{Value: line, Label: line})
	}
	return opts
}

// parseConfiguratorForm reads the self-contained field rows described in
// components.FieldRow's doc comment: row_ids[] lists which rows survived to
// submission (a removed row's hidden marker goes with it), and each row's
// other values live under row__<rowid>__*.
func parseConfiguratorForm(r *http.Request) ([]submittedField, error) {
	rowIDs := r.PostForm["row_ids[]"]
	out := make([]submittedField, 0, len(rowIDs))
	seen := map[string]bool{}

	for _, rowID := range rowIDs {
		prefix := "row__" + rowID + "__"
		name := strings.TrimSpace(r.PostForm.Get(prefix + "name"))
		label := strings.TrimSpace(r.PostForm.Get(prefix + "label"))
		original := strings.TrimSpace(r.PostForm.Get(prefix + "original"))
		typ := FieldType(r.PostForm.Get(prefix + "type"))
		optionsText := r.PostForm.Get(prefix + "options")
		required := r.PostForm.Get(prefix+"required") == "on"
		hideGrid := r.PostForm.Get(prefix+"hide_grid") == "on"
		hideForm := r.PostForm.Get(prefix+"hide_form") == "on"

		if name == "" {
			return nil, fmt.Errorf("every field needs a name")
		}
		if name == "id" && original != "id" {
			return nil, fmt.Errorf("%q is a reserved field name", name)
		}
		if name != "id" {
			if err := validIdentifier(name); err != nil {
				return nil, fmt.Errorf("field %q: %w", name, err)
			}
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate field name %q", name)
		}
		seen[name] = true

		if label == "" {
			label = name
		}

		out = append(out, submittedField{
			rowID:       rowID,
			Original:    original,
			OptionsText: optionsText,
			Field: Field{
				Name: name, Label: label, Type: typ, Options: parseOptionsText(optionsText),
				Required: required, HideInGrid: hideGrid, HideInForm: hideForm,
			},
		})
	}
	return out, nil
}

// applyFieldChanges diffs submitted against res's current fields and runs
// the resulting ADD/RENAME/DROP COLUMN statements plus the schema-store
// update inside one transaction, only swapping res's live Fields in once
// that transaction has committed.
func (a *App) applyFieldChanges(res *Resource, submitted []submittedField) error {
	existingNames := map[string]bool{}
	for _, f := range res.AllFields() {
		existingNames[f.Name] = true
	}

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	kept := map[string]bool{}
	newFields := make([]Field, 0, len(submitted))

	for _, sf := range submitted {
		newFields = append(newFields, sf.Field)

		switch {
		case sf.Original == "":
			if err := addColumn(tx, res.Key, sf.Field); err != nil {
				return fmt.Errorf("add field %q: %w", sf.Field.Name, err)
			}
		case sf.Original != sf.Field.Name:
			if err := renameColumn(tx, res.Key, sf.Original, sf.Field.Name); err != nil {
				return fmt.Errorf("rename field %q: %w", sf.Original, err)
			}
			kept[sf.Original] = true
		default:
			kept[sf.Original] = true
		}
	}

	for name := range existingNames {
		if name == "id" || kept[name] {
			continue
		}
		if err := dropColumn(tx, res.Key, name); err != nil {
			return fmt.Errorf("remove field %q: %w", name, err)
		}
	}

	saved := &Resource{Key: res.Key, Label: res.Label, PageSize: res.PageSize, Fields: newFields}
	if err := a.schema.upsertResource(tx, saved, 0); err != nil {
		return fmt.Errorf("saving field definitions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	res.replaceFields(newFields)
	return nil
}

func randomRowID() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "new" + hex.EncodeToString(buf), nil
}

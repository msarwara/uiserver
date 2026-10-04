# uiserver

A generic admin-tool framework in Go, built on [elem-go](https://github.com/chasefleming/elem-go)
(type-safe HTML, no templates) and [htmx](https://htmx.org/) (server-rendered interactivity,
no SPA framework).

## Run

```
go run .
```

Then open http://localhost:8080 and sign in with `admin` / `admin`.

## Concepts

- **Master page** ([admin/components/layout.go](admin/components/layout.go)) — the HTML shell,
  nav bar, and login/logout control. It wraps a `#content` area that htmx swaps content pages
  into on navigation.
- **Content pages** are one of:
  - **Data page** — paged, tabular list of a Resource's records.
  - **Create / Edit page** — a form, posting via `hx-post`/`hx-put`.
  - **Generic page** — a blank canvas (`admin.GenericPage`) for anything else, e.g. the
    Dashboard in [main.go](main.go).
- **Resource** ([admin/resource.go](admin/resource.go)) is the generic piece: a `Field` list
  (name, label, type, validation) plus a `DataProvider` (List/Get/Create/Update/Delete).
  Registering a Resource gets you the full Data/Create/Edit UI with no page-specific HTML.
  [admin/memory_provider.go](admin/memory_provider.go) ships a generic in-memory
  `DataProvider` for demos/tests — swap in your own backed by a real database.

Every page is assembled from small, reusable elem-go render functions in
[admin/components/](admin/components/) (nav bar, data table, pagination, form fields) — the
"micro-frontend" pieces every page kind composes.

## Adding a resource

```go
app.RegisterResource(&admin.Resource{
    Key:   "todos",
    Label: "Todos",
    Fields: []admin.Field{
        {Name: "id", Label: "ID", HideInForm: true},
        {Name: "text", Label: "Text", Type: admin.FieldText, Required: true},
        {Name: "done", Label: "Done", Type: admin.FieldCheckbox},
    },
    Provider: admin.NewMemoryProvider(nil),
})
```

That's the entire API surface needed for a new entity's list/create/edit/delete UI. See
[main.go](main.go) for the full wiring, including the Dashboard `GenericPage`.

## Layout

```
main.go                       # demo wiring: Dashboard page + Todos resource
admin/
  resource.go                  # Field, FieldType, Record, DataProvider, Resource
  pages.go                     # GenericPage
  auth.go                      # Authenticator, sessions, login/logout
  app.go                       # App: route registry, full-page-vs-htmx-fragment rendering
  resource_handlers.go         # Data/Create/Edit/Delete handlers for Resources
  memory_provider.go           # generic in-memory DataProvider
  components/
    layout.go                   # MasterPage, NavBar, Flash, Card
    table.go                    # DataTable, Pagination
    form.go                     # FormCard, FormField
    login.go                    # LoginPage
```

## Notes

- Sessions are an in-memory map (lost on restart) — fine for a demo, swap for a real session
  store before production use.
- `admin.SimpleAuthenticator` is a fixed username/password map; implement `admin.Authenticator`
  against a real user store to replace it.

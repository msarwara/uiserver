# uiserver

A generic admin-tool framework in Go, built on [elem-go](https://github.com/chasefleming/elem-go)
(type-safe HTML, no templates), [htmx](https://htmx.org/) (server-rendered interactivity, no SPA
framework), and SQLite ([modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite), pure Go, no
CGO) for persistence.

## Run

```
go run .
```

Then open http://localhost:8080 and sign in with `admin` / `admin` (seeded on first run only —
change it under "Users & Permissions"). Data, resource schemas, and users/roles persist in
`uiserver.db` (gitignored) in the working directory.

## Concepts

- **Master page** ([admin/components/layout.go](admin/components/layout.go)) — the HTML shell,
  nav bar, and login/logout control. It wraps a `#content` area that htmx swaps content pages
  into on navigation. The nav only lists what the signed-in user has permission to see.
- **Content pages** are one of:
  - **Data page** — paged, tabular list of a Resource's records.
  - **Create / Edit page** — a form, posting via `hx-post`/`hx-put`.
  - **Generic page** — a blank canvas (`admin.GenericPage`) for anything else, e.g. the
    Dashboard in [main.go](main.go).
- **Resource** ([admin/resource.go](admin/resource.go)) is the generic piece: a `Field` list
  (name, label, type, validation) plus a `DataProvider` (List/Get/Create/Update/Delete).
  Registering a Resource gets you the full Data/Create/Edit UI with no page-specific HTML.
  Leave `Provider` nil to get an auto-provisioned `SQLProvider` — a real SQLite table — or
  implement `DataProvider` yourself for any other backing store.
- **Configurator** ([admin/configurator.go](admin/configurator.go)) — a built-in page for
  managing resources *from the running app*: pick a resource to rename, add, or remove its
  fields (a Select field's options are a one-per-line textarea — `value:label`, or just
  `label` when the two match), use "Add new resource" to create one from scratch, or delete
  one outright. Saves run as real `ALTER TABLE`/`CREATE TABLE`/`DROP TABLE` statements, so a
  field rename moves its data to the new column rather than losing it, and changes apply
  immediately — no restart.
- **Users & Permissions** ([admin/users.go](admin/users.go)) — role-based access control:
  **Admin** (everything, including this module), **Editor** (Configurator access, plus
  whatever resource permissions are granted — same as User by default), **User** (resource
  access only via explicit grants), or any custom role created here. Permissions are
  `view`/`create`/`edit`/`delete` grants per role per resource, with optional per-user
  overrides (grant *or* deny) on top of the role. Also issues API tokens per user.
- **Web API** ([admin/api.go](admin/api.go)) — every resource is also reachable at
  `/api/{resource}` as JSON, authenticated by an `Authorization: Bearer <token>` header
  (independent of the browser session) and governed by the exact same permission grants as
  the UI. List responses carry `fields` (name/label/type/required) and `paging`
  (page/pageSize/total/totalPages) alongside `data`, so a consumer can introspect and page
  without extra round-trips.

Every page is assembled from small, reusable elem-go render functions in
[admin/components/](admin/components/) (nav bar, data table, pagination, form fields, the
Configurator's field editor, the permission matrices) — the "micro-frontend" pieces every page
kind composes.

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
    Seed: []admin.Record{{"text": "Explore elem-go", "done": true}}, // only used the first time
})
```

That's the entire API surface needed for a new entity's list/create/edit/delete UI (both the
htmx pages and the JSON API) — and from here, the Configurator can reshape its fields and the
Users & Permissions module can control who's allowed to touch it, without touching this code
again. See [main.go](main.go) for the full wiring.

## Calling the Web API

Every resource is reachable as JSON at `/api/{resource}` — including ones that only exist
because they were built in the Configurator, with no Go code at all. Walking through a
`tasks` resource created entirely in the UI (Configurator → Add new resource → key `tasks`,
then add a `status` Select field with options `todo:To Do`, `in_progress:In Progress`,
`done:Done`):

**1. Issue a token.** Users & Permissions → pick a user → API tokens → Generate token. Copy
it now; only its hash is stored, so it can't be shown again.

```
export TOKEN=<paste the token here>
```

**2. List, with paging.** The response's `fields` describe the resource's current shape
(whatever the Configurator last saved it as) and `paging` carries everything needed to fetch
the next page without a second request to find out how many there are:

```
curl -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/tasks?page=1&pageSize=10"
```
```json
{
  "resource": "tasks",
  "fields": [
    {"name": "id", "label": "ID", "type": "", "required": false},
    {"name": "status", "label": "Status", "type": "select", "required": false}
  ],
  "data": [
    {"id": 1, "status": "todo"}
  ],
  "paging": {"page": 1, "pageSize": 10, "total": 1, "totalPages": 1}
}
```

**3. Create a record** — a JSON body, same Required validation as the htmx form, and the
same `view`/`create`/`edit`/`delete` permission grants as the UI (a token issued to a user
whose role/overrides don't grant `create` on `tasks` gets a 403 here, same as clicking "New
Task" would):

```
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"status":"in_progress"}' \
  http://localhost:8080/api/tasks
```
```json
{
  "resource": "tasks",
  "fields": [ ... ],
  "data": {"id": 2, "status": "in_progress"}
}
```

**4. Update and delete**, by id:

```
curl -X PUT -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"status":"done"}' \
  http://localhost:8080/api/tasks/2

curl -X DELETE -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/tasks/2
# {"resource":"tasks","deleted":true,"id":"2"}
```

A validation failure (missing a Required field) comes back as `400` with per-field detail
instead of a generic error:

```json
{"error": "validation failed", "fieldErrors": {"text": "This field is required."}}
```

A token inherits exactly its user's effective permissions (role grants + that user's
overrides) — add a `status` field, rename it, or create the whole `tasks` resource tomorrow
and every token immediately sees the new shape; nothing about the API is regenerated or
needs redeploying.

## Layout

```
main.go                       # demo wiring: opens uiserver.db, Dashboard page, Todos resource
admin/
  resource.go                  # Field, FieldType, Record, DataProvider, Resource
  pages.go                     # GenericPage
  auth.go                      # Authenticator, sessions, login/logout, User
  rbac.go                      # Role
  rbac_store.go                # roles/users/permission-grants/API-tokens persistence
  permissions.go                # Action, hasPermission, requirePermission middleware
  app.go                       # App: route registry, full-page-vs-htmx-fragment rendering
  resource_handlers.go         # Data/Create/Edit/Delete handlers for Resources
  sql_provider.go              # SQLProvider (auto-provisioned DataProvider) + DDL helpers
  schema_store.go              # persists resource/field definitions (admin_resources/admin_fields)
  configurator.go              # the runtime resource/field editor
  users.go                     # Users & Permissions wizard handlers
  api.go                       # generic token-authenticated JSON REST API
  identifier.go                # table/column-name validation (SQL-injection guard)
  memory_provider.go           # generic in-memory DataProvider (no DB needed)
  components/
    layout.go                   # MasterPage, NavBar, Flash, Card
    table.go                    # DataTable, Pagination
    form.go                     # FormCard, FormField
    configurator.go              # ResourceListPage, FieldEditorPage, NewResourceForm
    users.go                     # Role/User list & forms, permission matrices, token list
    login.go                    # LoginPage
```

## Notes

- Sessions are an in-memory map (lost on restart) — fine for a demo, swap for a real session
  store before production use. API tokens, by contrast, are persisted (hashed) in SQLite.
- `admin.SimpleAuthenticator` only applies when `Config.DB` is nil (no RBAC) — once a DB is
  set, login goes through the DB-backed Users table instead and `Authenticator` is ignored.
- The Configurator's structural editing (add/rename/remove field, add resource) and the
  permission matrix both only apply to resources backed by the auto-provisioned
  `SQLProvider` — a resource registered with a custom `Provider` is left alone/always allowed,
  since a hand-written provider isn't guaranteed to tolerate schema changes made out from
  under it.
- The seeded `Admin` role can't be deleted or edited, a role can't be deleted while any user
  still has it, and the last Admin-role user can't be deleted — all to avoid locking everyone
  out.
- `db.SetMaxOpenConns(1)` in [main.go](main.go) avoids SQLite "database is locked" errors under
  concurrent requests; WAL mode is the next tuning step if this ever becomes a bottleneck.

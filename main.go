package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	_ "modernc.org/sqlite"

	"uiserver/admin"
)

func main() {
	db, err := sql.Open("sqlite", "uiserver.db")
	if err != nil {
		log.Fatalf("opening uiserver.db: %v", err)
	}
	defer db.Close()
	// SQLite allows one writer at a time; capping the pool at one
	// connection avoids spurious "database is locked" errors under
	// concurrent requests. Fine for an admin tool; WAL mode is the next
	// tuning step if this ever becomes a bottleneck.
	db.SetMaxOpenConns(1)

	// Config.Authenticator is ignored once a DB is configured: roles have
	// to live somewhere, and that's the DB-backed Users table (seeded with
	// admin/admin on first run — change it via Users & Permissions).
	app, err := admin.New(admin.Config{
		Title: "uiserver Admin",
		DB:    db,
	})
	if err != nil {
		log.Fatalf("initializing admin app: %v", err)
	}
	log.Println(`default login (seeded on first run only) is admin/admin — change it under "Users & Permissions"`)

	app.RegisterPage(&admin.GenericPage{
		Key:    "dashboard",
		Label:  "Dashboard",
		Render: dashboardContent,
	})

	if err := app.RegisterResource(&admin.Resource{
		Key:   "todos",
		Label: "Todos",
		Fields: []admin.Field{
			{Name: "id", Label: "ID", HideInForm: true},
			{Name: "text", Label: "Text", Type: admin.FieldText, Required: true},
			{Name: "done", Label: "Done", Type: admin.FieldCheckbox},
			{Name: "completedon", Label: "When", Type: admin.FieldDate},
		},
		Seed: seedTodos(),
	}); err != nil {
		log.Fatalf("registering todos resource: %v", err)
	}

	addr := ":8080"
	log.Printf("uiserver listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, app.Handler()))
}

// dashboardContent is a GenericPage: a blank canvas for anything that isn't
// a standard resource CRUD view.
func dashboardContent(r *http.Request) elem.Node {
	return elem.Div(attrs.Props{attrs.Class: "card"},
		elem.H1(nil, elem.Text("Welcome to uiserver Admin")),
		elem.P(nil, elem.Text("This dashboard is a GenericPage: a blank-canvas content page rendered in pure Go with elem-go.")),
		elem.P(nil, elem.Text("The Todos nav item is a Resource: its Data/Create/Edit UI is generated from a Field list and a DataProvider, with no page-specific HTML written by hand. Open the Configurator to rename, add, or remove its fields at runtime.")),
	)
}

// seedTodos gives the Todos resource enough demo rows to show paging in
// action (PageSize defaults to 10). Only used the first time the todos
// table is created — later runs reuse whatever's already in uiserver.db.
func seedTodos() []admin.Record {
	seed := []admin.Record{
		{"text": "Explore elem-go", "done": true},
	}
	for i := 1; i <= 12; i++ {
		seed = append(seed, admin.Record{
			"text": fmt.Sprintf("Sample todo item %d", i),
			"done": false,
		})
	}
	return seed
}

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"uiserver/admin"
)

func main() {
	app := admin.New(admin.Config{
		Title:         "uiserver Admin",
		Authenticator: admin.SimpleAuthenticator{"admin": "admin"},
	})

	app.RegisterPage(&admin.GenericPage{
		Key:    "dashboard",
		Label:  "Dashboard",
		Render: dashboardContent,
	})

	app.RegisterResource(&admin.Resource{
		Key:   "todos",
		Label: "Todos",
		Fields: []admin.Field{
			{Name: "id", Label: "ID", HideInForm: true},
			{Name: "text", Label: "Text", Type: admin.FieldText, Required: true},
			{Name: "done", Label: "Done", Type: admin.FieldCheckbox},
			{Name: "completedon", Label: "When", Type: admin.FieldDate},
		},
		Provider: admin.NewMemoryProvider(seedTodos()),
	})

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
		elem.P(nil, elem.Text("The Todos nav item is a Resource: its Data/Create/Edit UI is generated from a Field list and a DataProvider, with no page-specific HTML written by hand.")),
	)
}

// seedTodos gives the Todos resource enough demo rows to show paging in
// action (PageSize defaults to 10).
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

package admin

import (
	"net/http"

	"github.com/chasefleming/elem-go"
)

// GenericPage is a blank-canvas content page: anything that isn't a
// standard Data/Create/Edit CRUD view for a Resource. Register one to add
// arbitrary custom UI (a dashboard, a report, settings, ...) to the nav.
type GenericPage struct {
	Key    string
	Label  string
	Render func(r *http.Request) elem.Node
}

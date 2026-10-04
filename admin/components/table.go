package components

import (
	"fmt"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/chasefleming/elem-go/htmx"
)

// GridField is the subset of a Resource field needed to render a data-grid
// column, kept here (rather than importing the admin package) so this
// package has no dependency on admin — it only knows about plain data.
type GridField struct {
	Name  string
	Label string
}

// GridRow is one record's values, keyed by field name, plus its id.
type GridRow struct {
	ID     string
	Values map[string]string
}

// DataTable renders a record list as an HTML table with per-row Edit/Delete
// actions, wired up with htmx: Edit loads the edit form into the content
// area, Delete removes just that row in place.
func DataTable(fields []GridField, rows []GridRow, resourceKey string) elem.Node {
	headerCells := make([]elem.Node, 0, len(fields)+1)
	for _, f := range fields {
		headerCells = append(headerCells, elem.Th(nil, elem.Text(f.Label)))
	}
	headerCells = append(headerCells, elem.Th(attrs.Props{attrs.Style: "text-align:right"}, elem.Text("Actions")))

	bodyRows := elem.TransformEach(rows, func(row GridRow) elem.Node {
		cells := make([]elem.Node, 0, len(fields)+1)
		for _, f := range fields {
			cells = append(cells, elem.Td(nil, elem.Text(row.Values[f.Name])))
		}

		editHref := fmt.Sprintf("/r/%s/%s/edit", resourceKey, row.ID)
		deleteHref := fmt.Sprintf("/r/%s/%s", resourceKey, row.ID)

		cells = append(cells, elem.Td(attrs.Props{attrs.Class: "row-actions"},
			elem.Button(attrs.Props{
				attrs.Class:    "btn btn-sm",
				htmx.HXGet:     editHref,
				htmx.HXTarget:  "#" + ContentTargetID,
				htmx.HXPushURL: "true",
			}, elem.Text("Edit")),
			elem.Button(attrs.Props{
				attrs.Class:    "btn btn-sm btn-danger",
				htmx.HXDelete:  deleteHref,
				htmx.HXTarget:  "closest tr",
				htmx.HXSwap:    "outerHTML",
				htmx.HXConfirm: "Delete this record?",
			}, elem.Text("Delete")),
		))

		return elem.Tr(nil, cells...)
	})

	return elem.Table(nil,
		elem.THead(nil, elem.Tr(nil, headerCells...)),
		elem.TBody(attrs.Props{attrs.ID: "data-rows"}, bodyRows...),
	)
}

// Pagination renders Prev/Next controls for a resource's Data page. Both
// buttons re-fetch the Data page (same handler as the nav link) with a new
// ?page= value and swap it into the content area, keeping the browser URL
// in sync via hx-push-url.
func Pagination(resourceKey string, page, pageSize, total int) elem.Node {
	if total <= pageSize {
		return elem.None()
	}

	lastPage := (total + pageSize - 1) / pageSize
	prevHref := fmt.Sprintf("/r/%s?page=%d", resourceKey, page-1)
	nextHref := fmt.Sprintf("/r/%s?page=%d", resourceKey, page+1)

	prevAttrs := attrs.Props{attrs.Class: "btn btn-sm", htmx.HXGet: prevHref, htmx.HXTarget: "#" + ContentTargetID, htmx.HXPushURL: "true"}
	nextAttrs := attrs.Props{attrs.Class: "btn btn-sm", htmx.HXGet: nextHref, htmx.HXTarget: "#" + ContentTargetID, htmx.HXPushURL: "true"}
	if page <= 1 {
		prevAttrs[attrs.Disabled] = "true"
	}
	if page >= lastPage {
		nextAttrs[attrs.Disabled] = "true"
	}

	return elem.Div(attrs.Props{attrs.Class: "pagination"},
		elem.Button(prevAttrs, elem.Text("Prev")),
		elem.Span(nil, elem.Text(fmt.Sprintf("Page %d of %d", page, lastPage))),
		elem.Button(nextAttrs, elem.Text("Next")),
	)
}

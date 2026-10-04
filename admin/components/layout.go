// Package components holds the small, reusable elem-go rendering functions
// ("micro-frontends") that every admin page is composed from: the master
// page shell, the data grid, pagination, and forms.
package components

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/chasefleming/elem-go/htmx"
)

// NavItem is one entry in the master page's navigation bar.
type NavItem struct {
	Key   string // matches the active content page's key, for highlighting
	Label string
	Href  string
}

// ContentTargetID is the DOM id of the master page's content area that
// htmx swaps content pages into.
const ContentTargetID = "content"

// MasterPage renders the full document shell: nav + login/logout + the
// content area, used for direct navigation/page refreshes. htmx fragment
// responses (HX-Request requests) render just the content, bypassing this.
func MasterPage(title string, nav []NavItem, activeKey string, username string, content elem.Node) elem.Node {
	return elem.Html(attrs.Props{attrs.Lang: "en"},
		elem.Head(nil,
			elem.Meta(attrs.Props{attrs.Charset: "utf-8"}),
			elem.Meta(attrs.Props{
				attrs.Name:    "viewport",
				attrs.Content: "width=device-width, initial-scale=1",
			}),
			elem.Title(nil, elem.Text(title)),
			elem.Script(attrs.Props{
				attrs.Src: "https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js",
			}),
			elem.Style(nil, elem.Text(globalCSS)),
		),
		elem.Body(nil,
			NavBar(nav, activeKey, username),
			elem.Main(attrs.Props{
				attrs.ID:    ContentTargetID,
				attrs.Class: "container",
			}, content),
		),
	)
}

// NavBar renders the top navigation shared across all pages: links to every
// registered resource/page (loaded into #content via htmx) plus the
// signed-in user and a logout control, or a login link when signed out.
func NavBar(nav []NavItem, activeKey string, username string) elem.Node {
	links := make([]elem.Node, 0, len(nav))
	for _, item := range nav {
		class := "nav-link"
		if item.Key == activeKey {
			class = "nav-link nav-link-active"
		}
		links = append(links, elem.A(attrs.Props{
			attrs.Href:     item.Href,
			attrs.Class:    class,
			htmx.HXGet:     item.Href,
			htmx.HXTarget:  "#" + ContentTargetID,
			htmx.HXPushURL: "true",
		}, elem.Text(item.Label)))
	}

	var authArea elem.Node
	if username != "" {
		authArea = elem.Div(attrs.Props{attrs.Class: "nav-auth"},
			elem.Span(attrs.Props{attrs.Class: "nav-user"}, elem.Text(username)),
			elem.Button(attrs.Props{
				attrs.Class:   "btn btn-ghost",
				htmx.HXPost:   "/logout",
				htmx.HXTarget: "body",
			}, elem.Text("Logout")),
		)
	} else {
		authArea = elem.A(attrs.Props{attrs.Class: "btn btn-ghost", attrs.Href: "/login"}, elem.Text("Login"))
	}

	return elem.Nav(attrs.Props{attrs.Class: "navbar"},
		elem.Div(attrs.Props{attrs.Class: "nav-links"}, links...),
		authArea,
	)
}

// Flash renders an inline status message. kind is "error" or "success".
func Flash(kind, message string) elem.Node {
	if message == "" {
		return elem.None()
	}
	return elem.Div(attrs.Props{attrs.Class: "flash flash-" + kind}, elem.Text(message))
}

// Card wraps content in the standard padded, bordered content box used by
// every page kind.
func Card(content ...elem.Node) elem.Node {
	return elem.Div(attrs.Props{attrs.Class: "card"}, content...)
}

const globalCSS = `
	* { box-sizing: border-box; }
	body {
		margin: 0;
		font-family: -apple-system, Segoe UI, Roboto, sans-serif;
		background: #f5f6f8;
		color: #212529;
	}
	.navbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		background: #343a40;
		padding: 0.75rem 2rem;
	}
	.nav-links { display: flex; gap: 1.5rem; }
	.nav-link { color: #ced4da; text-decoration: none; font-weight: 500; }
	.nav-link-active { color: #fff; }
	.nav-auth { display: flex; align-items: center; gap: 0.75rem; }
	.nav-user { color: #f8f9fa; font-weight: 500; }
	.container { max-width: 900px; margin: 2rem auto; padding: 0 1rem; }
	.card {
		background: #fff;
		border-radius: 8px;
		padding: 1.5rem;
		box-shadow: 0 1px 3px rgba(0,0,0,0.1);
		margin-bottom: 1rem;
	}
	.btn {
		display: inline-block;
		background: #4263eb;
		color: #fff;
		border: none;
		padding: 0.5rem 1rem;
		border-radius: 6px;
		cursor: pointer;
		text-decoration: none;
		font-size: 0.95rem;
	}
	.btn-ghost { background: transparent; border: 1px solid #868e96; color: #f8f9fa; }
	.btn-danger { background: #e03131; }
	.btn-sm { padding: 0.3rem 0.6rem; font-size: 0.85rem; }
	.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
	table { width: 100%; border-collapse: collapse; }
	th, td { text-align: left; padding: 0.6rem 0.5rem; border-bottom: 1px solid #e9ecef; }
	th { color: #495057; font-size: 0.85rem; text-transform: uppercase; }
	.row-actions { display: flex; gap: 0.5rem; justify-content: flex-end; }
	.pagination { display: flex; justify-content: center; gap: 1rem; align-items: center; margin-top: 1rem; }
	.flash { padding: 0.75rem 1rem; border-radius: 6px; margin-bottom: 1rem; }
	.flash-error { background: #ffe3e3; color: #c92a2a; }
	.flash-success { background: #d3f9d8; color: #2b8a3e; }
	form.stacked-form { display: flex; flex-direction: column; gap: 1rem; max-width: 480px; }
	.form-field { display: flex; flex-direction: column; gap: 0.3rem; }
	.form-field label { font-weight: 500; font-size: 0.9rem; }
	.form-field input[type=text], .form-field input[type=number], .form-field input[type=email],
	.form-field input[type=password], .form-field input[type=date], .form-field textarea, .form-field select {
		padding: 0.5rem;
		border: 1px solid #ced4da;
		border-radius: 6px;
		font-size: 0.95rem;
	}
	.form-field-checkbox { flex-direction: row; align-items: center; }
	.field-error { color: #c92a2a; font-size: 0.85rem; }
	.form-actions { display: flex; gap: 0.75rem; margin-top: 0.5rem; }
`

package components

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/chasefleming/elem-go/htmx"
)

// LoginPage renders the standalone sign-in form (not wrapped in MasterPage:
// there is no nav/content area to show until the user is authenticated).
func LoginPage(title, errorMsg string) elem.Node {
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
			elem.Style(nil, elem.Text(globalCSS+loginCSS)),
		),
		elem.Body(nil,
			elem.Div(attrs.Props{attrs.Class: "login-page"},
				Card(
					elem.H1(nil, elem.Text(title)),
					Flash("error", errorMsg),
					elem.Form(attrs.Props{
						attrs.Class:   "stacked-form",
						htmx.HXPost:   "/login",
						htmx.HXTarget: "body",
					},
						elem.Div(attrs.Props{attrs.Class: "form-field"},
							elem.Label(attrs.Props{attrs.For: "username"}, elem.Text("Username")),
							elem.Input(attrs.Props{
								attrs.ID: "username", attrs.Name: "username",
								attrs.Type: "text", attrs.Required: "true",
							}),
						),
						elem.Div(attrs.Props{attrs.Class: "form-field"},
							elem.Label(attrs.Props{attrs.For: "password"}, elem.Text("Password")),
							elem.Input(attrs.Props{
								attrs.ID: "password", attrs.Name: "password",
								attrs.Type: "password", attrs.Required: "true",
							}),
						),
						elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text("Sign in")),
					),
				),
			),
		),
	)
}

const loginCSS = `
	.login-page {
		display: flex;
		justify-content: center;
		align-items: center;
		min-height: 100vh;
	}
	.login-page .card { width: 320px; }
`

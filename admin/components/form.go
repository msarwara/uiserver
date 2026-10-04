package components

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
	"github.com/chasefleming/elem-go/htmx"
)

// FormFieldType mirrors admin.FieldType without importing the admin
// package, keeping components storage/business-logic free.
type FormFieldType string

const (
	FormText     FormFieldType = "text"
	FormTextarea FormFieldType = "textarea"
	FormNumber   FormFieldType = "number"
	FormEmail    FormFieldType = "email"
	FormPassword FormFieldType = "password"
	FormDate     FormFieldType = "date"
	FormCheckbox FormFieldType = "checkbox"
	FormSelect   FormFieldType = "select"
)

// FormFieldOption is one <option> for a FormSelect field.
type FormFieldOption struct {
	Value string
	Label string
}

// FormField describes one input to render in a FormCard.
type FormField struct {
	Name     string
	Label    string
	Type     FormFieldType
	Required bool
	Options  []FormFieldOption
	Value    string // current value, as a string ("true"/"false" for checkboxes)
	Error    string // validation error to show inline, if any
}

// FormCard renders a Create/Edit form: one <form> posting to action via
// htmx (method is "post" or "put"), swapping the response into the content
// area so a validation failure can re-render the same form with errors.
func FormCard(action, method string, fields []FormField, submitLabel string) elem.Node {
	inputs := make([]elem.Node, 0, len(fields)+1)
	for _, f := range fields {
		inputs = append(inputs, renderField(f))
	}
	inputs = append(inputs, elem.Div(attrs.Props{attrs.Class: "form-actions"},
		elem.Button(attrs.Props{attrs.Type: "submit", attrs.Class: "btn"}, elem.Text(submitLabel)),
		elem.A(attrs.Props{
			attrs.Class: "btn btn-ghost",
			attrs.Href:  "javascript:history.back()",
		}, elem.Text("Cancel")),
	))

	formAttrs := attrs.Props{
		attrs.Class:   "stacked-form",
		htmx.HXTarget: "#" + ContentTargetID,
	}
	if method == "put" {
		formAttrs[htmx.HXPut] = action
	} else {
		formAttrs[htmx.HXPost] = action
	}

	return elem.Form(formAttrs, inputs...)
}

func renderField(f FormField) elem.Node {
	id := "field-" + f.Name
	label := elem.Label(attrs.Props{attrs.For: id}, elem.Text(f.Label))

	var input elem.Node
	switch f.Type {
	case FormTextarea:
		input = elem.Textarea(attrs.Props{
			attrs.ID:   id,
			attrs.Name: f.Name,
		}, elem.Text(f.Value))
	case FormSelect:
		options := make([]elem.Node, 0, len(f.Options))
		for _, opt := range f.Options {
			optAttrs := attrs.Props{attrs.Value: opt.Value}
			if opt.Value == f.Value {
				optAttrs[attrs.Selected] = "true"
			}
			options = append(options, elem.Option(optAttrs, elem.Text(opt.Label)))
		}
		input = elem.Select(attrs.Props{attrs.ID: id, attrs.Name: f.Name}, options...)
	case FormCheckbox:
		checkboxAttrs := attrs.Props{
			attrs.ID:   id,
			attrs.Name: f.Name,
			attrs.Type: "checkbox",
		}
		if f.Value == "true" {
			checkboxAttrs[attrs.Checked] = "true"
		}
		return elem.Div(attrs.Props{attrs.Class: "form-field form-field-checkbox"},
			elem.Input(checkboxAttrs),
			label,
			errorNode(f.Error),
		)
	default:
		inputAttrs := attrs.Props{
			attrs.ID:    id,
			attrs.Name:  f.Name,
			attrs.Type:  string(f.Type),
			attrs.Value: f.Value,
		}
		if f.Required {
			inputAttrs[attrs.Required] = "true"
		}
		input = elem.Input(inputAttrs)
	}

	return elem.Div(attrs.Props{attrs.Class: "form-field"},
		label,
		input,
		errorNode(f.Error),
	)
}

func errorNode(msg string) elem.Node {
	if msg == "" {
		return elem.None()
	}
	return elem.Small(attrs.Props{attrs.Class: "field-error"}, elem.Text(msg))
}

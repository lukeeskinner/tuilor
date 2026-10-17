// Package app validates saved declarative application definitions and record values.
package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/lukeeskinner/tuilor/internal/capability"
)

type Definition struct {
	Version    int        `json:"version"`
	Name       string     `json:"name"`
	Collection Collection `json:"collection"`
	Page       Page       `json:"page"`
}
type Collection struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}
type Field struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Choices  []string `json:"choices"`
}
type Page struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	Components []Component `json:"components"`
}
type Component struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Text       string   `json:"text"`
	Collection string   `json:"collection"`
	Columns    []string `json:"columns"`
	Mode       string   `json:"mode"`
	Inputs     []Input  `json:"inputs"`
	Actions    []Action `json:"actions"`
}
type Input struct {
	ID    string `json:"id"`
	Field string `json:"field"`
}
type Action struct {
	ID         string    `json:"id"`
	Label      string    `json:"label"`
	Role       string    `json:"role"` // add, edit, primary, delete, submit
	Capability string    `json:"capability"`
	Target     string    `json:"target"` // none or selected
	Form       string    `json:"form"`
	Bindings   []Binding `json:"bindings"`
}
type Binding struct {
	Field   string          `json:"field"`
	Source  string          `json:"source"`
	Input   string          `json:"input"`
	Literal json.RawMessage `json:"literal"`
}

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func label(s string) bool {
	return strings.TrimSpace(s) != "" && len(s) <= 200 && strings.IndexFunc(s, unicode.IsControl) < 0
}
func Decode(data []byte, target any) error {
	if len(data) > 1024*1024 {
		return fmt.Errorf("definition exceeds 1 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}
func Parse(data []byte, catalog capability.Catalog) (Definition, error) {
	var d Definition
	if err := Decode(data, &d); err != nil {
		return d, err
	}
	return d, d.Validate(catalog)
}
func (d Definition) Field(id string) (Field, bool) {
	for _, f := range d.Collection.Fields {
		if f.ID == id {
			return f, true
		}
	}
	return Field{}, false
}
func (d Definition) Component(id string) (Component, bool) {
	for _, c := range d.Page.Components {
		if c.ID == id {
			return c, true
		}
	}
	return Component{}, false
}
func (d Definition) List() Component {
	for _, c := range d.Page.Components {
		if c.Type == "list" {
			return c
		}
	}
	return Component{}
}
func (c Component) Action(role string) (Action, bool) {
	for _, a := range c.Actions {
		if a.Role == role {
			return a, true
		}
	}
	return Action{}, false
}

func (d Definition) Validate(catalog capability.Catalog) error {
	if d.Version != 1 || !label(d.Name) || !idPattern.MatchString(d.Collection.ID) || !label(d.Collection.Name) || !idPattern.MatchString(d.Page.ID) || !label(d.Page.Title) {
		return fmt.Errorf("app requires version 1, valid IDs, and nonempty names/titles up to 200 bytes")
	}
	if len(d.Collection.Fields) < 1 || len(d.Collection.Fields) > 20 || len(d.Page.Components) < 1 || len(d.Page.Components) > 20 {
		return fmt.Errorf("support 1–20 fields and 1–20 components")
	}
	fields := map[string]bool{}
	for _, f := range d.Collection.Fields {
		if !idPattern.MatchString(f.ID) || fields[f.ID] || !label(f.Label) {
			return fmt.Errorf("invalid or duplicate collection field %q", f.ID)
		}
		fields[f.ID] = true
		switch f.Type {
		case "text", "boolean", "date":
			if len(f.Choices) != 0 {
				return fmt.Errorf("choices apply only to choice fields")
			}
		case "choice":
			if len(f.Choices) < 1 || len(f.Choices) > 20 {
				return fmt.Errorf("choice field %q needs 1–20 choices", f.ID)
			}
			seen := map[string]bool{}
			for _, v := range f.Choices {
				if !label(v) || seen[v] {
					return fmt.Errorf("invalid/duplicate choice in %q", f.ID)
				}
				seen[v] = true
			}
		default:
			return fmt.Errorf("unsupported field type %q", f.Type)
		}
	}
	ids := map[string]bool{}
	unique := func(id string) bool {
		if !idPattern.MatchString(id) || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	lists := 0
	for _, c := range d.Page.Components {
		if !unique(c.ID) {
			return fmt.Errorf("invalid or duplicate component ID %q", c.ID)
		}
		if len(c.Title) > 200 || len(c.Text) > 8192 || len(c.Inputs) > 20 || len(c.Actions) > 8 {
			return fmt.Errorf("component %q exceeds size limits", c.ID)
		}
		switch c.Type {
		case "text":
			if c.Collection != "" || len(c.Columns) > 0 || len(c.Inputs) > 0 || len(c.Actions) > 0 || c.Mode != "" {
				return fmt.Errorf("text component %q cannot contain controls", c.ID)
			}
		case "list":
			lists++
			if c.Collection != d.Collection.ID || len(c.Columns) < 1 || len(c.Columns) > len(fields) || c.Mode != "" || len(c.Inputs) > 0 || c.Text != "" {
				return fmt.Errorf("invalid list %q", c.ID)
			}
			seen := map[string]bool{}
			for _, col := range c.Columns {
				if !fields[col] || seen[col] {
					return fmt.Errorf("invalid/duplicate display column %q", col)
				}
				seen[col] = true
			}
		case "form":
			if c.Collection != d.Collection.ID || (c.Mode != "create" && c.Mode != "edit") || len(c.Inputs) < 1 || len(c.Columns) > 0 || c.Text != "" {
				return fmt.Errorf("invalid form %q", c.ID)
			}
			seen := map[string]bool{}
			for _, in := range c.Inputs {
				if !unique(in.ID) || !fields[in.Field] || seen[in.Field] {
					return fmt.Errorf("invalid form input %q", in.ID)
				}
				seen[in.Field] = true
			}
		case "button":
			if c.Collection != d.Collection.ID || len(c.Actions) != 1 || len(c.Inputs) > 0 || len(c.Columns) > 0 || c.Mode != "" || c.Text != "" {
				return fmt.Errorf("invalid action button %q", c.ID)
			}
		default:
			return fmt.Errorf("unsupported component %q", c.Type)
		}
	}
	if lists != 1 {
		return fmt.Errorf("exactly one record list is supported")
	}
	for _, c := range d.Page.Components {
		roles := map[string]bool{}
		for _, a := range c.Actions {
			if !unique(a.ID) || !label(a.Label) || roles[a.Role] {
				return fmt.Errorf("invalid/duplicate action %q", a.ID)
			}
			roles[a.Role] = true
			if err := d.validateAction(c, a, catalog); err != nil {
				return fmt.Errorf("action %q: %w", a.ID, err)
			}
		}
		if c.Type == "form" && (len(c.Actions) != 1 || !roles["submit"]) {
			return fmt.Errorf("form %q needs one submit action", c.ID)
		}
	}
	for _, role := range []string{"add", "edit", "delete"} {
		if _, ok := d.List().Action(role); !ok {
			return fmt.Errorf("list needs %s action", role)
		}
	}
	return nil
}

func (d Definition) validateAction(c Component, a Action, catalog capability.Catalog) error {
	if a.Role == "add" || a.Role == "edit" {
		form, ok := d.Component(a.Form)
		mode, target := "create", "none"
		if a.Role == "edit" {
			mode, target = "edit", "selected"
		}
		if (c.Type != "list" && c.Type != "button") || !ok || form.Type != "form" || form.Mode != mode || a.Capability != "" || a.Target != target || len(a.Bindings) > 0 {
			return fmt.Errorf("invalid form navigation")
		}
		return nil
	}
	desc, ok := catalog.Lookup(a.Capability)
	if !ok {
		return fmt.Errorf("unavailable capability %q", a.Capability)
	}
	if a.Form != "" {
		return fmt.Errorf("capability action cannot also navigate to a form")
	}
	switch a.Role {
	case "submit":
		operation, target := "insert", "none"
		if c.Mode == "edit" {
			operation, target = "update", "selected"
		}
		if c.Type != "form" || desc.Operation != operation || a.Target != target {
			return fmt.Errorf("submit does not match form mode")
		}
	case "primary":
		if (c.Type != "list" && c.Type != "button") || desc.Operation != "update" || a.Target != "selected" {
			return fmt.Errorf("primary action must update the selected record")
		}
	case "delete":
		if (c.Type != "list" && c.Type != "button") || desc.Operation != "delete" || a.Target != "selected" || len(a.Bindings) > 0 {
			return fmt.Errorf("delete must target the selected record without values")
		}
	default:
		return fmt.Errorf("unsupported action role %q", a.Role)
	}
	if desc.Operation == "delete" {
		return nil
	}
	if len(a.Bindings) == 0 || len(a.Bindings) > 20 {
		return fmt.Errorf("insert/update requires field bindings")
	}
	seen := map[string]bool{}
	for _, b := range a.Bindings {
		f, ok := d.Field(b.Field)
		if !ok || seen[b.Field] {
			return fmt.Errorf("invalid/duplicate binding field %q", b.Field)
		}
		seen[b.Field] = true
		switch b.Source {
		case "input":
			found := false
			for _, in := range c.Inputs {
				if in.ID == b.Input && in.Field == b.Field {
					found = true
				}
			}
			if !found || c.Type != "form" || (len(b.Literal) > 0 && string(b.Literal) != "null") {
				return fmt.Errorf("binding must reference the matching form input")
			}
		case "literal":
			var value any
			if b.Input != "" || len(b.Literal) == 0 || json.Unmarshal(b.Literal, &value) != nil {
				return fmt.Errorf("invalid literal binding")
			}
			if err := ValidateValue(f, value); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported binding source %q", b.Source)
		}
	}
	if desc.Operation == "insert" {
		for _, f := range d.Collection.Fields {
			if f.Required && (f.Type == "text" || f.Type == "date") && !seen[f.ID] {
				return fmt.Errorf("insert does not provide required field %q", f.ID)
			}
		}
	}
	if c.Type == "form" {
		for _, input := range c.Inputs {
			bound := false
			for _, binding := range a.Bindings {
				if binding.Source == "input" && binding.Input == input.ID {
					bound = true
				}
			}
			if !bound {
				return fmt.Errorf("form input %q is not bound to its submit action", input.ID)
			}
		}
	}
	return nil
}

func ValidateValue(f Field, value any) error {
	if f.Type == "boolean" {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be boolean", f.Label)
		}
		return nil
	}
	if value == nil && f.Type == "date" && !f.Required {
		return nil
	}
	text, ok := value.(string)
	if !ok || len(text) > 8192 {
		return fmt.Errorf("%s must be text up to 8192 bytes", f.Label)
	}
	if f.Required && strings.TrimSpace(text) == "" {
		return fmt.Errorf("%s is required", f.Label)
	}
	switch f.Type {
	case "choice":
		for _, v := range f.Choices {
			if text == v {
				return nil
			}
		}
		return fmt.Errorf("%s must match a declared choice", f.Label)
	case "date":
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return fmt.Errorf("%s must use YYYY-MM-DD", f.Label)
		}
	}
	return nil
}
func (d Definition) Defaults() map[string]any {
	values := map[string]any{}
	for _, f := range d.Collection.Fields {
		switch f.Type {
		case "boolean":
			values[f.ID] = false
		case "choice":
			values[f.ID] = f.Choices[0]
		case "date":
			values[f.ID] = nil
		default:
			values[f.ID] = ""
		}
	}
	return values
}
func (d Definition) ValidateValues(values map[string]any, partial bool) error {
	if !partial && len(values) != len(d.Collection.Fields) {
		return fmt.Errorf("record fields do not match collection")
	}
	for id, v := range values {
		f, ok := d.Field(id)
		if !ok {
			return fmt.Errorf("unknown field %q", id)
		}
		if err := ValidateValue(f, v); err != nil {
			return err
		}
	}
	return nil
}

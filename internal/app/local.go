package app

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed local-schema.json
var localSchemaJSON []byte

type Collection struct {
	ID     string            `json:"id"`
	Fields []CollectionField `json:"fields"`
}

type CollectionField struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// LocalAction is a closed operation over an app's own records, never code.
// Insert uses Bindings; update uses Values; delete uses neither.
type LocalAction struct {
	Kind       string        `json:"kind"`
	Collection string        `json:"collection"`
	Bindings   []Binding     `json:"bindings"`
	Values     []RecordValue `json:"values"`
}

func (a LocalAction) MarshalJSON() ([]byte, error) {
	type plain LocalAction
	if a.Bindings == nil {
		a.Bindings = []Binding{}
	}
	if a.Values == nil {
		a.Values = []RecordValue{}
	}
	return json.Marshal(plain(a))
}

type RecordValue struct {
	Field string `json:"field"`
	Value any    `json:"value"`
}

func LocalJSONSchema() map[string]any {
	var schema map[string]any
	if err := json.Unmarshal(localSchemaJSON, &schema); err != nil {
		panic("invalid embedded local schema: " + err.Error())
	}
	return schema
}

func (d Definition) Collection(id string) (Collection, bool) {
	for _, c := range d.Collections {
		if c.ID == id {
			return c, true
		}
	}
	return Collection{}, false
}

func (d Definition) validateLocal() error {
	if strings.TrimSpace(d.Name) == "" || len(d.Pages) == 0 || len(d.Collections) == 0 {
		return fmt.Errorf("local app requires a name, collections, and pages")
	}
	collections := map[string]bool{}
	for _, c := range d.Collections {
		if strings.TrimSpace(c.ID) == "" || collections[c.ID] || len(c.Fields) == 0 {
			return fmt.Errorf("invalid or duplicate collection %q", c.ID)
		}
		collections[c.ID] = true
		fields := map[string]bool{}
		for _, f := range c.Fields {
			if strings.TrimSpace(f.ID) == "" || fields[f.ID] || (f.Type != "text" && f.Type != "boolean") {
				return fmt.Errorf("collection %q: invalid or duplicate field %q", c.ID, f.ID)
			}
			fields[f.ID] = true
		}
	}
	pages := map[string]bool{}
	for _, p := range d.Pages {
		if strings.TrimSpace(p.ID) == "" || pages[p.ID] || strings.TrimSpace(p.Title) == "" || len(p.Components) == 0 {
			return fmt.Errorf("invalid or duplicate page %q", p.ID)
		}
		pages[p.ID] = true
		ids := map[string]bool{}
		for _, c := range p.Components {
			if c.Action != nil {
				return fmt.Errorf("local apps cannot contain MCP actions")
			}
			if c.Type == "text" {
				if strings.TrimSpace(c.Text) == "" {
					return fmt.Errorf("text must not be empty")
				}
				continue
			}
			if strings.TrimSpace(c.ID) == "" || ids[c.ID] || strings.TrimSpace(c.Title) == "" {
				return fmt.Errorf("invalid or duplicate component %q", c.ID)
			}
			ids[c.ID] = true
			switch c.Type {
			case "form":
				if c.LocalAction == nil || c.LocalAction.Kind != "insert" || len(c.Fields) == 0 {
					return fmt.Errorf("form %q requires inputs and an insert action", c.ID)
				}
				inputs := map[string]bool{}
				for _, f := range c.Fields {
					if strings.TrimSpace(f.ID) == "" || inputs[f.ID] || f.Type != "text_input" || strings.TrimSpace(f.Label) == "" {
						return fmt.Errorf("invalid form input %q", f.ID)
					}
					inputs[f.ID] = true
				}
				if err := d.ValidateLocalAction(*c.LocalAction, inputs); err != nil {
					return fmt.Errorf("form %q: %w", c.ID, err)
				}
			case "list":
				collection, ok := d.Collection(c.Collection)
				if !ok {
					return fmt.Errorf("list %q: unknown collection %q", c.ID, c.Collection)
				}
				types := map[string]string{}
				for _, f := range collection.Fields {
					types[f.ID] = f.Type
				}
				if types[c.LabelField] != "text" || (c.StatusField != "" && types[c.StatusField] != "boolean") {
					return fmt.Errorf("list %q: invalid label or status binding", c.ID)
				}
				kinds := map[string]bool{}
				for _, a := range c.Actions {
					if a.Collection != c.Collection || a.Kind == "insert" || kinds[a.Kind] {
						return fmt.Errorf("list %q: invalid or duplicate action", c.ID)
					}
					kinds[a.Kind] = true
					if err := d.ValidateLocalAction(a, nil); err != nil {
						return fmt.Errorf("list %q: %w", c.ID, err)
					}
				}
			default:
				return fmt.Errorf("unsupported local component %q", c.Type)
			}
		}
	}
	return nil
}

// ValidateLocalAction checks references and operation-specific bindings. The
// runtime calls it again before changing records with actual input values.
func (d Definition) ValidateLocalAction(a LocalAction, inputs map[string]bool) error {
	c, ok := d.Collection(a.Collection)
	if !ok {
		return fmt.Errorf("unknown collection %q", a.Collection)
	}
	types := map[string]string{}
	for _, f := range c.Fields {
		types[f.ID] = f.Type
	}
	switch a.Kind {
	case "insert":
		if len(a.Values) != 0 {
			return fmt.Errorf("insert cannot assign literal values")
		}
		bound, used := map[string]bool{}, map[string]bool{}
		for _, b := range a.Bindings {
			if types[b.Parameter] != "text" || !inputs[b.Field] || bound[b.Parameter] {
				return fmt.Errorf("invalid insert binding %q -> %q", b.Field, b.Parameter)
			}
			bound[b.Parameter], used[b.Field] = true, true
		}
		for name, kind := range types {
			if kind == "text" && !bound[name] {
				return fmt.Errorf("text field %q requires a binding", name)
			}
		}
		for name := range inputs {
			if !used[name] {
				return fmt.Errorf("input %q has no binding", name)
			}
		}
	case "update":
		if len(a.Bindings) != 0 || len(a.Values) == 0 {
			return fmt.Errorf("update requires literal values only")
		}
		seen := map[string]bool{}
		for _, v := range a.Values {
			if seen[v.Field] || !ValueMatches(types[v.Field], v.Value) {
				return fmt.Errorf("invalid update value for %q", v.Field)
			}
			seen[v.Field] = true
		}
	case "delete":
		if len(a.Bindings) != 0 || len(a.Values) != 0 {
			return fmt.Errorf("delete cannot contain bindings or values")
		}
	default:
		return fmt.Errorf("unsupported local action %q", a.Kind)
	}
	return nil
}

func ValueMatches(kind string, value any) bool {
	switch kind {
	case "text":
		v, ok := value.(string)
		return ok && strings.TrimSpace(v) != ""
	case "boolean":
		_, ok := value.(bool)
		return ok
	}
	return false
}

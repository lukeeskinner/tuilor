// Package app defines the declarative format shared by generation and a future runtime.
// Definitions describe intent; they do not grant permission to execute tools.
package app

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

//go:embed schema.json
var schemaJSON []byte

type Definition struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Pages   []Page `json:"pages"`
}

type Page struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	Components []Component `json:"components"`
}

// Component is a closed union: text uses Text; form uses ID, Title, Fields, Action.
type Component struct {
	Type   string  `json:"type"`
	Text   string  `json:"text,omitempty"`
	ID     string  `json:"id,omitempty"`
	Title  string  `json:"title,omitempty"`
	Fields []Field `json:"fields,omitempty"`
	Action *Action `json:"action,omitempty"`
}

// MarshalJSON retains required empty fields for forms bound to zero-argument tools.
func (c Component) MarshalJSON() ([]byte, error) {
	if c.Type == "form" {
		fields := c.Fields
		if fields == nil {
			fields = []Field{}
		}
		return json.Marshal(struct {
			Type   string  `json:"type"`
			ID     string  `json:"id"`
			Title  string  `json:"title"`
			Fields []Field `json:"fields"`
			Action *Action `json:"action"`
		}{c.Type, c.ID, c.Title, fields, c.Action})
	}
	type plain Component
	return json.Marshal(plain(c))
}

type Field struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label"`
}

type Action struct {
	Tool     string    `json:"tool"`
	Bindings []Binding `json:"bindings"`
}

type Binding struct {
	Parameter string `json:"parameter"`
	Field     string `json:"field"`
}

// Capability is validation data, independent of any MCP SDK or AI provider.
type Capability struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// JSONSchema returns an independent copy of the same schema used locally and by the AI.
func JSONSchema() map[string]any {
	var schema map[string]any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		panic("invalid embedded app schema: " + err.Error())
	}
	return schema
}

// Parse validates the complete JSON shape, including required fields and unknown
// properties, before decoding and checking references. No schema loader is used.
func Parse(data []byte, capabilities []Capability) (*Definition, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode app JSON: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return nil, fmt.Errorf("decode app schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("resolve app schema: %w", err)
	}
	if err := resolved.Validate(value); err != nil {
		return nil, fmt.Errorf("validate app structure: %w", err)
	}
	var definition Definition
	if err := json.Unmarshal(data, &definition); err != nil {
		return nil, fmt.Errorf("decode app definition: %w", err)
	}
	if err := definition.Validate(capabilities); err != nil {
		return nil, fmt.Errorf("validate app semantics: %w", err)
	}
	return &definition, nil
}

// Validate checks supported components and semantic references. Use Parse when
// loading JSON so missing properties and unknown keys are checked as well.
func (d Definition) Validate(capabilities []Capability) error {
	if d.Version != 1 || strings.TrimSpace(d.Name) == "" || len(d.Pages) == 0 {
		return fmt.Errorf("app requires version 1, a name, and at least one page")
	}
	tools := make(map[string]Capability, len(capabilities))
	for _, tool := range capabilities {
		if _, exists := tools[tool.Name]; exists || tool.Name == "" {
			return fmt.Errorf("empty or duplicate discovered tool name %q", tool.Name)
		}
		tools[tool.Name] = tool
	}
	pages := map[string]bool{}
	for _, page := range d.Pages {
		if strings.TrimSpace(page.ID) == "" || pages[page.ID] || strings.TrimSpace(page.Title) == "" || len(page.Components) == 0 {
			return fmt.Errorf("page %q requires a unique ID, title, and components", page.ID)
		}
		pages[page.ID] = true
		forms := map[string]bool{}
		for _, c := range page.Components {
			switch c.Type {
			case "text":
				if strings.TrimSpace(c.Text) == "" || c.ID != "" || c.Title != "" || c.Fields != nil || c.Action != nil {
					return fmt.Errorf("page %q: text requires only nonempty text", page.ID)
				}
			case "form":
				if strings.TrimSpace(c.ID) == "" || forms[c.ID] || strings.TrimSpace(c.Title) == "" || c.Action == nil || c.Text != "" {
					return fmt.Errorf("page %q: form %q requires unique ID, title and action", page.ID, c.ID)
				}
				forms[c.ID] = true
				tool, ok := tools[c.Action.Tool]
				if !ok {
					return fmt.Errorf("form %q: unknown MCP tool %q", c.ID, c.Action.Tool)
				}
				if err := validateForm(c, tool); err != nil {
					return fmt.Errorf("form %q: %w", c.ID, err)
				}
			default:
				return fmt.Errorf("unsupported component %q", c.Type)
			}
		}
	}
	return nil
}

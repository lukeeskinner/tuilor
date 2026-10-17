package codeapp

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

//go:embed runtime.go.txt
var runtimeSource string

func CanonicalSchema(raw string) (string, error) {
	if len(raw) == 0 || len(raw) > 64*1024 {
		return "", fmt.Errorf("a data schema up to 64 KiB is required")
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return "", fmt.Errorf("parse data schema: %w", err)
	}
	b, _ := json.Marshal(value)
	var schema jsonschema.Schema
	if err := json.Unmarshal(b, &schema); err != nil {
		return "", fmt.Errorf("invalid data schema: %w", err)
	}
	if _, err := schema.Resolve(&jsonschema.ResolveOptions{Loader: func(*url.URL) (*jsonschema.Schema, error) {
		return nil, fmt.Errorf("external schema references are unsupported")
	}}); err != nil {
		return "", fmt.Errorf("resolve data schema: %w", err)
	}
	// Reject external references even in currently unused schema branches.
	var inspect func(any) error
	inspect = func(v any) error {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if k == "$ref" || k == "$dynamicRef" {
					ref, ok := v.(string)
					if !ok || !strings.HasPrefix(ref, "#") {
						return fmt.Errorf("only local data-schema references are supported")
					}
				}
				if err := inspect(v); err != nil {
					return err
				}
			}
		case []any:
			for _, v := range x {
				if err := inspect(v); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := inspect(value); err != nil {
		return "", err
	}
	return string(b), nil
}
func ValidateState(schemaRaw string, data []byte) error {
	if len(data) > 4*1024*1024 {
		return fmt.Errorf("local state exceeds 4 MiB")
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(schemaRaw), &schema); err != nil {
		return err
	}
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{Loader: func(*url.URL) (*jsonschema.Schema, error) {
		return nil, fmt.Errorf("external schema references are unsupported")
	}})
	if err != nil {
		return err
	}
	var state any
	if err = json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("invalid local JSON; preserved")
	}
	if resolved.Validate(state) != nil {
		return fmt.Errorf("local data does not match its unchanged data schema; preserved")
	}
	return nil
}
func (p *Project) ValidateData() error {
	if p.Spec.DataSchema == "" {
		return nil
	}
	raw, err := readRegular(p.Dir + "/data/state.json")
	if err != nil {
		return err
	}
	return ValidateState(p.Spec.DataSchema, raw)
}
func (p *Project) starter() string {
	if p.Spec.DataSchema == "" {
		return Starter
	}
	return "package main\n\nfunc main() { hostMain() }\n"
}

func sameSchema(a, b string) bool {
	aa, e := CanonicalSchema(a)
	bb, f := CanonicalSchema(b)
	return e == nil && f == nil && bytes.Equal([]byte(aa), []byte(bb))
}
func (p *Project) sourceFiles() []string {
	names := []string{"manifest.json", "app.go", "main.go", "go.mod", "go.sum"}
	if p.Spec.DataSchema != "" {
		names = append(names, "runtime.go", "data.schema.json")
	}
	return names
}
func (p *Project) writeContract() error {
	if p.Spec.DataSchema == "" {
		return nil
	}
	if err := atomicFile(p.Dir+"/runtime.go", []byte(runtimeSource), 0600); err != nil {
		return err
	}
	return atomicFile(p.Dir+"/data.schema.json", []byte(p.Spec.DataSchema), 0600)
}
func (p *Project) initializeState() error {
	if p.Spec.DataSchema == "" {
		return nil
	}
	if _, err := os.Lstat(p.Dir + "/data/state.json"); os.IsNotExist(err) {
		return atomicFile(p.Dir+"/data/state.json", []byte(p.Spec.InitialState), 0600)
	}
	return p.ValidateData()
}

func (p *Project) contractFiles() []string {
	if p.Spec.DataSchema == "" {
		return nil
	}
	return []string{"runtime.go", "data.schema.json"}
}

package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

func supportedSchema(raw json.RawMessage, root bool) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, fmt.Errorf("unsupported input schema: expected an object")
	}
	for key := range obj {
		switch key {
		case "title", "description", "default", "examples", "deprecated", "readOnly", "writeOnly", "$schema", "type":
			continue
		}
		if root && (key == "properties" || key == "required" || key == "additionalProperties") {
			continue
		}
		if !root && (key == "enum" || key == "const" || key == "minLength" || key == "maxLength" || key == "pattern") {
			continue
		}
		return nil, fmt.Errorf("unsupported input schema keyword %q", key)
	}
	expected := "string"
	if root {
		expected = "object"
	}
	var kind string
	if err := json.Unmarshal(obj["type"], &kind); err != nil || kind != expected {
		return nil, fmt.Errorf("unsupported input schema: expected type %q", expected)
	}
	return obj, nil
}

func validateForm(form Component, tool Capability) error {
	root, err := supportedSchema(tool.InputSchema, true)
	if err != nil {
		return fmt.Errorf("tool %q: %w", tool.Name, err)
	}
	if additional, ok := root["additionalProperties"]; ok {
		var allowed bool
		if err := json.Unmarshal(additional, &allowed); err != nil || string(additional) == "null" {
			return fmt.Errorf("tool %q: unsupported additionalProperties schema", tool.Name)
		}
	}
	var properties map[string]json.RawMessage
	if p, ok := root["properties"]; ok {
		if err := json.Unmarshal(p, &properties); err != nil || properties == nil {
			return fmt.Errorf("tool %q: invalid properties", tool.Name)
		}
	}
	var required []string
	if p, ok := root["required"]; ok {
		if err := json.Unmarshal(p, &required); err != nil || required == nil {
			return fmt.Errorf("tool %q: invalid required parameters", tool.Name)
		}
	}
	// Validate schema syntax without a remote loader. Unknown keywords were rejected above.
	for name, raw := range properties {
		if _, err := supportedSchema(raw, false); err != nil {
			return fmt.Errorf("tool %q parameter %q: %w", tool.Name, name, err)
		}
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
		return fmt.Errorf("tool %q input schema: %w", tool.Name, err)
	}
	if _, err := schema.Resolve(nil); err != nil {
		return fmt.Errorf("tool %q input schema: %w", tool.Name, err)
	}
	fields := map[string]bool{}
	for _, field := range form.Fields {
		if strings.TrimSpace(field.ID) == "" || fields[field.ID] || field.Type != "text_input" || strings.TrimSpace(field.Label) == "" {
			return fmt.Errorf("invalid or duplicate text input %q", field.ID)
		}
		fields[field.ID] = true
	}
	bound := map[string]bool{}
	for _, binding := range form.Action.Bindings {
		if !fields[binding.Field] {
			return fmt.Errorf("binding references missing field %q", binding.Field)
		}
		if _, ok := properties[binding.Parameter]; !ok {
			return fmt.Errorf("tool %q has no parameter %q", tool.Name, binding.Parameter)
		}
		if bound[binding.Parameter] {
			return fmt.Errorf("duplicate binding for parameter %q", binding.Parameter)
		}
		bound[binding.Parameter] = true
	}
	for _, name := range required {
		if !bound[name] {
			return fmt.Errorf("tool %q required parameter %q has no binding", tool.Name, name)
		}
	}
	return nil
}

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func greeting(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../examples/greeting.json")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func capability(schema string) []Capability {
	return []Capability{{Name: "greet", InputSchema: json.RawMessage(schema)}}
}

const greetSchema = `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"],"additionalProperties":false}`

func TestParseAndSave(t *testing.T) {
	d, err := Parse(greeting(t), capability(greetSchema))
	if err != nil {
		t.Fatal(err)
	}
	d.Name = "../../do-not-use-as-filename"
	dir := filepath.Join(t.TempDir(), "apps")
	first, err := Save(d, dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Save(d, dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || filepath.Dir(first) != dir {
		t.Fatalf("unexpected paths %q, %q", first, second)
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data, capability(greetSchema)); err != nil {
		t.Fatalf("saved definition: %v", err)
	}
	info, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %v", info.Mode())
	}
	if _, err := Save(d, first); err == nil {
		t.Fatal("expected directory error")
	}
}

func TestRejectDefinitions(t *testing.T) {
	base := string(greeting(t))
	tests := []struct{ name, old, replacement, want string }{
		{"invalid JSON", base, `{`, "decode app JSON"},
		{"trailing JSON", base, base + ` {}`, "decode app JSON"},
		{"unknown top property", `"version": 1`, `"command":"sh", "version":1`, "structure"},
		{"missing version", `"version": 1,`, ``, "structure"},
		{"unsupported version", `"version": 1`, `"version": 2`, "structure"},
		{"empty pages", base, `{"version":1,"name":"App","pages":[]}`, "structure"},
		{"null pages", base, `{"version":1,"name":"App","pages":null}`, "structure"},
		{"unsupported component", `"type": "text"`, `"type":"shell"`, "structure"},
		{"unsupported field", `"text_input"`, `"number_input"`, "structure"},
		{"nonexistent tool", `"tool": "greet"`, `"tool":"delete_everything"`, "unknown MCP tool"},
		{"unknown parameter", `"parameter": "name"`, `"parameter":"password"`, "no parameter"},
		{"unknown field", `"field": "person"`, `"field":"elsewhere"`, "missing field"},
		{"missing required binding", `[{"parameter": "name", "field": "person"}]`, `[]`, "has no binding"},
		{"duplicate binding", `[{"parameter": "name", "field": "person"}]`, `[{"parameter":"name","field":"person"},{"parameter":"name","field":"person"}]`, "duplicate binding"},
		{"duplicate field", `[{"id": "person", "type": "text_input", "label": "Your name"}]`, `[{"id":"person","type":"text_input","label":"Name"},{"id":"person","type":"text_input","label":"Name"}]`, "duplicate text input"},
		{"blank name", `"Greeting App"`, `"   "`, "app requires"},
		{"blank title", `"title": "Greeting"`, `"title":" "`, "requires a unique ID"},
		{"unknown action property", `"tool": "greet"`, `"execute":true, "tool":"greet"`, "structure"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := strings.Replace(base, tt.old, tt.replacement, 1)
			if data == base {
				t.Fatal("test did not modify fixture")
			}
			_, err := Parse([]byte(data), capability(greetSchema))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v; want %q", err, tt.want)
			}
		})
	}
}

func TestInputSchemas(t *testing.T) {
	for _, schema := range []string{
		`{`, `true`, `null`,
		`{"type":"object","properties":{"name":{"type":"integer"}},"required":["name"]}`,
		`{"type":"object","properties":{"name":{"type":"object"}}}`,
		`{"type":"object","properties":{"name":{"$ref":"https://example.com/schema"}}}`,
		`{"type":"object","allOf":[],"properties":{"name":{"type":"string"}}}`,
		`{"type":"object","properties":{"name":{"type":"string","format":"email"}}}`,
		`{"type":"object","properties":{"name":{"type":"string","pattern":"["}}}`,
		`{"type":"object","properties":{"name":{"type":"string"}},"additionalProperties":{}}`,
		`{"type":"object","properties":{"name":{"type":"string"}},"required":["missing"]}`,
	} {
		t.Run(schema, func(t *testing.T) {
			if _, err := Parse(greeting(t), capability(schema)); err == nil {
				t.Fatal("unsupported schema accepted")
			}
		})
	}
	if _, err := Parse(greeting(t), capability(`{"type":"object","properties":{"name":{"type":"string","minLength":1,"maxLength":40,"pattern":"^[A-Z]","enum":["Ada"]}},"required":["name"]}`)); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyAndDuplicateCapabilities(t *testing.T) {
	if _, err := Parse(greeting(t), nil); err == nil {
		t.Fatal("unknown tool accepted")
	}
	caps := capability(greetSchema)
	caps = append(caps, caps[0])
	if _, err := Parse(greeting(t), caps); err == nil {
		t.Fatal("duplicate capability accepted")
	}
	if _, err := Parse([]byte(`{"version":1,"name":"Info","pages":[{"id":"home","title":"Info","components":[{"type":"text","text":"No tools available"}]}]}`), nil); err != nil {
		t.Fatal(err)
	}
}

func TestZeroArgumentFormRoundTrip(t *testing.T) {
	data := []byte(`{"version":1,"name":"Ping","pages":[{"id":"home","title":"Ping","components":[{"type":"form","id":"ping","title":"Ping","fields":[],"action":{"tool":"greet","bindings":[]}}]}]}`)
	caps := capability(`{"type":"object","properties":{}}`)
	d, err := Parse(data, caps)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(encoded, caps); err != nil {
		t.Fatal(err)
	}
}

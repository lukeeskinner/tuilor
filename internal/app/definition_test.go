package app_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/capability"
)

func fixture(t *testing.T) app.Definition {
	t.Helper()
	b, e := os.ReadFile("../../examples/task-manager/app.json")
	if e != nil {
		t.Fatal(e)
	}
	d, e := app.Parse(b, capability.Local())
	if e != nil {
		t.Fatal(e)
	}
	return d
}
func TestDefinitions(t *testing.T) {
	fixture(t)
	b, e := os.ReadFile("testdata/application-tracker.json")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = app.Parse(b, capability.Local()); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{`, `{"version":1,"shell":"rm -rf"}`, string(b) + ` {}`, strings.Replace(string(b), `"type": "list"`, `"type": "terminal"`, 1)} {
		if _, e := app.Parse([]byte(raw), capability.Local()); e == nil {
			t.Fatal("invalid definition accepted")
		}
	}
}
func TestInvalidSemantics(t *testing.T) {
	tests := map[string]func(*app.Definition){
		"unknown capability": func(d *app.Definition) { d.Page.Components[1].Actions[2].Capability = "github.issues.list" },
		"bad column":         func(d *app.Definition) { d.Page.Components[1].Columns[0] = "secret" },
		"bad target":         func(d *app.Definition) { d.Page.Components[1].Actions[3].Target = "none" },
		"wrong literal type": func(d *app.Definition) {
			d.Page.Components[1].Actions[2].Bindings[0].Literal = json.RawMessage(`"true"`)
		},
		"foreign input": func(d *app.Definition) { d.Page.Components[2].Actions[0].Bindings[0].Input = "edit_title" },
		"duplicate IDs": func(d *app.Definition) { d.Page.Components[2].ID = d.Page.Components[1].ID },
		"missing create field": func(d *app.Definition) {
			d.Page.Components[2].Actions[0].Bindings = d.Page.Components[2].Actions[0].Bindings[1:]
		},
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			d := fixture(t)
			change(&d)
			if e := d.Validate(capability.Local()); e == nil {
				t.Fatal("invalid app accepted")
			}
		})
	}
}
func TestTypedValues(t *testing.T) {
	d := fixture(t)
	for _, tc := range []struct {
		field string
		value any
		valid bool
	}{{"due", "2026-02-30", false}, {"due", "2026-10-17", true}, {"due", nil, true}, {"priority", "Urgent", false}, {"completed", "true", false}, {"title", " ", false}} {
		f, _ := d.Field(tc.field)
		if got := app.ValidateValue(f, tc.value) == nil; got != tc.valid {
			t.Errorf("field %s: accepted=%v", tc.field, got)
		}
	}
}

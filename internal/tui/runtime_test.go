package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/capability"
	"github.com/lukeeskinner/tuilor/internal/localapp"
)

func runtimeFixture(t *testing.T, file string) (*Runtime, *localapp.Store) {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	def, err := app.Parse(raw, capability.Local())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "fixture-app")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := localapp.Manifest{Kind: "declarative", Version: 1, ID: "fixture-app"}
	manifest.App.Name = def.Name
	data := map[string]any{"version": 1, "collection": def.Collection.ID, "records": []any{}}
	for name, value := range map[string]any{"manifest.json": manifest, "app.json": def, "data.json": data} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	b, err := localapp.Read(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	s, err := localapp.Open(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRuntime(def, s, nil, capability.Local())
	t.Cleanup(func() { r.Close(); s.Close() })
	return r, s
}

func TestSavedDeclarativeLibraryReopensOffline(t *testing.T) {
	_, store := runtimeFixture(t, "../../examples/task-manager/app.json")
	path := store.Bundle.Dir
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	m := homeFixture(t) // No API credentials or executable tools on PATH.
	m.appsDir = filepath.Dir(path)
	m.screen = "loading"
	next, _ := m.Update(loadApp(context.Background(), path))
	m = next.(Model)
	defer func() { m.Close() }()
	if m.screen != "localapp" || m.runtime == nil {
		t.Fatal(m.View())
	}
	typeText(m.runtime, "a")
	typeText(m.runtime, "Saved q")
	settle(t, m.runtime, press(m.runtime, tea.KeyEnter))
	m, _ = homeKey(m, tea.KeyCtrlS)
	if !strings.Contains(m.runtime.message, "already saved") {
		t.Fatal("saved app tried to publish again")
	}
	m, _ = homeKey(m, tea.KeyEsc)
	if m.screen != "apps" || len(m.files) != 1 || m.files[m.appCursor] != filepath.Base(path) {
		t.Fatal(m.View())
	}
	m.screen = "loading"
	next, _ = m.Update(loadApp(context.Background(), path))
	m = next.(Model)
	if m.screen != "localapp" || m.runtime.Rows[0].Values["title"] != "Saved q" {
		t.Fatal(m.View())
	}
}
func press(r *Runtime, key tea.KeyType) tea.Cmd {
	_, cmd := r.Update(tea.KeyMsg{Type: key})
	return cmd
}
func typeText(r *Runtime, text string) { r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}) }
func settle(t *testing.T, r *Runtime, cmd tea.Cmd) {
	t.Helper()
	if !r.busy || cmd == nil {
		t.Fatalf("operation did not start: %s", r.message)
	}
	r.Update(cmd())
	if r.busy {
		t.Fatal("operation did not complete")
	}
}
func TestRuntimeRecordLifecycle(t *testing.T) {
	r, s := runtimeFixture(t, "../../examples/task-manager/app.json")
	typeText(r, "a")
	typeText(r, "same q")
	press(r, tea.KeyTab)
	press(r, tea.KeyRight)
	press(r, tea.KeyTab)
	typeText(r, "2026-12-01")
	settle(t, r, press(r, tea.KeyEnter))
	if len(r.Rows) != 1 || r.Rows[0].Values["title"] != "same q" || r.Rows[0].Values["priority"] != "High" || r.Rows[0].Values["due"] != "2026-12-01" {
		t.Fatal(r.Rows, r.message)
	}
	first := r.Rows[0].ID
	typeText(r, "a")
	typeText(r, "same q")
	settle(t, r, press(r, tea.KeyEnter))
	if r.Rows[1].ID == first {
		t.Fatal("duplicate IDs")
	}
	r.Selected = 0
	typeText(r, "e")
	press(r, tea.KeyCtrlU)
	typeText(r, "renamed")
	settle(t, r, press(r, tea.KeyEnter))
	if r.Rows[0].Values["title"] != "renamed" {
		t.Fatal(r.Rows)
	}
	settle(t, r, press(r, tea.KeySpace))
	if r.Rows[0].Values["completed"] != true {
		t.Fatal("completion failed")
	}
	press(r, tea.KeyDelete)
	press(r, tea.KeyEsc)
	if len(r.Rows) != 2 || r.confirmation != nil {
		t.Fatal("deletion cancellation failed")
	}
	press(r, tea.KeyDelete)
	settle(t, r, press(r, tea.KeyEnter))
	if len(r.Rows) != 1 || r.Rows[0].ID == first {
		t.Fatal("wrong record deleted")
	}
	rows, err := s.Execute(context.Background(), capability.Call{ID: "local.records.list", Collection: "tasks"})
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
}

type failingExecutor struct{}

func (failingExecutor) Execute(context.Context, capability.Call) ([]capability.Record, error) {
	return nil, errors.New("disk full")
}
func TestRuntimeWriteErrorKeepsInputAndState(t *testing.T) {
	r, _ := runtimeFixture(t, "../../examples/task-manager/app.json")
	r.executor = failingExecutor{}
	typeText(r, "a")
	typeText(r, "keep q")
	settle(t, r, press(r, tea.KeyEnter))
	if len(r.Rows) != 0 || r.form == nil || r.form.inputs[0].Value() != "keep q" || !strings.Contains(r.View(), "disk full") {
		t.Fatal(r.View())
	}
}
func TestTrackerUsesSameRuntimeAndResize(t *testing.T) {
	r, _ := runtimeFixture(t, "../app/testdata/application-tracker.json")
	typeText(r, "a")
	typeText(r, "Example Company")
	press(r, tea.KeyTab)
	typeText(r, "Go engineer")
	settle(t, r, press(r, tea.KeyEnter))
	settle(t, r, press(r, tea.KeySpace))
	if r.Rows[0].Values["stage"] != "Shortlisted" {
		t.Fatal(r.Rows)
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 40}, {Width: 80, Height: 24}, {Width: 40, Height: 12}, {Width: 20, Height: 6}} {
		r.Update(size)
		view := r.View()
		if !strings.Contains(view, "Esc") {
			t.Fatalf("missing navigation at %dx%d: %s", size.Width, size.Height, view)
		}
	}
	typeText(r, "e")
	press(r, tea.KeyEsc)
	if r.form != nil {
		t.Fatal("edit cancellation")
	}
	r.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	for i := 0; i < 40; i++ {
		typeText(r, "a")
		typeText(r, fmt.Sprintf("Company %02d", i))
		press(r, tea.KeyTab)
		typeText(r, "Role")
		settle(t, r, press(r, tea.KeyEnter))
	}
	press(r, tea.KeyPgUp)
	if r.Selected >= len(r.Rows)-1 {
		t.Fatal("page navigation did not move")
	}
	if !strings.Contains(r.View(), inputValue(r.Rows[r.Selected].Values["company"])) {
		t.Fatal("scrolling hid selected record", r.View())
	}
}

func TestFormFocusVisibleAfterResizing(t *testing.T) {
	r, _ := runtimeFixture(t, "../../examples/task-manager/app.json")
	typeText(r, "a")
	typeText(r, "Keep q")
	for i := 0; i < len(r.form.inputs); i++ {
		press(r, tea.KeyTab)
	}
	r.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	if !strings.Contains(r.View(), "> [ Add ]") {
		t.Fatal("focused submit button hidden", r.View())
	}
	press(r, tea.KeyShiftTab)
	if !strings.Contains(r.View(), "> Due date") {
		t.Fatal("focused input hidden", r.View())
	}
	r.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if r.form.inputs[0].Value() != "Keep q" {
		t.Fatal("resize discarded input")
	}
}
func TestLegacyRequiresConfirmationAndSettingsOptional(t *testing.T) {
	t.Setenv("TERM", "dumb")
	m := NewModel(t.TempDir())
	m.codeProject = nil
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"kind":"go","app":{"name":"Old"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	m.screen = "loading"
	m2, _ := m.Update(loadedAppMsg{code: nil, err: errors.New("legacy fixture")})
	if m2.(Model).CodeLaunch() != nil {
		t.Fatal("failed legacy opened")
	}
	c := goAppConfirmation{name: "Old"}
	next, _ := c.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if next.(goAppConfirmation).approved {
		t.Fatal("legacy approved on escape")
	}
	next, _ = c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !next.(goAppConfirmation).approved {
		t.Fatal("legacy approval missing")
	}
	if len(m.choices) != 3 || m.choices[2] != "Settings" || m.mcp != nil {
		t.Fatal("MCP required on startup")
	}
}

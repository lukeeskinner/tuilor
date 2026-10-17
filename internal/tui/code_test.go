package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/codeapp"
)

func TestHomeCreationDefaultsToSourceAndPreflightsBeforeAI(t *testing.T) {
	m := homeFixture(t)
	t.Setenv("OPENAI_API_KEY", "fixture-not-a-real-key")
	m.codeSpec = &codeapp.Spec{Name: "Previous", Source: "stale source"}
	next, cmd := m.beginGeneration()
	m = next.(Model)
	if cmd == nil || m.screen != "codegenerating" || m.codeSpec != nil || m.runtime != nil {
		t.Fatal("new creation did not use a fresh Go project")
	}
	next, _ = m.Update(cmd()) // PATH has no Go: no API call can occur.
	m = next.(Model)
	if m.screen != "create" || m.loading || !strings.Contains(m.creator.message, "Go 1.25.5") {
		t.Fatal(m.View())
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 40}, {Width: 80, Height: 24}, {Width: 40, Height: 12}} {
		m.Update(size)
		view := m.creator.View(size.Width, size.Height)
		if !strings.Contains(view, "OS") || !strings.Contains(view, "Ctrl+G") {
			t.Fatal("missing execution notice or controls", view)
		}
	}
}
func TestBuildFailureAndCanceledResultNeverLaunch(t *testing.T) {
	m := homeFixture(t)
	project := &codeapp.Project{Dir: t.TempDir(), Draft: true, Spec: codeapp.Spec{Name: "Custom app", Source: "bad source"}}
	spec := project.Spec
	m.codeProject, m.codeSpec = project, &spec
	m.screen = "codebuilding"
	next, cmd := m.Update(codeBuiltMsg{project: project, err: errors.New("compile failed")})
	m = next.(Model)
	if cmd != nil || m.CodeLaunch() != nil || m.screen != "codereview" || !strings.Contains(strings.Join(m.codeChoices(), ","), "Repair with AI") {
		t.Fatal("build failure launched or omitted explicit repair", m.View())
	}
	// Navigating a failed result never starts a paid request.
	m, cmd = homeKey(m, tea.KeyDown)
	if cmd != nil {
		t.Fatal("navigation triggered repair")
	}
	m.screen = "codecancel"
	m.codeReturn = "create"
	next, _ = m.Update(codeBuiltMsg{project: project})
	m = next.(Model)
	if m.CodeLaunch() != nil || m.screen != "create" {
		t.Fatal("late canceled build launched")
	}
}
func TestExplicitRepairCanCancelAndRetainsTrialData(t *testing.T) {
	m := homeFixture(t)
	t.Setenv("OPENAI_API_KEY", "fixture-not-a-real-key")
	data := filepath.Join(t.TempDir(), "trial.json")
	if err := os.WriteFile(data, []byte(`{"notes":["keep"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	project := &codeapp.Project{Dir: filepath.Dir(data), Draft: true, Spec: codeapp.Spec{Name: "Custom app", Source: "broken source"}}
	spec := project.Spec
	m.codeProject, m.codeSpec, m.codeError, m.codePrompt = project, &spec, "compile failed", "Build a custom app"
	m.screen = "codereview"
	next, cmd := m.codeReviewKey("enter") // First choice is the explicit repair.
	m = next.(Model)
	if cmd == nil || m.screen != "codegenerating" || !m.loading {
		t.Fatal("explicit repair did not start")
	}
	m, _ = homeKey(m, tea.KeyEsc)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.screen != "codereview" || m.codeProject != project || m.codeSpec != &spec || m.CodeLaunch() != nil {
		t.Fatal("cancel lost prior candidate")
	}
	raw, err := os.ReadFile(data)
	if err != nil || string(raw) != `{"notes":["keep"]}` {
		t.Fatal("cancel changed trial data", err)
	}
}

func TestCancelRunAgainReturnsToRetainedCandidate(t *testing.T) {
	m := homeFixture(t)
	p := &codeapp.Project{Dir: t.TempDir(), Draft: true, Spec: codeapp.Spec{Name: "Trial", Source: "source"}}
	m.codeProject, m.codeSpec = p, &p.Spec
	m.screen, m.codeReturn = "codeconfirm", "codereview"
	m, _ = homeKey(m, tea.KeyEsc)
	if m.screen != "codereview" || m.codeProject != p || m.CodeLaunch() != nil {
		t.Fatal("canceling Run Again lost the trial project")
	}
}

func TestTinyTerminalCannotStartHiddenGeneration(t *testing.T) {
	m := homeFixture(t)
	t.Setenv("OPENAI_API_KEY", "fixture-not-a-real-key")
	m.width, m.height = 20, 6
	next, cmd := m.beginGeneration()
	m = next.(Model)
	if cmd != nil || m.loading || m.screen != "create" || !strings.Contains(m.View(), "Enlarge") {
		t.Fatal("started generation with its execution notice hidden")
	}
}

func homeFixture(t *testing.T) Model {
	t.Helper()
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("PATH", t.TempDir()) // Neither Go nor an MCP executable is needed.
	t.Setenv("TUILOR_REDUCED_MOTION", "1")
	m := NewModel(filepath.Join(t.TempDir(), "apps")).WithWorkspace(t.TempDir())
	m.creator = newCreation()
	m.creator.prompt.SetValue("Build a task manager\nwith priorities and due dates q")
	m.screen = "create"
	return m
}

func homeKey(m Model, key tea.KeyType) (Model, tea.Cmd) {
	next, cmd := m.Update(tea.KeyMsg{Type: key})
	return next.(Model), cmd
}

func TestHomeMissingCredentialsBeforeGeneration(t *testing.T) {
	m := homeFixture(t)
	next, cmd := m.beginGeneration()
	m = next.(Model)
	if cmd != nil || m.loading || !strings.Contains(m.creator.message, "OPENAI_API_KEY") {
		t.Fatal("missing credentials started work")
	}
}
func TestLibraryLegacyLaunchIsExplicit(t *testing.T) {
	m := homeFixture(t)
	project := &codeapp.Project{Spec: codeapp.Spec{Name: "Old notes"}}
	m.screen = "loading"
	next, cmd := m.Update(loadedAppMsg{code: project})
	m = next.(Model)
	if cmd != nil || m.screen != "codeconfirm" || m.CodeLaunch() != nil || !strings.Contains(m.View(), "Go app") {
		t.Fatal("legacy project launched without approval")
	}
	m, _ = homeKey(m, tea.KeyEsc)
	if m.screen != "apps" || m.CodeLaunch() != nil {
		t.Fatal("declined legacy app launched")
	}
	m.screen = "codereview"
	next, _ = m.codeReviewKey("enter")
	if next.(Model).CodeLaunch() != nil {
		t.Fatal("review bypassed approval")
	}
}

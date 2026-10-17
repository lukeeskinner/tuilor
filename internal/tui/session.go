package tui

import (
	"context"
	"errors"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/capability"
	"github.com/lukeeskinner/tuilor/internal/codeapp"
	"github.com/lukeeskinner/tuilor/internal/localapp"
)

type standaloneLocal struct{ *Runtime }

func (s standaloneLocal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := s.Runtime.Update(msg)
	if s.ExitRequested {
		return s, tea.Quit
	}
	if s.SaveRequested {
		s.message = "Changes are already saved locally."
	}
	return s, cmd
}
func RunLocal(ctx context.Context, bundle localapp.Bundle, in io.Reader, out io.Writer) (err error) {
	store, err := localapp.Open(ctx, bundle)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, store.Close()) }()
	records, err := store.Execute(ctx, capability.Call{ID: "local.records.list", Collection: bundle.Definition.Collection.ID})
	if err != nil {
		return err
	}
	runtime := NewRuntime(bundle.Definition, store, records, capability.Local())
	runtime.Path = bundle.Dir
	defer func() { err = errors.Join(err, runtime.Close()) }()
	_, err = tea.NewProgram(standaloneLocal{runtime}, tea.WithAltScreen(), tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	return err
}

type goAppConfirmation struct {
	name     string
	approved bool
}

func (c goAppConfirmation) Init() tea.Cmd { return nil }
func (c goAppConfirmation) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			c.approved = true
			return c, tea.Quit
		case "esc", "q", "ctrl+c":
			return c, tea.Quit
		}
	}
	return c, nil
}
func goAppNotice(name string) string {
	return "Go app: " + escapeControls(name) + "\n\nThis runs saved executable Go code with your OS permissions.\nIt is not sandboxed.\n\nEnter Trust and launch · Esc Cancel\n"
}
func (c goAppConfirmation) View() string { return goAppNotice(c.name) }
func ConfirmGoApp(ctx context.Context, name string, in io.Reader, out io.Writer) (bool, error) {
	result, err := tea.NewProgram(goAppConfirmation{name: name}, tea.WithAltScreen(), tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if err != nil {
		return false, err
	}
	return result.(goAppConfirmation).approved, nil
}

// ResolveAppConnections runs only connection approval/setup, without generation.
func ResolveAppConnections(ctx context.Context, project *codeapp.Project, in io.Reader, out io.Writer) (bool, error) {
	if len(project.Spec.Requirements) == 0 {
		return true, nil
	}
	model := NewModel("")
	model.codeProject = project
	spec := project.Spec
	model.codeSpec = &spec
	next, cmd := model.prepareCodeLaunch()
	session := connectionSession{model: next.(Model), start: cmd}
	result, err := tea.NewProgram(session, tea.WithAltScreen(), tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out)).Run()
	if final, ok := result.(connectionSession); ok {
		err = errors.Join(err, final.model.Close())
		return final.model.CodeLaunch() != nil, err
	}
	return false, err
}

type connectionSession struct {
	model Model
	start tea.Cmd
}

func (s connectionSession) Init() tea.Cmd { return s.start }
func (s connectionSession) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := s.model.Update(msg)
	s.model = next.(Model)
	if s.model.screen == "codereview" || s.model.screen == "home" {
		return s, tea.Quit
	}
	return s, cmd
}
func (s connectionSession) View() string { return s.model.View() }

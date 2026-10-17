package tui

import (
	"context"
	"errors"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/capability"
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

package tui

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/capability"
	"github.com/lukeeskinner/tuilor/internal/localapp"
)

// Saved declarative apps retain their runtime; new creation uses code.go.
func (m *Model) openLocal(bundle localapp.Bundle) error {
	store, err := localapp.Open(context.Background(), bundle)
	if err != nil {
		return err
	}
	records, err := store.Execute(context.Background(), capability.Call{ID: "local.records.list", Collection: bundle.Definition.Collection.ID})
	if err != nil {
		store.Close()
		return err
	}
	m.localStore = store
	m.runtime = NewRuntime(bundle.Definition, store, records, capability.Local())
	m.runtime.Width, m.runtime.Height = m.width, m.height
	m.runtime.Path = bundle.Dir
	m.screen = "localapp"
	return nil
}
func (m Model) updateLocal(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.runtime.Update(msg)
	if m.runtime.ExitRequested {
		path := m.localStore.Bundle.Dir
		err := errors.Join(m.runtime.Close(), m.localStore.Close())
		m.runtime, m.localStore = nil, nil
		m.returnToLibrary(path)
		m.err = err
		return m, nil
	}
	if m.runtime.SaveRequested {
		m.runtime.message = "Changes are already saved locally."
	}
	return m, cmd
}

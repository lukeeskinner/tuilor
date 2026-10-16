package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/localapp"
)

type localInvocation struct {
	action          app.LocalAction
	inputs          map[string]string
	selected, label string
	page, form      int
}

type changedLocalMsg struct {
	store *localapp.Store
	call  localInvocation
	err   error
}

func (m Model) requestLocal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	target, ok := m.preview.activeTarget()
	if !ok {
		return m, nil
	}
	p := &m.preview
	c := p.definition.Pages[p.page].Components[target.form]
	call := localInvocation{page: p.page, form: target.form}
	if c.Type == "form" {
		if key != "enter" {
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return m, cmd
		}
		call.action = *c.LocalAction
		call.inputs = map[string]string{}
		offset := 0
		for _, previous := range p.definition.Pages[p.page].Components[:target.form] {
			offset += len(previous.Fields)
		}
		for i, f := range c.Fields {
			call.inputs[f.ID] = p.inputs[p.page][offset+i].Value()
		}
	} else {
		kind := ""
		if key == " " {
			kind = "update"
		}
		if key == "delete" {
			kind = "delete"
		}
		for _, a := range c.Actions {
			if a.Kind == kind {
				call.action = a
				break
			}
		}
		if call.action.Kind == "" {
			return m, nil
		}
		rows := p.local.Records(c.Collection)
		if len(rows) == 0 {
			return m, nil
		}
		row := rows[p.selected[[2]int{p.page, target.form}]]
		call.selected, call.label = row.ID, row.Values[c.LabelField].(string)
	}
	if call.action.Kind == "delete" {
		m.localCall = &call
		m.screen = "localdelete"
		p.blurInput()
		return m, nil
	}
	return m.startLocal(call)
}

func (m Model) startLocal(call localInvocation) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	m.localCall = nil
	m.loading, m.screen = true, "saving"
	m.cancel, m.done = func() {}, make(chan struct{})
	m.preview.blurInput()
	store, done := m.preview.local, m.done
	return m, func() tea.Msg {
		defer close(done)
		next, err := store.Apply(call.action, call.inputs, call.selected)
		return changedLocalMsg{next, call, err}
	}
}

func (m Model) localResult(msg changedLocalMsg) (tea.Model, tea.Cmd) {
	m.loading, m.cancel, m.done = false, nil, nil
	if m.quitting {
		return m, tea.Quit
	}
	m.screen = "preview"
	p := &m.preview
	if msg.err != nil {
		p.saveMessage = "Change failed: " + msg.err.Error()
	} else {
		p.local = msg.store
		p.saveMessage = ""
		if msg.call.action.Kind == "insert" {
			offset := 0
			for _, c := range p.definition.Pages[msg.call.page].Components[:msg.call.form] {
				offset += len(c.Fields)
			}
			for i := range p.definition.Pages[msg.call.page].Components[msg.call.form].Fields {
				p.inputs[msg.call.page][offset+i].SetValue("")
			}
		}
	}
	p.renderContent(msg.err == nil)
	if msg.err != nil {
		p.viewport.GotoTop()
	}
	return m, p.focusInput()
}

func (m Model) localDeleteView() string {
	if m.width < 30 || m.height < 6 {
		return "Enlarge terminal to confirm."
	}
	label := lipgloss.NewStyle().Width(m.width).MaxHeight(max(1, m.height-4)).Render(escapeControls(m.localCall.label))
	id := lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(1).Render(fmt.Sprintf("Record: %s", escapeControls(m.localCall.selected)))
	return strings.Join([]string{"Delete this record?", label, id, "y Delete · Esc Cancel"}, "\n")
}

package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
)

type invocation struct {
	page, form int
	tool       string
	arguments  map[string]any
}

type calledToolMsg struct {
	result mcpclient.CallResult
	err    error
}

func (p previewModel) invocation() (invocation, bool) {
	target, ok := p.activeTarget()
	if !ok {
		return invocation{}, false
	}
	components := p.definition.Pages[p.page].Components
	form := components[target.form]
	values := map[string]string{}
	offset := 0
	for _, component := range components[:target.form] {
		offset += len(component.Fields)
	}
	for i, field := range form.Fields {
		values[field.ID] = p.inputs[p.page][offset+i].Value()
	}
	arguments := make(map[string]any, len(form.Action.Bindings))
	for _, binding := range form.Action.Bindings {
		arguments[binding.Parameter] = values[binding.Field]
	}
	return invocation{p.page, target.form, form.Action.Tool, arguments}, true
}

func (p *previewModel) showResult(call invocation, text string) {
	p.results[[2]int{call.page, call.form}] = text
	p.renderContent(false)
	if p.resultLine >= 0 {
		p.viewport.SetYOffset(p.resultLine)
	}
}

func (m Model) requestSubmission() (tea.Model, tea.Cmd) {
	call, ok := m.preview.invocation()
	if !ok {
		return m, nil
	}
	for _, tool := range m.tools {
		if tool.Name != call.tool {
			continue
		}
		if err := tool.ValidateArguments(call.arguments); err != nil {
			m.preview.showResult(call, err.Error())
			return m, nil
		}
		m.preview.blurInput()
		m.confirmation = &confirmationModel{call: call, server: m.server, view: viewport.New(1, 1)}
		m.confirmation.resize(m.width, m.height)
		m.screen = "confirm"
		return m, nil
	}
	m.preview.showResult(call, fmt.Sprintf("Tool %q is unavailable.", call.tool))
	return m, nil
}

func (m Model) startCall() (tea.Model, tea.Cmd) {
	// The only caller handles a literal 'y' on the exact-arguments confirmation.
	call := m.confirmation.call
	m.call, m.confirmation = &call, nil
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.cancel, m.done = cancel, make(chan struct{})
	m.screen, m.loading = "calling", true
	server, done := m.server, m.done
	return m, func() tea.Msg {
		defer close(done)
		defer cancel()
		result, err := mcpclient.CallTool(ctx, server, call.tool, call.arguments)
		return calledToolMsg{result, err}
	}
}

type confirmationModel struct {
	call   invocation
	server string
	view   viewport.Model
}

func (c *confirmationModel) resize(width, height int) {
	c.view.Width, c.view.Height = max(1, width), max(1, height-2)
	arguments, _ := json.MarshalIndent(c.call.arguments, "", "  ") // Arguments contain only strings.
	text := "Server: " + escapeControls(c.server) + "\nTool: " + escapeControls(c.call.tool) + "\nArguments:\n" + string(arguments)
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, lipgloss.NewStyle().Width(c.view.Width).Render(escapeControls(line)))
	}
	c.view.SetContent(strings.Join(lines, "\n"))
}

func (c confirmationModel) View() string {
	line := lipgloss.NewStyle().MaxWidth(c.view.Width).MaxHeight(1)
	if c.view.Width < 30 || c.view.Height < 4 {
		return line.Render("Enlarge terminal to confirm.")
	}
	return line.Render("Allow this one tool call?") + "\n" + c.view.View() + "\n" + line.Render("y Run once · Esc Cancel · PgUp/PgDn Scroll")
}

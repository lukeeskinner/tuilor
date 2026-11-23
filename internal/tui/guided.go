package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/integrations"
)

func (m Model) connectionRequirement() (integrations.Requirement, error) {
	if m.codeProject == nil {
		return integrations.Requirement{}, fmt.Errorf("no application is waiting for a connection")
	}
	requirements, err := m.codeProject.ConnectionRequirements()
	if err != nil {
		return integrations.Requirement{}, err
	}
	if m.api.requirement >= len(requirements) {
		return integrations.Requirement{}, fmt.Errorf("no pending integration")
	}
	return requirements[m.api.requirement], nil
}

func (m Model) nextConnection() (tea.Model, tea.Cmd) {
	a := m.api
	if a.requirement >= len(m.codeProject.Spec.Requirements) {
		return m.beginAPI("launch")
	}
	_, matches := m.bindingChoices()
	if len(matches) != 0 {
		m.screen = "apibind"
		return m, nil
	}
	r, err := m.connectionRequirement()
	if err != nil {
		m.codeError = err.Error()
		m.screen = "codereview"
		return m, nil
	}
	return m.startGuide(integrations.Connection{ID: integrations.ID(), Name: r.Name, Kind: r.Kind, BaseURL: r.BaseURL, Auth: r.Auth, Key: r.Key})
}

func (m Model) startGuide(c integrations.Connection) (tea.Model, tea.Cmd) {
	a := m.api
	a.guided = true
	a.form(c, m.width)
	a.guideFocus, a.guideScroll, a.guideFollow = 0, 0, true
	a.inputs[0].Blur()
	if c.Auth != "none" && c.Auth != "oauth" {
		a.inputs[3].Focus()
	}
	m.screen = "apiguide"
	return m, textinput.Blink
}

func (m Model) guideConnected() (tea.Model, tea.Cmd) {
	a := m.api
	r, err := m.connectionRequirement()
	if err != nil {
		a.message = err.Error()
		m.screen = "apiguide"
		return m, nil
	}
	c := a.editing
	if m.codeProject.Overrides == nil {
		m.codeProject.Overrides = map[string]integrations.Requirement{}
	}
	if c.BaseURL != r.BaseURL || c.Auth != r.Auth || c.Key != r.Key {
		r.BaseURL, r.Auth, r.Key = c.BaseURL, c.Auth, c.Key
		m.codeProject.Overrides[r.ID] = r
	}
	m.codeProject.Bindings[r.ID] = c.ID
	a.inputs[3].SetValue("")
	a.guided, a.action, a.guideScroll = false, 0, 0
	a.requirement++
	return m.nextConnection()
}

func (m Model) guideControls() []string {
	a := m.api
	controls := []string{}
	if a.editing.Auth != "none" && a.editing.Auth != "oauth" {
		controls = append(controls, "credential")
	}
	controls = append(controls, "connect")
	r, _ := m.connectionRequirement()
	if r.Setup != nil {
		if r.Setup.DocumentationURL != "" {
			controls = append(controls, "documentation")
		}
		if r.Setup.CredentialURL != "" {
			controls = append(controls, "setup")
		}
	}
	return append(controls, "advanced")
}

func (m Model) guideKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := m.api
	if key.String() == "esc" {
		m.screen = "apibind"
		return m, nil
	}
	if m.width < 28 || m.height < 10 {
		return m, nil
	} // Destination must be visible before approval.
	controls := m.guideControls()
	a.guideFocus = min(a.guideFocus, len(controls)-1)
	switch key.String() {
	case "tab", "shift+tab":
		delta := 1
		if key.String() == "shift+tab" {
			delta = -1
		}
		a.guideFocus = (a.guideFocus + delta + len(controls)) % len(controls)
		a.guideFollow = true
		a.inputs[3].Blur()
		if controls[a.guideFocus] == "credential" {
			a.inputs[3].Focus()
		}
		return m, textinput.Blink
	case "pgup", "pgdown":
		delta := max(1, m.height-10)
		if key.String() == "pgup" {
			delta = -delta
		}
		a.guideScroll = max(0, a.guideScroll+delta)
		a.guideFollow = false
		return m, nil
	case "enter":
		switch controls[a.guideFocus] {
		case "credential", "connect":
			return m.beginAPI("connect")
		case "advanced":
			a.field = 0
			a.focus()
			m.screen = "apiform"
			return m, textinput.Blink
		case "documentation", "setup":
			r, _ := m.connectionRequirement()
			a.linkURL = r.Setup.DocumentationURL
			if controls[a.guideFocus] == "setup" {
				a.linkURL = r.Setup.CredentialURL
			}
			return m.beginAPI("openlink")
		}
	}
	if controls[a.guideFocus] == "credential" {
		var cmd tea.Cmd
		a.inputs[3], cmd = a.inputs[3].Update(key)
		return m, cmd
	}
	return m, nil
}

func (m Model) guideView() string {
	a := m.api
	width := studioWidth(m.width)
	if m.width < 28 || m.height < 10 {
		return m.welcomeCanvas("Connect service\nEnlarge terminal · Esc Keep candidate")
	}
	r, _ := m.connectionRequirement()
	line := lipgloss.NewStyle().Width(width).MaxWidth(width)
	header := studioHeader("Connect "+escapeControls(a.editing.Name), width) + "\n" +
		line.MaxHeight(2).Foreground(welcomeMint).Render("Destination: "+escapeControls(a.editing.BaseURL)) + "\n" +
		line.MaxHeight(1).Foreground(welcomeMuted).Render(authLabel(a.editing.Auth))
	if a.message != "" {
		header += "\n" + line.MaxHeight(2).Foreground(welcomePeach).Render(escapeControls(a.message))
	}
	var lines []string
	add := func(text string) { lines = append(lines, strings.Split(line.Render(escapeControls(text)), "\n")...) }
	if lipgloss.Height(line.Render("Destination: "+escapeControls(a.editing.BaseURL))) > 2 {
		add("Full destination: " + a.editing.BaseURL)
		add("")
	}
	if a.message != "" && lipgloss.Height(line.Render(escapeControls(a.message))) > 2 {
		add(a.message)
		add("")
	}
	if r.Setup != nil {
		add("AI-suggested setup · review before connecting")
		if r.Setup.Instructions != "" {
			add(r.Setup.Instructions)
		}
		if len(r.Setup.Permissions) > 0 {
			add("Suggested permissions: " + strings.Join(r.Setup.Permissions, ", "))
		}
		add("")
	}
	add("Connecting approves this destination for the app. Writes still require confirmation.")
	focusLine := len(lines)
	controls := m.guideControls()
	for i, control := range controls {
		lines = append(lines, "")
		if i == a.guideFocus {
			focusLine = len(lines)
		}
		marker := "  "
		if i == a.guideFocus {
			marker = "> "
		}
		label := ""
		switch control {
		case "credential":
			label = "Credential (hidden)"
			if r.Setup != nil && r.Setup.CredentialLabel != "" {
				label = escapeControls(r.Setup.CredentialLabel) + " (hidden)"
			}
		case "connect":
			label = "Connect " + escapeControls(a.editing.Name)
		case "advanced":
			label = "Advanced · name, endpoint and authentication"
		case "documentation", "setup":
			raw := r.Setup.DocumentationURL
			label = "Open documentation"
			if control == "setup" {
				raw = r.Setup.CredentialURL
				label = "Open credential setup"
			}
			if u, err := integrations.SetupURL(raw); err == nil {
				label += " · " + u.Host
			}
		}
		style := line
		if i == a.guideFocus {
			style = style.Foreground(welcomeMint).Bold(true)
		}
		lines = append(lines, strings.Split(style.Render(marker+label), "\n")...)
		if control == "credential" {
			lines = append(lines, a.inputs[3].View())
		}
	}
	help := line.Foreground(welcomeMuted).Render("Tab Focus · Enter Continue · PgUp/PgDn Scroll\nEsc Keep candidate · Ctrl+C Quit")
	if m.width < 60 || m.height < 16 {
		help = line.Foreground(welcomeMuted).Render("Tab/Enter · PgUp/PgDn\nEsc Back · Ctrl+C Quit")
	}
	capacity := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(help)-1)
	start := min(a.guideScroll, max(0, len(lines)-capacity))
	if a.guideFollow {
		end := focusLine
		if controls[a.guideFocus] == "credential" {
			end++
		}
		start = max(0, min(start, focusLine))
		if end >= start+capacity {
			start = end - capacity + 1
		}
	}
	a.guideScroll = start
	body := strings.Join(lines[start:min(len(lines), start+capacity)], "\n")
	return m.welcomeCanvas(header + "\n\n" + lipgloss.NewStyle().Height(capacity).MaxHeight(capacity).Render(body) + "\n" + help)
}

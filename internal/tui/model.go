package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	choices []string
	cursor  int
	screen  string
}

func NewModel() Model {
	return Model{
		choices: []string{
			"My Apps",
			"Create App",
			"MCP servers",
			"Settings",
		},
		cursor: 0,
		screen: "home",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.screen = "home"
		case "down":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.screen == "home" {
				m.screen = m.choices[m.cursor]
			}

		}
	}
	return m, nil

}

func (m Model) View() string {
	if m.screen != "home" {
		return "◆ TUILOR\n\n" +
			m.screen + "\n\n" +
			"Coming soon...\n\n" +
			"Esc Back • q Quit\n"
	}
	output := "Welcome to Tuilor!\n\n"

	for i, choice := range m.choices {
		if i == m.cursor {
			output += "> " + choice + "\n"
		} else {
			output += " " + choice + "\n"
		}
	}
	output += "\n↑↓ Navigate • Enter Select • q Quit\n"
	return output
}

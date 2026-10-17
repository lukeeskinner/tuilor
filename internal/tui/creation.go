package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/generator"
)

type creationModel struct {
	prompt  textarea.Model
	message string
}

func newCreation() *creationModel {
	prompt := textarea.New()
	prompt.Placeholder = "Describe the terminal app you want to build…"
	prompt.ShowLineNumbers = false
	return &creationModel{prompt: prompt}
}

func (c *creationModel) resize(width, height int) {
	c.prompt.SetWidth(max(1, width-2))
	c.prompt.SetHeight(max(1, height-7-lipgloss.Height(c.messageView(width, height))))
}

func (c creationModel) messageView(width, height int) string {
	return lipgloss.NewStyle().Width(max(1, width)).MaxHeight(max(1, height/3)).Render(escapeControls(c.message))
}

func (c *creationModel) setMessage(message string, width, height int) {
	c.message = message
	c.resize(width, height)
}

func (c creationModel) View(width, height int) string {
	line := lipgloss.NewStyle().MaxWidth(max(1, width)).MaxHeight(1)
	if width < 30 || height < 10 {
		return line.Render("Enlarge terminal to create an app.")
	}
	return strings.Join([]string{
		"Create App", line.Render("Generated Go app · no MCP server needed"),
		line.Render("Model: " + escapeControls(generator.ConfiguredModel()) + " · OpenAI request (billable)."),
		line.Render("Runs Go with your OS access."),
		c.prompt.View(), c.messageView(width, height),
		line.Render("Ctrl+G Generate · Enter Newline · Esc Back · Ctrl+C Quit"),
	}, "\n")
}

func (m Model) beginGeneration() (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	prompt := m.creator.prompt.Value()
	if strings.TrimSpace(prompt) == "" {
		m.creator.setMessage("Describe an app before generating.", m.width, m.height)
		return m, nil
	}
	provider, err := generator.NewOpenAIFromEnv()
	if err != nil {
		m.creator.setMessage(err.Error(), m.width, m.height)
		return m, nil
	}
	return m.beginCodeGeneration(provider, false)
}

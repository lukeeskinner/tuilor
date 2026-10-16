package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/generator"
	"github.com/lukeeskinner/tuilor/internal/localapp"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
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
	c.prompt.SetHeight(max(1, height-6-lipgloss.Height(c.messageView(width, height))))
}

func (c creationModel) messageView(width, height int) string {
	return lipgloss.NewStyle().Width(max(1, width)).MaxHeight(max(1, height/3)).Render(escapeControls(c.message))
}

func (c *creationModel) setMessage(message string, width, height int) {
	c.message = message
	c.resize(width, height)
}

func (c creationModel) View(server string, width, height int) string {
	line := lipgloss.NewStyle().MaxWidth(max(1, width)).MaxHeight(1)
	if width < 30 || height < 10 {
		return line.Render("Enlarge terminal to create an app.")
	}
	mode := "Local collections · no MCP server needed"
	if server != "" {
		mode = "MCP server: " + escapeControls(server)
	}
	return strings.Join([]string{
		"Create App", line.Render(mode),
		line.Render("Generate sends your prompt to OpenAI (billable)."),
		c.prompt.View(), c.messageView(width, height),
		line.Render("Ctrl+G Generate · Enter Newline · Esc Back · Ctrl+C Quit"),
	}, "\n")
}

type generatedAppMsg struct {
	definition *app.Definition
	tools      []mcpclient.Tool
	err        error
}

type savedAppMsg struct {
	path  string
	err   error
	local *localapp.Store
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	m.cancel, m.done = cancel, make(chan struct{})
	m.loading, m.screen = true, "generating"
	m.creator.setMessage("", m.width, m.height)
	m.creator.prompt.Blur()
	server, done := m.server, m.done
	return m, func() tea.Msg {
		defer close(done)
		defer cancel()
		if strings.TrimSpace(server) == "" {
			definition, err := generator.GenerateLocal(ctx, provider, prompt)
			return generatedAppMsg{definition: definition, err: err}
		}
		tools, err := mcpclient.ListTools(ctx, server)
		if err != nil {
			return generatedAppMsg{err: err}
		}
		definition, err := generator.Generate(ctx, provider, prompt, capabilitiesFor(tools))
		return generatedAppMsg{definition: definition, tools: tools, err: err}
	}
}

func capabilitiesFor(tools []mcpclient.Tool) []app.Capability {
	capabilities := make([]app.Capability, 0, len(tools))
	for _, tool := range tools {
		capabilities = append(capabilities, app.Capability{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
	}
	return capabilities
}

func (m Model) saveDraft() (tea.Model, tea.Cmd) {
	if m.loading || !m.preview.draft {
		return m, nil
	}
	m.loading, m.screen = true, "saving"
	// Saving is an explicit small local write. Wait for its result before leaving
	// this screen; cancellation cannot reliably undo a completed filesystem write.
	m.cancel, m.done = func() {}, make(chan struct{})
	m.preview.blurInput()
	definition, directory, done := m.preview.definition, m.appsDir, m.done
	local := m.preview.local
	return m, func() tea.Msg {
		defer close(done)
		if local != nil {
			saved, err := local.Save(directory)
			if err != nil {
				return savedAppMsg{err: err}
			}
			return savedAppMsg{path: saved.Path, local: saved}
		}
		path, err := app.Save(definition, directory)
		return savedAppMsg{path: path, err: err}
	}
}

func (m *Model) returnToLibrary(savedPath string) {
	m.preview, m.tools = previewModel{}, nil
	m.screen = "apps"
	m.readLibrary()
	m.appCursor = max(0, min(m.appCursor, len(m.files)-1))
	if savedPath != "" {
		for i, name := range m.files {
			if name == filepath.Base(savedPath) {
				m.appCursor = i
				break
			}
		}
	}
}

func (m Model) generationResult(msg generatedAppMsg) (tea.Model, tea.Cmd) {
	m.loading, m.cancel, m.done = false, nil, nil
	if m.quitting {
		return m, tea.Quit
	}
	if m.screen == "cancelgeneration" {
		m.creator.message = "Generation canceled. Your prompt is preserved."
	} else if msg.err != nil {
		m.creator.message = msg.err.Error()
	} else {
		m.tools, m.screen = msg.tools, "preview"
		var cmd tea.Cmd
		m.preview, cmd = newPreview(msg.definition, m.width, m.height)
		m.preview.draft = true
		return m, cmd
	}
	m.screen = "create"
	m.creator.resize(m.width, m.height)
	return m, m.creator.prompt.Focus()
}

func (m Model) saveResult(msg savedAppMsg) (tea.Model, tea.Cmd) {
	m.loading, m.cancel, m.done = false, nil, nil
	if m.quitting {
		return m, tea.Quit
	}
	m.screen = "preview"
	if msg.err != nil {
		m.preview.saveMessage = fmt.Sprintf("Save failed: %s", msg.err)
	} else {
		if msg.local != nil {
			m.preview.local = msg.local
		}
		m.preview.draft = false
		m.preview.savedPath = msg.path
		m.preview.saveMessage = "Saved: " + msg.path
	}
	m.preview.renderContent(false)
	m.preview.viewport.GotoTop()
	return m, m.preview.focusInput()
}

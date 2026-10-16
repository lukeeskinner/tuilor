package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
)

type Model struct {
	appsDir, server   string
	choices           []string
	cursor, appCursor int
	screen            string
	files             []string
	err               error
	preview           previewModel
	tools             []mcpclient.Tool
	confirmation      *confirmationModel
	call              *invocation
	width, height     int
	cancel            context.CancelFunc
	done              chan struct{}
	loading, quitting bool
}

type loadedAppMsg struct {
	definition *app.Definition
	err        error
	tools      []mcpclient.Tool
}

func NewModel(appsDir, server string) Model {
	return Model{
		appsDir: appsDir, server: server,
		choices: []string{"My Apps", "Create App", "MCP servers", "Settings"},
		screen:  "home", width: 80, height: 24,
	}
}

func (m Model) Init() tea.Cmd { return nil }

// Close waits for SDK-owned subprocess cleanup after an external cancellation
// or terminal error. The CLI calls it on the final model returned by Bubble Tea.
func (m Model) Close() error {
	if m.cancel == nil {
		return nil
	}
	m.cancel()
	select {
	case <-m.done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("MCP operation did not finish cleanup within 5 seconds")
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		if m.confirmation != nil {
			m.confirmation.resize(m.width, m.height)
		}
		if m.preview.definition != nil {
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return m, cmd
		}
	case calledToolMsg:
		m.loading, m.cancel, m.done = false, nil, nil
		if m.quitting {
			return m, tea.Quit
		}
		text := "Result: " + msg.result.Text
		if msg.result.IsError {
			text = "Tool error: " + msg.result.Text
		}
		if msg.err != nil {
			if msg.result.Text == "" {
				text = ""
			}
			text += "\nError: " + msg.err.Error()
		}
		m.preview.showResult(*m.call, strings.TrimSpace(text))
		m.call, m.screen = nil, "preview"
		return m, m.preview.focusInput()
	case loadedAppMsg:
		m.loading, m.cancel, m.done = false, nil, nil
		if m.quitting {
			return m, tea.Quit
		}
		if m.screen != "loading" {
			return m, nil
		} // Esc already left this request.
		m.err, m.screen = msg.err, "apps"
		if msg.err == nil {
			m.tools = msg.tools
			m.screen = "preview"
			var cmd tea.Cmd
			m.preview, cmd = newPreview(msg.definition, m.width, m.height)
			return m, cmd
		}
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" || (key == "q" && m.preview.definition == nil) {
			if m.loading {
				m.cancel()
				m.quitting = true
				return m, nil
			}
			return m, tea.Quit
		}
		if m.quitting {
			return m, nil
		}
		if key == "esc" {
			m.err = nil
			switch m.screen {
			case "confirm":
				m.confirmation, m.screen = nil, "preview"
				return m, m.preview.focusInput()
			case "calling", "canceling":
				m.cancel()
				m.screen = "canceling"
			case "loading":
				m.cancel()
				m.screen = "apps"
			case "preview":
				m.preview, m.screen = previewModel{}, "apps"
				m.tools = nil
			default:
				m.screen = "home"
			}
			return m, nil
		}
		switch m.screen {
		case "preview":
			if key == "enter" {
				return m.requestSubmission()
			}
		case "confirm":
			if key == "y" && m.width >= 30 && m.height >= 6 {
				return m.startCall()
			}
			if key == "pgup" || key == "pgdown" {
				m.confirmation.view, _ = m.confirmation.view.Update(msg)
			}
		case "home":
			switch key {
			case "up":
				m.cursor = max(0, m.cursor-1)
			case "down":
				m.cursor = min(len(m.choices)-1, m.cursor+1)
			case "enter":
				m.screen = m.choices[m.cursor]
				if m.cursor == 0 {
					m.screen, m.appCursor = "apps", 0
					m.readLibrary()
				}
			}
		case "apps":
			switch key {
			case "up":
				m.appCursor = max(0, m.appCursor-1)
			case "down":
				m.appCursor = max(0, min(len(m.files)-1, m.appCursor+1))
			case "enter":
				if m.loading || len(m.files) == 0 {
					return m, nil
				}
				if strings.TrimSpace(m.server) == "" {
					m.err = fmt.Errorf("start Tuilor with --server /path/to/trusted-mcp-server to open an app")
					return m, nil
				}
				// The request owns its context; Esc and Close cancel its lifetime.
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				m.cancel, m.done = cancel, make(chan struct{})
				m.screen, m.loading, m.err = "loading", true, nil
				path, server, done := filepath.Join(m.appsDir, m.files[m.appCursor]), m.server, m.done
				return m, func() tea.Msg {
					defer close(done)
					defer cancel()
					return loadApp(ctx, path, server)
				}
			}
		}
	}
	if m.screen == "preview" {
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) readLibrary() {
	m.files, m.err = nil, nil
	entries, err := os.ReadDir(m.appsDir) // ReadDir sorts entries by filename.
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		m.err = fmt.Errorf("read app directory: %w", err)
		return
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && filepath.Ext(entry.Name()) == ".json" {
			m.files = append(m.files, entry.Name())
		}
	}
}

func loadApp(ctx context.Context, path, server string) loadedAppMsg {
	data, err := os.ReadFile(path)
	if err != nil {
		return loadedAppMsg{err: fmt.Errorf("read app: %w", err)}
	}
	tools, err := mcpclient.ListTools(ctx, server) // Closes the session before returning.
	if err != nil {
		return loadedAppMsg{err: err}
	}
	capabilities := make([]app.Capability, 0, len(tools))
	for _, tool := range tools {
		capabilities = append(capabilities, app.Capability{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
	}
	definition, err := app.Parse(data, capabilities)
	if err == nil {
		err = ctx.Err()
	}
	return loadedAppMsg{definition: definition, err: err, tools: tools}
}

func (m Model) View() string {
	if m.quitting {
		return "Closing MCP connection…\n"
	}
	switch m.screen {
	case "home":
		return "Welcome to Tuilor!\n\n" + menu(m.choices, m.cursor, m.height) + "\n↑↓ Navigate • Enter Select • q Quit\n"
	case "apps":
		body := menu(m.files, m.appCursor, m.height)
		if len(m.files) == 0 && m.err == nil {
			body = "No saved apps. Generate one with tuilor create.\n"
		}
		if m.loading {
			body += "Stopping previous validation…\n"
		}
		if m.err != nil {
			body += "\n" + escapeControls(m.err.Error()) + "\n"
		}
		return "My Apps\n\n" + body + "\n↑↓ Navigate • Enter Open • Esc Back • q Quit\n"
	case "loading":
		return "Validating app against MCP tools…\n\nEsc Cancel • Ctrl+C Quit\n"
	case "preview":
		return m.preview.View()
	case "confirm":
		return m.confirmation.View()
	case "calling":
		return "Running MCP tool…\n\nEsc Cancel call • Ctrl+C Quit\n"
	case "canceling":
		return "Canceling MCP call and closing connection…\n\nThe tool may already have run.\n"
	default:
		return "◆ TUILOR\n\n" + m.screen + "\n\nComing soon...\n\nEsc Back • q Quit\n"
	}
}

func menu(items []string, cursor, height int) string {
	var output strings.Builder
	count := max(1, height-7)
	start := max(0, cursor-count+1)
	for i := start; i < min(len(items), start+count); i++ {
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		fmt.Fprintf(&output, "%s%s\n", prefix, escapeControls(items[i]))
	}
	return output.String()
}

func escapeControls(text string) string {
	var output strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) {
			fmt.Fprintf(&output, "\\u%04x", r)
		} else {
			output.WriteRune(r)
		}
	}
	return output.String()
}

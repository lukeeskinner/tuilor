package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lukeeskinner/tuilor/internal/codeapp"
)

type Model struct {
	workspace                         string
	codeProject                       *codeapp.Project
	codeSpec                          *codeapp.Spec
	codeError, codePrompt, codeReturn string
	codeCursor                        int
	launchCode                        bool
	appsDir                           string
	choices                           []string
	cursor, appCursor                 int
	screen                            string
	files                             []string
	fileLabels                        []string
	err                               error
	creator                           *creationModel
	width, height                     int
	cancel                            context.CancelFunc
	done                              chan struct{}
	loading, quitting                 bool
}

type loadedAppMsg struct {
	code *codeapp.Project
	err  error
}

func NewModel(appsDir string) Model {
	return Model{appsDir: appsDir,
		choices: []string{"My Apps", "Create App", "MCP servers", "Settings"},
		screen:  "home", width: 80, height: 24,
	}
}

func (m Model) Init() tea.Cmd { return nil }

// Close waits for the pending operation's cleanup after external cancellation
// or a terminal error. The CLI calls it on the final model returned by Bubble Tea.
func (m Model) Close() error {
	if m.cancel == nil {
		return nil
	}
	m.cancel()
	select {
	case <-m.done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("background operation did not finish cleanup within 5 seconds")
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		if m.creator != nil {
			m.creator.resize(m.width, m.height)
		}
	case codePreparedMsg:
		return m.codePrepared(msg)
	case codeBuiltMsg:
		return m.codeBuilt(msg)
	case codeSavedMsg:
		m.loading, m.cancel, m.done = false, nil, nil
		if m.quitting {
			return m, tea.Quit
		}
		m.screen = "codereview"
		m.codeCursor = 0
		if msg.err != nil {
			m.codeError = "Save failed: " + msg.err.Error()
		} else {
			m.codeProject = msg.project
			m.codeError = ""
		}
		return m, nil
	case loadedAppMsg:
		m.loading, m.cancel, m.done = false, nil, nil
		if m.quitting {
			return m, tea.Quit
		}
		if m.screen != "loading" {
			return m, nil
		} // Esc already left this request.
		if msg.code != nil && msg.err == nil {
			m.codeProject = msg.code
			spec := msg.code.Spec
			m.codeSpec = &spec
			m.codePrompt = "Build " + spec.Name + "; preserve the existing app behavior and data."
			m.codeError = ""
			if msg.code.Ready() {
				m.launchCode = true
				return m, tea.Quit
			}
			return m.beginCodeBuild(msg.code, codeapp.Toolchain{})
		}
		m.err, m.screen = msg.err, "apps"
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" || (key == "q" && m.screen != "create" && m.screen != "codegenerating" && m.screen != "codebuilding" && m.screen != "codecancel") {
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
			case "codereview":
				return m.leaveCode()
			case "codegenerating", "codebuilding", "codecancel":
				m.cancel()
				m.screen = "codecancel"
			case "saving":
				return m, nil
			case "create":
				m.creator.prompt.Blur()
				m.screen = "home"
			case "loading":
				m.cancel()
				m.screen = "apps"
			default:
				m.screen = "home"
			}
			return m, nil
		}
		switch m.screen {
		case "codereview":
			return m.codeReviewKey(key)
		case "create":
			if key == "ctrl+g" {
				return m.beginGeneration()
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
				if m.cursor == 1 {
					if m.creator == nil {
						m.creator = newCreation()
					}
					m.creator.resize(m.width, m.height)
					m.screen = "create"
					return m, m.creator.prompt.Focus()
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
				// The request owns its context; Esc and Close cancel its lifetime.
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				m.cancel, m.done = cancel, make(chan struct{})
				m.screen, m.loading, m.err = "loading", true, nil
				path, done := filepath.Join(m.appsDir, m.files[m.appCursor]), m.done
				return m, func() tea.Msg {
					defer close(done)
					defer cancel()
					return loadApp(ctx, path)
				}
			}
		}
	}
	if m.screen == "create" {
		var cmd tea.Cmd
		m.creator.prompt, cmd = m.creator.prompt.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Stopping current operation…\n"
	}
	switch m.screen {
	case "home":
		return "Welcome to Tuilor!\n\n" + menu(m.choices, m.cursor, m.height) + "\n↑↓ Navigate • Enter Select • q Quit\n"
	case "apps":
		body := menu(m.fileLabels, m.appCursor, m.height)
		if len(m.files) == 0 && m.err == nil {
			body = "No saved apps. Choose Create App on the home screen.\n"
		}
		if m.loading {
			body += "Stopping previous app load…\n"
		}
		if m.err != nil {
			body += "\n" + escapeControls(m.err.Error()) + "\n"
		}
		return "My Apps\n\n" + body + "\n↑↓ Navigate • Enter Open • Esc Back • q Quit\n"
	case "codegenerating":
		return "Checking Go and generating application source…\n\nOpenAI request is billable. Esc Cancel · Ctrl+C Quit\n"
	case "codebuilding":
		return "Preparing dependencies and compiling app…\n\nA successful build opens automatically. Esc Cancel · Ctrl+C Quit\n"
	case "codecancel":
		return "Canceling operation and waiting for cleanup…\n"
	case "codereview":
		return m.codeView()
	case "create":
		return m.creator.View(m.width, m.height)
	case "saving":
		return "Saving app…\n"
	case "loading":
		return "Opening app project…\n\nEsc Cancel • Ctrl+C Quit\n"
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

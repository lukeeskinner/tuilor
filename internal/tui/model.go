package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/codeapp"
	"github.com/lukeeskinner/tuilor/internal/connections"
	"github.com/lukeeskinner/tuilor/internal/generator"
	"github.com/lukeeskinner/tuilor/internal/localapp"
)

type Model struct {
	api                               *apiModel
	settingsCursor                    int
	modifier                          *creationModel
	revisionBase                      *codeapp.Project
	modifying                         bool
	workspace                         string
	runtime                           *Runtime
	localStore                        *localapp.Store
	codeProject                       *codeapp.Project
	codeSpec                          *codeapp.Spec
	codeError, codePrompt, codeReturn string
	codeCursor                        int
	launchCode                        bool
	appsDir                           string
	choices                           []string
	cursor, appCursor                 int
	introFrame                        int
	screen                            string
	files                             []string
	fileLabels                        []string
	err                               error
	creator                           *creationModel
	mcp                               *connectionModel
	width, height                     int
	cancel                            context.CancelFunc
	done                              chan struct{}
	loading, quitting                 bool
}

type loadedAppMsg struct {
	local *localapp.Bundle
	code  *codeapp.Project
	err   error
}

func NewModel(appsDir string) Model {
	screen := "boot"
	if os.Getenv("TUILOR_REDUCED_MOTION") == "1" || os.Getenv("TERM") == "dumb" {
		screen = "home"
	}
	return Model{appsDir: appsDir,
		choices: []string{"My Apps", "Create App", "Settings"},
		screen:  screen, width: 80, height: 24,
	}
}

func (m Model) Init() tea.Cmd {
	if m.screen == "boot" {
		return introTick()
	}
	return nil
}

// Close waits for the pending operation's cleanup after external cancellation
// or a terminal error. The CLI calls it on the final model returned by Bubble Tea.
func (m Model) Close() error {
	if m.cancel != nil {
		m.cancel()
		select {
		case <-m.done:
		case <-time.After(8 * time.Second):
			return fmt.Errorf("background operation did not finish cleanup within 8 seconds")
		}
	}
	var err error
	if m.runtime != nil {
		err = errors.Join(err, m.runtime.Close())
	}
	if m.localStore != nil {
		err = errors.Join(err, m.localStore.Close())
	}
	return err
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case apiMsg:
		return m.apiResult(msg)
	case apiURLMsg:
		if m.api != nil && msg.request == m.api.request && m.loading && !m.api.cancelled {
			m.api.url = msg.url
			m.api.code = msg.code
		}
		return m, nil
	case connectionMsg:
		return m.connectionResult(msg)
	case connectionURLMsg:
		if m.mcp != nil && msg.request == m.mcp.request && !m.mcp.cancelled && m.loading {
			m.mcp.authURL = msg.url
			m.mcp.viewportText = "Open this sign-in URL if the browser did not open:\n\n" + escapeControls(msg.url)
			m.mcp.viewportWidth = 0
			m.mcp.viewport.GotoTop()
			return m, waitConnectionURL(msg.request, msg.events)
		}
		return m, nil
	case introTickMsg:
		if m.screen != "boot" {
			return m, nil
		}
		m.introFrame++
		if m.introFrame >= 8 {
			m.screen = "home"
			return m, nil
		}
		return m, introTick()
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		if m.runtime != nil {
			m.runtime.Update(msg)
		}
		if m.creator != nil {
			m.creator.resize(m.width, m.height)
		}
		if m.modifier != nil {
			m.modifier.resize(m.width, m.height)
		}
		if m.api != nil {
			for i := range m.api.inputs {
				m.api.inputs[i].Width = max(1, studioWidth(m.width)-6)
			}
		}
		if m.mcp != nil && m.mcp.form != nil {
			m.mcp.form.resize(m.width)
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
			m.revisionBase = nil
			m.modifying = false
			if msg.project != nil {
				spec := msg.project.Spec
				m.codeSpec = &spec
			}
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
		if msg.local != nil && msg.err == nil {
			if err := m.openLocal(*msg.local); err != nil {
				m.err = err
				m.screen = "apps"
			}
			return m, nil
		}
		if msg.code != nil && msg.err == nil {
			m.codeProject = msg.code
			spec := msg.code.Spec
			m.codeSpec = &spec
			m.codePrompt = "Build " + spec.Name + "; preserve the existing app behavior and data."
			m.codeError = ""
			m.codeReturn = "apps"
			m.screen = "codeconfirm"
			return m, nil
		}
		m.err, m.screen = msg.err, "apps"
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" || (key == "q" && m.screen != "apiform" && m.screen != "apibusy" && m.screen != "modify" && m.screen != "localapp" && m.screen != "mcpform" && m.screen != "create" && m.screen != "codegenerating" && m.screen != "codebuilding" && m.screen != "codecancel") {
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
		if m.api != nil && strings.HasPrefix(m.screen, "api") {
			return m.apiKey(msg)
		}
		if m.mcp != nil && strings.HasPrefix(m.screen, "mcp") {
			return m.connectionKey(msg)
		}
		if m.runtime != nil && m.screen == "localapp" {
			return m.updateLocal(msg)
		}
		if m.screen == "boot" {
			m.screen = "home"
			return m, nil
		}
		if key == "esc" {
			m.err = nil
			switch m.screen {
			case "coderestoreconfirm":
				m.screen = "codereview"
			case "codeconfirm":
				m.screen = m.codeReturn
				if m.screen == "" {
					m.screen = "apps"
				}
			case "codereview":
				return m.leaveCode()
			case "codegenerating", "codebuilding", "codecancel":
				m.cancel()
				m.screen = "codecancel"
			case "saving":
				return m, nil
			case "modify":
				m.screen = "codereview"
				m.modifying = false
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
		case "coderestoreconfirm":
			if key == "enter" {
				m.loading = true
				m.screen = "saving"
				ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
				m.cancel = cancel
				m.done = make(chan struct{})
				done := m.done
				p := m.codeProject
				workspace := m.workspace
				return m, func() tea.Msg {
					defer close(done)
					defer cancel()
					saved, err := p.Restore(ctx, workspace)
					return codeSavedMsg{saved, err}
				}
			}
		case "codeconfirm":
			if key == "enter" {
				if m.codeProject.Ready() {
					return m.prepareCodeLaunch()
				}
				return m.beginCodeBuild(m.codeProject, codeapp.Toolchain{})
			}
		case "settings":
			if key == "up" {
				m.settingsCursor = 0
			}
			if key == "down" {
				m.settingsCursor = 1
			}
			if key == "enter" && m.settingsCursor == 0 {
				m.api = newAPI()
				return m.beginAPI("load")
			}
			if key == "enter" {
				if m.mcp == nil {
					m.mcp = newConnections()
				}
				m.screen = "mcp"
				return m.beginConnection("load", connections.Server{})
			}
		case "codereview":
			return m.codeReviewKey(key)
		case "modify":
			if key == "ctrl+g" {
				provider, err := generator.NewOpenAIFromEnv()
				if err != nil {
					m.modifier.setMessage(err.Error(), m.width, m.height)
					return m, nil
				}
				m.modifying = true
				return m.beginCodeGeneration(provider, false)
			}
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
				if m.loading {
					return m, nil
				}
				m.screen = m.choices[m.cursor]
				if m.cursor == 0 {
					m.screen, m.appCursor = "apps", 0
					m.readLibrary()
				}
				if m.cursor == 1 {
					m.modifying = false
					m.revisionBase = nil
					if m.creator == nil {
						m.creator = newCreation()
					}
					m.creator.resize(m.width, m.height)
					m.screen = "create"
					return m, m.creator.focus()
				}
				if m.cursor == 2 {
					m.screen = "settings"
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
	if m.runtime != nil && m.screen == "localapp" {
		return m.updateLocal(msg)
	}
	if m.screen == "create" {
		var cmd tea.Cmd
		m.creator.prompt, cmd = m.creator.prompt.Update(msg)
		return m, cmd
	}
	if m.screen == "modify" {
		var cmd tea.Cmd
		m.modifier.prompt, cmd = m.modifier.prompt.Update(msg)
		return m, cmd
	}
	if m.screen == "apiform" {
		return m, m.updateAPIInputs(msg)
	}
	if m.screen == "mcpform" {
		return m, m.mcp.form.update(msg)
	}

	return m, nil
}

func (m Model) View() string {
	if m.api != nil && strings.HasPrefix(m.screen, "api") {
		return m.apiView()
	}
	if m.quitting {
		return "Stopping current operation…\n"
	}
	if m.mcp != nil && strings.HasPrefix(m.screen, "mcp") {
		return m.connectionsView()
	}
	switch m.screen {
	case "localapp":
		return m.runtime.View()
	case "coderestoreconfirm":
		return m.welcomeCanvas("Restore previous code with current local data?\nGitHub changes will not be undone.\n\nEnter Restore · Esc Cancel")
	case "codeconfirm":
		return m.welcomeCanvas(goAppNotice(m.codeProject.Spec.Name))
	case "settings":
		return m.welcomeCanvas(studioHeader("Settings", studioWidth(m.width)) + "\n\n" + menu([]string{"Connections", "Advanced connections (MCP)"}, m.settingsCursor, m.height) + "\nEnter Open · Esc Back · q Quit")
	case "boot":
		return m.bootView()
	case "home":
		return m.homeView()
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
	case "codegenerating", "codebuilding", "codecancel", "saving":
		return m.generationView()
	case "codereview":
		return m.codeView()
	case "modify":
		return m.welcomeCanvas(strings.Replace(m.modifier.View(m.width, m.height), "Create App", "Modify App", 1))
	case "create":
		return m.welcomeCanvas(m.creator.View(m.width, m.height))
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
		label := prefix + escapeControls(items[i])
		if i == cursor {
			label = lipgloss.NewStyle().Foreground(welcomeMint).Bold(true).Render(label)
		}
		fmt.Fprintln(&output, label)
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

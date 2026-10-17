package tui

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/codeapp"
	"github.com/lukeeskinner/tuilor/internal/generator"
)

type codePreparedMsg struct {
	project *codeapp.Project
	spec    codeapp.Spec
	tool    codeapp.Toolchain
	err     error
}
type codeBuiltMsg struct {
	project *codeapp.Project
	err     error
}
type codeSavedMsg struct {
	project *codeapp.Project
	err     error
}

// WithWorkspace gives the CLI ownership of all unsaved app definitions and data,
// as well as any retained legacy project work.
func (m Model) WithWorkspace(path string) Model { m.workspace = path; return m }
func (m Model) CodeLaunch() *codeapp.Project {
	if m.launchCode {
		return m.codeProject
	}
	return nil
}
func (m Model) CodeReturned(err error) Model {
	m.launchCode = false
	m.screen = "codereview"
	m.codeCursor = 0
	if err != nil {
		m.codeError = err.Error()
	} else {
		m.codeError = ""
	}
	return m
}

// Source generation and explicit repair share preparation and build stages.
// Terminal handoff remains the CLI's responsibility after a successful build.
func (m Model) beginCodeGeneration(provider generator.Provider, repair bool) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	prompt := ""
	if m.creator != nil {
		prompt = m.creator.prompt.Value()
	}
	if m.modifying && m.modifier != nil {
		prompt = m.modifier.prompt.Value()
	}
	var previous *codeapp.Spec
	var old *codeapp.Project
	if m.modifying && !repair {
		previous = &m.revisionBase.Spec
		old = m.revisionBase
	}
	if repair {
		prompt = m.codePrompt
		previous = m.codeSpec
		old = m.codeProject
	}
	if strings.TrimSpace(prompt) == "" {
		return m, nil
	}
	if m.workspace == "" {
		m.codeError = "No project workspace configured."
		m.screen = "codereview"
		return m, nil
	}
	if !repair && !m.modifying {
		m.codeProject, m.codeSpec, m.codeError = nil, nil, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	m.cancel, m.done = cancel, make(chan struct{})
	m.loading, m.screen = true, "codegenerating"
	m.codeReturn = "create"
	if repair || m.modifying {
		m.codeReturn = "codereview"
	}
	if m.modifying && !repair {
		m.codeReturn = "modify"
	}
	m.codePrompt = prompt
	if m.creator != nil {
		m.creator.prompt.Blur()
	}
	directory := filepath.Join(m.workspace, "draft-"+rand.Text())
	diagnostics, done := m.codeError, m.done
	return m, func() tea.Msg {
		defer close(done)
		defer cancel()
		tool, err := codeapp.Preflight(ctx)
		if err != nil {
			return codePreparedMsg{err: err}
		}
		spec, err := generator.GenerateCode(ctx, provider, prompt, previous, diagnostics)
		if err != nil {
			return codePreparedMsg{spec: spec, err: err}
		}
		project, err := codeapp.Create(ctx, directory, spec)
		if err == nil && old != nil {
			err = project.CopyTrial(ctx, old)
		}
		if ctx.Err() != nil {
			os.RemoveAll(directory)
			return codePreparedMsg{err: ctx.Err()}
		}
		return codePreparedMsg{project: project, spec: spec, tool: tool, err: err}
	}
}

func (m Model) codePrepared(msg codePreparedMsg) (tea.Model, tea.Cmd) {
	m.loading, m.cancel, m.done = false, nil, nil
	if m.quitting {
		return m, tea.Quit
	}
	if m.screen == "codecancel" {
		if msg.project != nil && msg.project.Draft {
			_ = os.RemoveAll(msg.project.Dir)
		}
		return m.codeCanceled()
	}
	if msg.spec.Source != "" {
		spec := msg.spec
		m.codeSpec = &spec
		// A failed repair must retain the previous project's trial data.
		if msg.err == nil {
			m.codeProject = msg.project
		}
	}
	if msg.err != nil {
		if errors.Is(msg.err, context.DeadlineExceeded) {
			msg.err = fmt.Errorf("Generation timed out; no app was launched. Your prompt is preserved. Generate again only when ready for another paid request: %w", msg.err)
		}
		m.codeCursor = 0
		if msg.project != nil && msg.project.Draft && msg.project != m.codeProject {
			_ = os.RemoveAll(msg.project.Dir)
		}
		if m.codeSpec != nil {
			m.codeError = msg.err.Error()
			m.screen = "codereview"
			return m, nil
		}
		m.creator.setMessage(msg.err.Error(), m.width, m.height)
		m.screen = "create"
		return m, m.creator.focus()
	}
	m.codeProject = msg.project
	return m.beginCodeBuild(msg.project, msg.tool)
}

func (m Model) beginCodeBuild(project *codeapp.Project, tool codeapp.Toolchain) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	m.cancel, m.done = cancel, make(chan struct{})
	m.loading, m.screen = true, "codebuilding"
	if project.Draft {
		m.codeReturn = "codereview"
	} else {
		m.codeReturn = "apps"
	}
	done := m.done
	return m, func() tea.Msg {
		defer close(done)
		defer cancel()
		var err error
		if tool.Path == "" {
			tool, err = codeapp.Preflight(ctx)
		}
		if err == nil {
			err = project.Build(ctx, tool)
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return codeBuiltMsg{project, err}
	}
}

func (m Model) codeBuilt(msg codeBuiltMsg) (tea.Model, tea.Cmd) {
	m.loading, m.cancel, m.done = false, nil, nil
	if m.quitting {
		return m, tea.Quit
	}
	if m.screen == "codecancel" {
		return m.codeCanceled()
	}
	m.codeProject = msg.project
	m.codeError = ""
	m.codeCursor = 0
	if msg.err != nil {
		m.codeError = msg.err.Error()
		m.screen = "codereview"
		return m, nil
	}
	return m.prepareCodeLaunch()
}

func (m Model) codeCanceled() (tea.Model, tea.Cmd) {
	m.screen = m.codeReturn
	if m.screen == "modify" {
		m.modifier.setMessage("Modification canceled. Your prompt is preserved.", m.width, m.height)
		return m, m.modifier.focus()
	}
	if m.screen == "create" {
		m.creator.setMessage("Generation canceled. Your prompt is preserved.", m.width, m.height)
		return m, m.creator.focus()
	}
	m.codeError = "Operation canceled. No app was launched."
	m.codeCursor = 0
	return m, nil
}

func (m Model) codeChoices() []string {
	var choices []string
	if m.currentCodeReady() {
		if m.codeProject.Draft {
			if m.codeProject.Root != "" {
				choices = append(choices, "Keep Revision")
			} else {
				choices = append(choices, "Save")
			}
		}
		choices = append(choices, "Run Again")
		if !m.codeProject.Draft && m.codeProject.Spec.DataSchema != "" {
			choices = append(choices, "Modify with AI (billable)")
			if m.codeProject.Previous() {
				choices = append(choices, "Restore Previous Version")
			}
		}
	}
	if m.codeError != "" && m.codeSpec != nil {
		choices = append(choices, "Repair with AI (billable)")
	}
	if m.codeProject == nil || m.codeProject.Draft {
		choices = append(choices, "Discard")
	} else {
		choices = append(choices, "Back to My Apps")
	}
	return choices
}

func (m Model) currentCodeReady() bool {
	p, s := m.codeProject, m.codeSpec
	return p != nil && s != nil && s.Name == p.Spec.Name && s.Source == p.Spec.Source && slices.Equal(s.Dependencies, p.Spec.Dependencies) && p.Ready()
}

func (m Model) codeReviewKey(key string) (tea.Model, tea.Cmd) {
	choices := m.codeChoices()
	switch key {
	case "up":
		m.codeCursor = max(0, m.codeCursor-1)
	case "down":
		m.codeCursor = min(len(choices)-1, m.codeCursor+1)
	case "enter":
		switch choices[min(m.codeCursor, len(choices)-1)] {
		case "Run Again":
			m.codeReturn = "codereview"
			m.screen = "codeconfirm"
			return m, nil
		case "Modify with AI (billable)":
			m.revisionBase = m.codeProject
			m.modifier = newCreation()
			m.modifier.prompt.Placeholder = "Describe the change. Existing data format will be preserved."
			m.modifier.resize(m.width, m.height)
			m.screen = "modify"
			return m, m.modifier.focus()
		case "Restore Previous Version":
			m.screen = "coderestoreconfirm"
			return m, nil
		case "Save", "Keep Revision":
			m.loading, m.screen = true, "saving"
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			m.cancel, m.done = cancel, make(chan struct{})
			project, directory, done := m.codeProject, m.appsDir, m.done
			return m, func() tea.Msg {
				defer close(done)
				defer cancel()
				saved, err := project.Save(ctx, directory)
				return codeSavedMsg{saved, err}
			}
		case "Repair with AI (billable)":
			provider, err := generator.NewOpenAIFromEnv()
			if err != nil {
				m.codeError = err.Error()
				return m, nil
			}
			if m.creator == nil {
				m.creator = newCreation()
			}
			return m.beginCodeGeneration(provider, true)
		default:
			return m.leaveCode()
		}
	}
	return m, nil
}

func (m Model) leaveCode() (tea.Model, tea.Cmd) {
	if m.revisionBase != nil && (m.codeProject == nil || !m.codeProject.Draft) {
		m.codeProject = m.revisionBase
		spec := m.revisionBase.Spec
		m.codeSpec = &spec
		m.revisionBase = nil
		m.modifying = false
		m.codeError = "Revision discarded. Saved app unchanged."
		m.screen = "codereview"
		return m, nil
	}
	if m.codeProject != nil && !m.codeProject.Draft {
		path := m.codeProject.Path()
		m.codeProject = nil
		m.codeSpec = nil
		m.revisionBase = nil
		m.modifying = false
		m.returnToLibrary(path)
		return m, nil
	}
	// The directory is a host-created child of the CLI-owned session workspace.
	if m.codeProject != nil {
		if err := os.RemoveAll(m.codeProject.Dir); err != nil {
			m.codeError = fmt.Sprintf("Discard failed: %s", err)
			return m, nil
		}
	}
	if m.revisionBase != nil {
		m.codeProject = m.revisionBase
		spec := m.revisionBase.Spec
		m.codeSpec = &spec
		m.revisionBase = nil
		m.modifying = false
		m.codeError = "Revision discarded. Saved app unchanged."
		m.screen = "codereview"
		return m, nil
	}
	m.codeProject, m.codeSpec, m.codeError = nil, nil, ""
	if m.creator == nil {
		m.creator = newCreation()
	}
	m.creator.setMessage("Candidate discarded. Your prompt is preserved.", m.width, m.height)
	m.screen = "create"
	return m, m.creator.focus()
}

func (m Model) codeView() string {
	name := "Generated Go app"
	if m.codeSpec != nil {
		name = m.codeSpec.Name
	}
	status := "Unsaved project · source and trial data stay together"
	if m.codeProject != nil && m.codeProject.Draft && m.codeProject.Root != "" {
		status = "Unsaved revision · trial data isolated · GitHub changes are real"
	}
	if m.codeProject != nil && !m.codeProject.Draft {
		status = "Saved: " + m.codeProject.Path()
	}
	panel := studioWidth(m.width)
	line := lipgloss.NewStyle().MaxWidth(panel).MaxHeight(1)
	details := ""
	if m.codeError != "" {
		details = lipgloss.NewStyle().Foreground(welcomePeach).Width(panel).MaxHeight(max(1, m.height-14)).Render(escapeControls(m.codeError)) + "\n\n"
	}
	header := ""
	if m.width >= 60 && m.height >= 20 {
		header = studioHeader("Your app", panel) + "\n\n"
	}
	body := header + line.Foreground(welcomeInk).Bold(true).Render(escapeControls(name)) + "\n" +
		line.Foreground(welcomeLilac).Render(escapeControls(status)) + "\n\n" + details +
		menu(m.codeChoices(), m.codeCursor, m.height) + "\n" +
		lipgloss.NewStyle().Width(panel).Foreground(welcomeMuted).Render("↑↓ Choose · Enter Select · Esc Back · Ctrl+C Quit")
	return m.welcomeCanvas(body)
}

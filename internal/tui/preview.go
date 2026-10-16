package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/localapp"
)

// previewModel owns editable state separately from the saved definition.
// Only the parent model performs MCP operations; this model has no MCP or AI client.
type previewModel struct {
	definition    *app.Definition
	page          int
	inputs        [][]textinput.Model
	focus         []int
	targets       [][]focusTarget
	results       map[[2]int]string
	resultLine    int
	viewport      viewport.Model
	width, height int
	draft         bool
	savedPath     string
	saveMessage   string
	local         *localapp.Store
	selected      map[[2]int]int
}

// A submit button has input == -1. This also makes zero-input forms accessible.
type focusTarget struct{ form, input int }

func newPreview(definition *app.Definition, width, height int) (previewModel, tea.Cmd) {
	p := previewModel{
		definition: definition,
		inputs:     make([][]textinput.Model, len(definition.Pages)),
		focus:      make([]int, len(definition.Pages)),
		targets:    make([][]focusTarget, len(definition.Pages)),
		results:    make(map[[2]int]string),
		viewport:   viewport.New(1, 1),
		selected:   make(map[[2]int]int),
	}
	if definition.Version == 2 {
		p.local, _ = localapp.New(definition)
	}
	for page, content := range definition.Pages {
		for form, component := range content.Components {
			if component.Type == "list" {
				p.targets[page] = append(p.targets[page], focusTarget{form, -2})
			}
			for range component.Fields {
				input := textinput.New()
				input.Prompt = "> "
				p.targets[page] = append(p.targets[page], focusTarget{form, len(p.inputs[page])})
				p.inputs[page] = append(p.inputs[page], input)
			}
			if component.Type == "form" {
				p.targets[page] = append(p.targets[page], focusTarget{form, -1})
			}
		}
	}
	p.resize(width, height)
	cmd := p.focusInput()
	p.renderContent(true)
	return p, cmd
}

func (p *previewModel) resize(width, height int) {
	p.width, p.height = width, height
	p.viewport.Width, p.viewport.Height = max(1, width), max(1, height-4)
	for page := range p.inputs {
		for i := range p.inputs[page] {
			p.inputs[page][i].Width = max(1, width-4)
		}
	}
}

func (p *previewModel) focusInput() tea.Cmd {
	target, ok := p.activeTarget()
	if !ok || target.input < 0 {
		return nil
	}
	return p.inputs[p.page][target.input].Focus()
}

func (p previewModel) activeTarget() (focusTarget, bool) {
	if len(p.targets[p.page]) == 0 {
		return focusTarget{}, false
	}
	return p.targets[p.page][p.focus[p.page]], true
}

func (p *previewModel) blurInput() {
	if target, ok := p.activeTarget(); ok && target.input >= 0 {
		p.inputs[p.page][target.input].Blur()
	}
}

func (p previewModel) Update(msg tea.Msg) (previewModel, tea.Cmd) {
	ensureVisible := false
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.resize(msg.Width, msg.Height)
		p.renderContent(true)
		return p, nil
	case tea.KeyMsg:
		if target, ok := p.activeTarget(); ok && target.input == -2 && (msg.String() == "up" || msg.String() == "down") {
			key := [2]int{p.page, target.form}
			count := len(p.local.Records(p.definition.Pages[p.page].Components[target.form].Collection))
			direction := 1
			if msg.String() == "up" {
				direction = -1
			}
			p.selected[key] = max(0, min(p.selected[key]+direction, count-1))
			p.renderContent(true)
			return p, nil
		}
		switch msg.String() {
		case "ctrl+n", "ctrl+p":
			p.blurInput()
			direction := 1
			if msg.String() == "ctrl+p" {
				direction = -1
			}
			p.page = (p.page + direction + len(p.definition.Pages)) % len(p.definition.Pages)
			cmd := p.focusInput()
			p.viewport.GotoTop()
			p.renderContent(true)
			return p, cmd
		case "tab", "shift+tab":
			count := len(p.targets[p.page])
			if count == 0 {
				return p, nil
			}
			p.blurInput()
			direction := 1
			if msg.String() == "shift+tab" {
				direction = -1
			}
			p.focus[p.page] = (p.focus[p.page] + direction + count) % count
			cmd := p.focusInput()
			p.renderContent(true)
			return p, cmd
		case "pgup", "pgdown":
			var cmd tea.Cmd
			p.viewport, cmd = p.viewport.Update(msg)
			return p, cmd
		}
		ensureVisible = true
	}
	var cmd tea.Cmd
	if target, ok := p.activeTarget(); ok && target.input >= 0 {
		i := target.input
		p.inputs[p.page][i], cmd = p.inputs[p.page][i].Update(msg)
	}
	p.renderContent(ensureVisible)
	return p, cmd
}

func (p *previewModel) renderContent(ensureVisible bool) {
	var lines []string
	appendText := func(text string) {
		wrapped := lipgloss.NewStyle().Width(p.viewport.Width).Render(escapeControls(text))
		lines = append(lines, strings.Split(wrapped, "\n")...)
	}
	fieldIndex, focusedLine := 0, -1
	if p.saveMessage != "" {
		appendText(p.saveMessage)
		lines = append(lines, "")
	}
	target, hasTarget := p.activeTarget()
	p.resultLine = -1
	for form, component := range p.definition.Pages[p.page].Components {
		switch component.Type {
		case "text":
			appendText(component.Text)
		case "list":
			appendText(component.Title)
			rows := p.local.Records(component.Collection)
			key := [2]int{p.page, form}
			p.selected[key] = max(0, min(p.selected[key], len(rows)-1))
			if len(rows) == 0 {
				if hasTarget && target.form == form && target.input == -2 {
					focusedLine = len(lines)
				}
				appendText("No records yet.")
			}
			for i, row := range rows {
				prefix := "  "
				if hasTarget && target.form == form && target.input == -2 && i == p.selected[key] {
					prefix = "> "
					focusedLine = len(lines)
				}
				if component.StatusField != "" {
					if row.Values[component.StatusField] == true {
						prefix += "[x] "
					} else {
						prefix += "[ ] "
					}
				}
				appendText(prefix + row.Values[component.LabelField].(string))
			}
		case "form":
			appendText(component.Title)
			for _, field := range component.Fields {
				appendText(field.Label)
				if hasTarget && fieldIndex == target.input {
					focusedLine = len(lines)
				}
				lines = append(lines, p.inputs[p.page][fieldIndex].View())
				fieldIndex++
			}
			label := "Add"
			if component.Action != nil {
				label = "Submit: " + component.Action.Tool
			}
			button := "  [ " + label + " ]"
			if hasTarget && target.form == form && target.input < 0 {
				button = "> [ " + label + " ]"
				focusedLine = len(lines)
			}
			appendText(button)
			if result, ok := p.results[[2]int{p.page, form}]; ok {
				if hasTarget && target.form == form {
					p.resultLine = len(lines)
				}
				for _, line := range strings.Split(result, "\n") {
					appendText(line)
				}
			}
		}
		lines = append(lines, "")
	}
	p.viewport.SetContent(strings.Join(lines, "\n"))
	if ensureVisible && focusedLine >= 0 {
		if focusedLine < p.viewport.YOffset {
			p.viewport.SetYOffset(max(0, focusedLine-1))
		} else if focusedLine >= p.viewport.YOffset+p.viewport.Height {
			p.viewport.SetYOffset(focusedLine - p.viewport.Height + 1)
		}
	}
}

func (p previewModel) View() string {
	line := lipgloss.NewStyle().MaxWidth(max(1, p.width)).MaxHeight(1)
	if p.width < 30 || p.height < 6 {
		return line.Render("Enlarge terminal to view app.")
	}
	header := escapeControls(p.definition.Name) + " — " + escapeControls(p.definition.Pages[p.page].Title)
	status := fmt.Sprintf("App · page %d/%d · calls require confirmation", p.page+1, len(p.definition.Pages))
	help := "Tab/Shift+Tab Focus · Enter Submit · PgUp/PgDn Scroll"
	if p.local != nil {
		status = fmt.Sprintf("Local app · page %d/%d · changes saved automatically", p.page+1, len(p.definition.Pages))
		help = "Tab Focus · Enter Add · ↑↓ Select · Space Update · Del Delete"
	}
	footer := "Ctrl+N/P Pages · Esc Back · Ctrl+C Quit"
	if p.draft {
		status = fmt.Sprintf("Unsaved app · page %d/%d · Ctrl+S Save", p.page+1, len(p.definition.Pages))
		footer = "Ctrl+N/P Pages · Esc Discard · Ctrl+C Quit"
	}
	return strings.Join([]string{
		line.Render(header), line.Render(status), p.viewport.View(),
		line.Render(help),
		line.Render(footer),
	}, "\n")
}

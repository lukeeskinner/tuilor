package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/app"
)

// previewModel owns editable state separately from the saved definition.
// Only the parent model performs discovery; this model has no MCP or AI client.
type previewModel struct {
	definition    *app.Definition
	page          int
	inputs        [][]textinput.Model
	focus         []int
	viewport      viewport.Model
	width, height int
}

func newPreview(definition *app.Definition, width, height int) (previewModel, tea.Cmd) {
	p := previewModel{
		definition: definition,
		inputs:     make([][]textinput.Model, len(definition.Pages)),
		focus:      make([]int, len(definition.Pages)),
		viewport:   viewport.New(1, 1),
	}
	for page, content := range definition.Pages {
		for _, component := range content.Components {
			for range component.Fields {
				input := textinput.New()
				input.Prompt = "> "
				p.inputs[page] = append(p.inputs[page], input)
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
	if len(p.inputs[p.page]) == 0 {
		return nil
	}
	return p.inputs[p.page][p.focus[p.page]].Focus()
}

func (p previewModel) Update(msg tea.Msg) (previewModel, tea.Cmd) {
	ensureVisible := false
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.resize(msg.Width, msg.Height)
		p.renderContent(true)
		return p, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+n", "ctrl+p":
			if len(p.inputs[p.page]) > 0 {
				p.inputs[p.page][p.focus[p.page]].Blur()
			}
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
			count := len(p.inputs[p.page])
			if count == 0 {
				return p, nil
			}
			p.inputs[p.page][p.focus[p.page]].Blur()
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
	if len(p.inputs[p.page]) > 0 {
		i := p.focus[p.page]
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
	for _, component := range p.definition.Pages[p.page].Components {
		switch component.Type {
		case "text":
			appendText(component.Text)
		case "form":
			appendText(component.Title)
			for _, field := range component.Fields {
				appendText(field.Label)
				if fieldIndex == p.focus[p.page] {
					focusedLine = len(lines)
				}
				lines = append(lines, p.inputs[p.page][fieldIndex].View())
				fieldIndex++
			}
			appendText("Action: " + component.Action.Tool + " (disabled in preview)")
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
		return line.Render("Enlarge terminal to preview.")
	}
	header := escapeControls(p.definition.Name) + " — " + escapeControls(p.definition.Pages[p.page].Title)
	status := fmt.Sprintf("Preview — actions disabled · page %d/%d", p.page+1, len(p.definition.Pages))
	return strings.Join([]string{
		line.Render(header), line.Render(status), p.viewport.View(),
		line.Render("Tab/Shift+Tab Fields · PgUp/PgDn Scroll"),
		line.Render("Ctrl+N/P Pages · Esc Back · Ctrl+C Quit"),
	}, "\n")
}

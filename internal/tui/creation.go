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
	prompt.Placeholder = "What should your app do?\nFor example: a reading tracker with ratings and a yearly goal."
	prompt.Prompt = "› "
	prompt.ShowLineNumbers = false
	style := textarea.Style{
		Base:        lipgloss.NewStyle().Foreground(welcomeInk),
		CursorLine:  lipgloss.NewStyle().Foreground(welcomeInk),
		Text:        lipgloss.NewStyle().Foreground(welcomeInk),
		Prompt:      lipgloss.NewStyle().Foreground(welcomeMint),
		Placeholder: lipgloss.NewStyle().Foreground(welcomeMuted),
	}
	prompt.FocusedStyle, prompt.BlurredStyle = style, style
	prompt.Cursor.Style = lipgloss.NewStyle().Foreground(welcomeMint)
	return &creationModel{prompt: prompt}
}

func studioWidth(width int) int {
	if width < 60 {
		return max(1, width-4)
	}
	return max(1, min(88, width-8))
}

func studioHeader(title string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).MaxHeight(1).Render(
		lipgloss.NewStyle().Foreground(welcomeMint).Bold(true).Render("TUILOR") +
			lipgloss.NewStyle().Foreground(welcomeMuted).Render("  /  ") +
			lipgloss.NewStyle().Foreground(welcomeInk).Render(title))
}

func (c *creationModel) resize(width, height int) {
	overhead, margin, limit := 14, 4, 12
	if width < 60 || height < 20 {
		overhead, margin, limit = 8, 2, 8
	}
	c.prompt.SetWidth(max(1, studioWidth(width)-4))
	c.prompt.SetHeight(max(1, min(limit, height-margin-overhead-lipgloss.Height(c.messageView(width, height)))))
	// Resizing for an error banner must also keep the caret in view.
	if c.prompt.Focused() {
		c.prompt, _ = c.prompt.Update(nil)
	}
}

func (c *creationModel) focus() tea.Cmd {
	cmd := c.prompt.Focus()
	c.prompt, _ = c.prompt.Update(nil)
	return cmd
}

func (c creationModel) messageView(width, height int) string {
	if c.message == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(welcomePeach).Width(studioWidth(width)).
		MaxHeight(max(1, min(2, height-9))).Render("! " + escapeControls(c.message))
}

func (c *creationModel) setMessage(message string, width, height int) {
	c.message = message
	c.resize(width, height)
}

func (c creationModel) View(width, height int) string {
	panel := studioWidth(width)
	line := lipgloss.NewStyle().Width(panel).MaxWidth(panel).MaxHeight(1)
	if width < 30 || height < 10 {
		return line.Foreground(welcomePeach).Render("Enlarge terminal to create an app.")
	}
	compact := width < 60 || height < 20
	rows := []string{studioHeader("Create App", panel)}
	if !compact {
		rows = append(rows, "", line.Foreground(welcomeInk).Bold(true).Render("Make something yours."),
			line.Foreground(welcomeMuted).Render("Describe what it does and how you want to use it."), "",
			line.Foreground(welcomeLilac).Render("YOUR BRIEF"))
	}
	model := line.Foreground(welcomeLilac).Render("Model: " + escapeControls(generator.ConfiguredModel()) + " · Generated Go app")
	billing := line.Foreground(welcomeMuted).Render("OpenAI request (billable).")
	permissions := line.Foreground(welcomePeach).Render("Runs Go with your OS access.")
	if compact {
		model = line.Foreground(welcomeLilac).Render("Model: " + escapeControls(generator.ConfiguredModel()))
		rows = append(rows, model, billing, permissions)
	}
	border := welcomeRule
	if c.prompt.Focused() {
		border = welcomeMint
	}
	editor := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(0, 1).Width(panel - 2).Render(c.prompt.View())
	rows = append(rows, editor)
	if !compact {
		rows = append(rows, "", model, billing, permissions)
	}
	if message := c.messageView(width, height); message != "" {
		rows = append(rows, message)
	}
	generate := lipgloss.NewStyle().Foreground(welcomeMint).Bold(true).Render("Ctrl+G Generate")
	if compact {
		editing := " · Enter Newline"
		if panel < 32 {
			editing = " · ↵ Line"
		}
		rows = append(rows, line.Render(generate+editing), line.Foreground(welcomeMuted).Render("Esc Back · Ctrl+C Quit"))
	} else {
		rows = append(rows, "", line.Render(generate+lipgloss.NewStyle().Foreground(welcomeMuted).Render(" · Enter Newline · Esc Back · Ctrl+C Quit")))
	}
	return strings.Join(rows, "\n")
}

// The current state is the progress indicator; no percentage or invented work is shown.
func (m Model) generationView() string {
	panel := studioWidth(m.width)
	line := lipgloss.NewStyle().Width(panel).MaxWidth(panel).MaxHeight(1)
	title, detail, notice, help := "Checking Go and generating application source…", "Designing the layout and behavior from your brief.", "OpenAI request is billable.", "Esc Cancel · Ctrl+C Quit"
	switch m.screen {
	case "codebuilding":
		title, detail, notice = "Preparing dependencies and compiling app…", "Checking the generated source before it can run.", "A successful build opens automatically."
	case "codecancel":
		title, detail, notice, help = "Canceling operation and waiting for cleanup…", "Waiting for the current operation to stop.", "Your candidate and prompt remain available.", "Ctrl+C Quit"
	case "saving":
		title, detail, notice, help = "Saving app…", "Keeping the source, build, and trial data together.", "Your candidate stays available if saving fails.", "Ctrl+C Quit"
	}
	if m.width < 30 || m.height < 10 {
		return m.welcomeCanvas(line.Foreground(welcomeMint).Render(title) + "\n" + line.Foreground(welcomeMuted).Render(help))
	}
	steps := []string{"01 Describe", "02 Generate", "03 Build & open"}
	active := 1
	if m.screen == "codebuilding" {
		active = 2
	}
	for i := range steps {
		color := lipgloss.TerminalColor(welcomeMuted)
		if i < active {
			color = welcomeMint
		} else if i == active {
			color = welcomePeach
		}
		steps[i] = lipgloss.NewStyle().Foreground(color).Render(steps[i])
	}
	rows := []string{studioHeader("Create App", panel), "", line.Foreground(welcomeInk).Bold(true).Render(title), line.Foreground(welcomeMuted).Render(detail)}
	if m.width >= 60 && m.height >= 20 && m.screen != "saving" && m.screen != "codecancel" {
		rows = append(rows, "", line.Render(strings.Join(steps, "  /  ")), "", line.Foreground(welcomeLilac).Render("YOUR BRIEF"))
		promptLines := strings.Split(m.codePrompt, "\n")
		for i := range promptLines {
			promptLines[i] = escapeControls(promptLines[i])
		}
		rows = append(rows, lipgloss.NewStyle().Width(panel).MaxHeight(3).Foreground(welcomeInk).Render(strings.Join(promptLines, "\n")))
	}
	rows = append(rows, "", line.Foreground(welcomePeach).Render(notice), "", line.Foreground(welcomeMuted).Render(help))
	return m.welcomeCanvas(strings.Join(rows, "\n"))
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

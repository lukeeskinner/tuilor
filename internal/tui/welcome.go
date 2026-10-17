package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Use the terminal's background; dark and light terminals get separate ink colors.
var (
	welcomeInk   = lipgloss.AdaptiveColor{Light: "#25253C", Dark: "#EEEAF7"}
	welcomeMuted = lipgloss.AdaptiveColor{Light: "#626277", Dark: "#9D9BAF"}
	welcomeMint  = lipgloss.AdaptiveColor{Light: "#146848", Dark: "#94F7C5"}
	welcomePeach = lipgloss.AdaptiveColor{Light: "#A44818", Dark: "#FFB38A"}
	welcomeLilac = lipgloss.AdaptiveColor{Light: "#7043A8", Dark: "#CDB5FF"}
	welcomeRule  = lipgloss.AdaptiveColor{Light: "#C6C5D1", Dark: "#454353"}
)

type introTickMsg struct{}

func introTick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return introTickMsg{} })
}

func wordmark() string {
	letters := [][5]string{
		{"█████", "  █  ", "  █  ", "  █  ", "  █  "},
		{"█   █", "█   █", "█   █", "█   █", " ███ "},
		{"███", " █ ", " █ ", " █ ", "███"},
		{"█    ", "█    ", "█    ", "█    ", "█████"},
		{" ███ ", "█   █", "█   █", "█   █", " ███ "},
		{"████ ", "█   █", "████ ", "█  █ ", "█   █"},
	}
	colors := []lipgloss.TerminalColor{welcomeMint, welcomeMint, welcomePeach, welcomePeach, welcomeLilac, welcomeLilac}
	rows := make([]string, 5)
	for row := range rows {
		parts := make([]string, len(letters))
		for i, letter := range letters {
			parts[i] = lipgloss.NewStyle().Foreground(colors[i]).Render(letter[row])
		}
		rows[row] = strings.Join(parts, "  ")
	}
	return strings.Join(rows, "\n")
}

func (m Model) welcomeCanvas(content string) string {
	// Bounding the content first prevents Place from overflowing tiny terminals.
	content = lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) bootView() string {
	width := max(1, min(60, m.width-6))
	if m.width < 36 {
		width = max(1, m.width-2)
	}
	line := lipgloss.NewStyle().MaxWidth(width).MaxHeight(1)
	title := lipgloss.NewStyle().Foreground(welcomeMint).Bold(true).Render("TUILOR")
	if width >= 42 && m.height >= 16 {
		title = wordmark()
	}
	var thread strings.Builder
	for i := 0; i < min(26, width); i++ {
		color := lipgloss.TerminalColor(welcomeRule)
		if i%8 == m.introFrame%8 {
			color = welcomeMint
		} else if (i+3)%8 == m.introFrame%8 {
			color = welcomePeach
		}
		thread.WriteString(lipgloss.NewStyle().Foreground(color).Render("━"))
	}
	rows := []string{title, "", line.Foreground(welcomeInk).Render("Your terminal, made to measure.")}
	if m.height >= 12 {
		rows = append(rows, "", thread.String(), "", line.Foreground(welcomeLilac).Render("A little idea. A whole new interface."))
	}
	skip := "Press any key to skip · q quit"
	if width < 30 {
		skip = "Any key skip · q quit"
	}
	rows = append(rows, "", line.Foreground(welcomeMuted).Render(skip))
	return m.welcomeCanvas(lipgloss.JoinVertical(lipgloss.Center, rows...))
}

func (m Model) homeView() string {
	if m.width < 18 || m.height < 6 {
		return m.welcomeCanvas(lipgloss.NewStyle().Foreground(welcomeMint).Render("TUILOR\nEnlarge terminal\nq quit"))
	}
	width := max(1, min(64, m.width-6))
	if m.width < 36 {
		width = max(1, m.width-2)
	}
	if m.width < 12 {
		width = m.width
	}
	line := lipgloss.NewStyle().Width(width).MaxWidth(width).MaxHeight(1)
	roomy := width >= 42 && m.height >= 22
	descriptions := roomy && m.height >= 24
	var rows []string
	if roomy {
		rows = append(rows, wordmark(), "", line.Foreground(welcomeInk).Render("Welcome to Tuilor!"), line.Foreground(welcomeMuted).Render("Dream it. Build it. Run it."), "")
	} else {
		rows = append(rows, line.Foreground(welcomeMint).Bold(true).Render("TUILOR  /  Welcome"))
		if m.height >= 12 {
			rows = append(rows, line.Foreground(welcomeMuted).Render("Your terminal, made to measure."), "")
		}
	}
	descriptionsText := []string{"Your saved apps. Ready when you are.", "Turn an idea into a working terminal app.", "Your servers, sign-ins, and discovered tools.", "Make yourself at home. Coming soon."}
	// Keep the selected entry and help visible even in a six-row terminal.
	footerRows := 1
	if m.height >= 12 {
		footerRows = 3
	}
	available := m.height - lipgloss.Height(strings.Join(rows, "\n")) - footerRows
	count := max(1, available)
	if descriptions {
		count = max(1, available/2)
	}
	start := max(0, m.cursor-count+1)
	for i := start; i < min(len(m.choices), start+count); i++ {
		label := m.choices[i]
		if i == 3 && width >= 30 {
			label += "  · soon"
		}
		prefix := "  "
		style := line.Foreground(welcomeInk)
		detailStyle := line.Foreground(welcomeMuted)
		if i == m.cursor {
			prefix = "> "
			style = style.Bold(true).Foreground(lipgloss.Color("#172D26")).Background(lipgloss.Color("#94F7C5"))
			detailStyle = detailStyle.Foreground(lipgloss.Color("#284C3D")).Background(lipgloss.Color("#94F7C5"))
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s%02d  %s", prefix, i+1, label)))
		if descriptions {
			rows = append(rows, detailStyle.Render("      "+descriptionsText[i]))
		}
	}
	if m.height >= 12 {
		rows = append(rows, "", line.Foreground(welcomeRule).Render(strings.Repeat("─", width)))
	}
	help := "↑↓ Move · Enter Open · q Quit"
	if width < 30 {
		help = "↑↓ ↵ open · q quit"
	}
	rows = append(rows, line.Foreground(welcomeMuted).Render(help))
	return m.welcomeCanvas(strings.Join(rows, "\n"))
}

package main

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type todoModel struct {
	tasks                   []task
	input                   textinput.Model
	width, height, selected int
	list, confirm           bool
	err                     string
}

func newModel() (tea.Model, error) {
	m := todoModel{input: textinput.New()}
	if err := loadData(&m.tasks); err != nil {
		return nil, err
	}
	m.input.Placeholder = "What needs doing?"
	m.input.Focus()
	return m, nil
}
func (m todoModel) Init() tea.Cmd { return textinput.Blink }
func (m todoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
		m.input.Width = max(1, m.width-4)
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.confirm {
			if key.String() == "y" && len(m.tasks) > 0 {
				next := append([]task{}, m.tasks[:m.selected]...)
				next = append(next, m.tasks[m.selected+1:]...)
				if err := saveData(next); err != nil {
					m.err = err.Error()
				} else {
					m.tasks = next
					m.selected = max(0, min(m.selected, len(next)-1))
					m.err = ""
				}
			}
			m.confirm = false
			return m, nil
		}
		switch key.String() {
		case "tab", "shift+tab":
			m.list = !m.list
			if m.list {
				m.input.Blur()
				return m, nil
			}
			return m, m.input.Focus()
		case "enter":
			if !m.list {
				title := strings.TrimSpace(m.input.Value())
				if title == "" {
					m.err = "Enter a task title."
					return m, nil
				}
				next := append(append([]task{}, m.tasks...), task{ID: rand.Text(), Title: title})
				if err := saveData(next); err != nil {
					m.err = err.Error()
				} else {
					m.tasks = next
					m.input.SetValue("")
					m.err = ""
				}
				return m, nil
			}
		case "up":
			if m.list {
				m.selected = max(0, m.selected-1)
				return m, nil
			}
		case "down":
			if m.list {
				m.selected = min(max(0, len(m.tasks)-1), m.selected+1)
				return m, nil
			}
		case " ":
			if m.list && len(m.tasks) > 0 {
				next := append([]task{}, m.tasks...)
				next[m.selected].Done = true
				if err := saveData(next); err != nil {
					m.err = err.Error()
				} else {
					m.tasks = next
					m.err = ""
				}
				return m, nil
			}
		case "delete":
			if m.list && len(m.tasks) > 0 {
				m.confirm = true
				return m, nil
			}
		}
	}
	if !m.list {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}
func (m todoModel) View() string {
	if m.width < 25 || m.height < 7 {
		return "Todo · Enlarge terminal"
	}
	clip := lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(1)
	heading := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render("My tasks")
	rows := []string{heading, m.input.View(), ""}
	if m.confirm {
		rows = append(rows, "Delete selected task? y Yes · any other key Cancel")
	}
	if m.err != "" {
		rows = append(rows, clip.Render("Error: "+m.err))
	}
	capacity := max(1, m.height-len(rows)-2)
	start := max(0, m.selected-capacity+1)
	if len(m.tasks) == 0 {
		rows = append(rows, "No tasks yet. Add one above.")
	}
	for i := start; i < min(len(m.tasks), start+capacity); i++ {
		mark := "[ ]"
		if m.tasks[i].Done {
			mark = "[x]"
		}
		prefix := "  "
		if m.list && i == m.selected {
			prefix = "> "
		}
		rows = append(rows, clip.Render(fmt.Sprintf("%s%s %s", prefix, mark, m.tasks[i].Title)))
	}
	rows = append(rows, "", clip.Render("Tab Focus · Enter Add · ↑↓ Select · Space Complete · Del Delete"))
	return strings.Join(rows, "\n")
}

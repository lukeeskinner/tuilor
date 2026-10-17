package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/app"
	"github.com/lukeeskinner/tuilor/internal/capability"
)

// Executor is defined where it is consumed. The renderer does not import storage,
// MCP, HTTP, or the generator, and cannot acquire new capabilities itself.
type Executor interface {
	Execute(context.Context, capability.Call) ([]capability.Record, error)
}
type Runtime struct {
	Definition                   app.Definition
	Rows                         []capability.Record
	Selected                     int
	Width, Height                int
	Path                         string
	executor                     Executor
	catalog                      capability.Catalog
	form                         *recordForm
	confirmation                 *capability.Call
	confirmationLabel            string
	actionFocus                  int
	message                      string
	busy                         bool
	cancel                       context.CancelFunc
	done                         chan struct{}
	request                      uint64
	ExitRequested, SaveRequested bool
}
type recordForm struct {
	component app.Component
	inputs    []textinput.Model
	focus     int
	recordID  string
}
type mutationMsg struct {
	request  uint64
	rows     []capability.Record
	err      error
	inserted bool
}

func NewRuntime(definition app.Definition, executor Executor, rows []capability.Record, catalog capability.Catalog) *Runtime {
	return &Runtime{Definition: definition, executor: executor, catalog: catalog, Rows: rows, Width: 80, Height: 24, actionFocus: -1}
}
func (r *Runtime) Init() tea.Cmd { return nil }
func (r *Runtime) Close() error {
	if r.cancel == nil {
		return nil
	}
	r.cancel()
	select {
	case <-r.done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("local operation did not stop within 5 seconds")
	}
}
func (r *Runtime) selected() (capability.Record, bool) {
	if r.Selected < 0 || r.Selected >= len(r.Rows) {
		return capability.Record{}, false
	}
	return r.Rows[r.Selected], true
}
func (r *Runtime) actions() []app.Action {
	actions := append([]app.Action{}, r.Definition.List().Actions...)
	for _, c := range r.Definition.Page.Components {
		if c.Type == "button" {
			actions = append(actions, c.Actions...)
		}
	}
	return actions
}
func (r *Runtime) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	r.ExitRequested, r.SaveRequested = false, false
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.Width, r.Height = max(1, msg.Width), max(1, msg.Height)
		r.resizeForm()
	case mutationMsg:
		if msg.request != r.request {
			return r, nil
		}
		r.busy, r.cancel, r.done = false, nil, nil
		if msg.err != nil {
			r.message = msg.err.Error()
			return r, nil
		}
		previous, hadSelection := r.selected()
		r.Rows = msg.rows
		if msg.inserted {
			r.Selected = max(0, len(r.Rows)-1)
		} else if hadSelection {
			for i, row := range r.Rows {
				if row.ID == previous.ID {
					r.Selected = i
					break
				}
			}
		}
		r.Selected = max(0, min(r.Selected, len(r.Rows)-1))
		r.message = "Saved locally."
		r.form = nil
		r.confirmation = nil
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return r, tea.Quit
		}
		if r.busy {
			if key == "esc" && r.cancel != nil {
				r.cancel()
			}
			return r, nil
		}
		if key == "ctrl+s" {
			r.SaveRequested = true
			return r, nil
		}
		if r.confirmation != nil {
			if key == "esc" {
				r.confirmation = nil
				r.message = "Deletion canceled."
			}
			if key == "enter" {
				call := *r.confirmation
				call.Approved = true
				return r, r.execute(call)
			}
			return r, nil
		}
		if r.form != nil {
			return r, r.formKey(msg)
		}
		switch key {
		case "esc":
			r.ExitRequested = true
			return r, nil
		case "up":
			r.Selected = max(0, r.Selected-1)
		case "down":
			r.Selected = min(max(0, len(r.Rows)-1), r.Selected+1)
		case "pgup":
			r.Selected = max(0, r.Selected-max(1, r.Height-10))
		case "pgdown":
			r.Selected = min(max(0, len(r.Rows)-1), r.Selected+max(1, r.Height-10))
		case "tab":
			r.actionFocus = (r.actionFocus+2)%(len(r.actions())+1) - 1
		case "shift+tab":
			r.actionFocus = (r.actionFocus+len(r.actions())+1)%(len(r.actions())+1) - 1
		case "a":
			if action, ok := r.Definition.List().Action("add"); ok {
				return r, r.perform(action)
			}
		case "e":
			if action, ok := r.Definition.List().Action("edit"); ok {
				return r, r.perform(action)
			}
		case " ":
			if action, ok := r.Definition.List().Action("primary"); ok {
				return r, r.perform(action)
			}
		case "delete", "backspace":
			if action, ok := r.Definition.List().Action("delete"); ok {
				return r, r.perform(action)
			}
		case "enter":
			if r.actionFocus >= 0 && r.actionFocus < len(r.actions()) {
				return r, r.perform(r.actions()[r.actionFocus])
			}
		}
	}
	if r.form != nil && !r.busy {
		index := r.form.focus
		if index < len(r.form.inputs) {
			field, _ := r.Definition.Field(r.form.component.Inputs[index].Field)
			if field.Type == "text" || field.Type == "date" {
				var cmd tea.Cmd
				r.form.inputs[index], cmd = r.form.inputs[index].Update(msg)
				return r, cmd
			}
		}
	}
	return r, nil
}
func (r *Runtime) execute(call capability.Call) tea.Cmd {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	r.cancel, r.done, r.busy = cancel, make(chan struct{}), true
	r.request++
	request, done, executor := r.request, r.done, r.executor
	r.message = "Saving…"
	descriptor, _ := r.catalog.Lookup(call.ID)
	return func() tea.Msg {
		defer close(done)
		defer cancel()
		rows, err := executor.Execute(ctx, call)
		return mutationMsg{request, rows, err, descriptor.Operation == "insert"}
	}
}
func (r *Runtime) perform(action app.Action) tea.Cmd {
	r.message = ""
	selected, ok := r.selected()
	if action.Target == "selected" && !ok {
		r.message = "Select a record first."
		return nil
	}
	if action.Role == "add" || action.Role == "edit" {
		component, _ := r.Definition.Component(action.Form)
		form := &recordForm{component: component, inputs: make([]textinput.Model, len(component.Inputs))}
		values := r.Definition.Defaults()
		if action.Role == "edit" {
			values = selected.Values
			form.recordID = selected.ID
		}
		for i, in := range component.Inputs {
			field, _ := r.Definition.Field(in.Field)
			t := textinput.New()
			t.CharLimit = 8192
			t.Prompt = "> "
			t.SetValue(inputValue(values[in.Field]))
			if field.Type == "date" {
				t.Placeholder = "YYYY-MM-DD (optional)"
			}
			form.inputs[i] = t
		}
		r.form = form
		r.resizeForm()
		return r.focusForm()
	}
	call, err := r.bind(action)
	if err != nil {
		r.message = err.Error()
		return nil
	}
	desc, exists := r.catalog.Lookup(action.Capability)
	if !exists {
		r.message = "This capability is unavailable."
		return nil
	}
	if desc.Sensitive {
		r.confirmation = &call
		r.confirmationLabel = recordLabel(r.Definition.List(), selected)
		return nil
	}
	return r.execute(call)
}
func (r *Runtime) bind(action app.Action) (capability.Call, error) {
	call := capability.Call{ID: action.Capability, Collection: r.Definition.Collection.ID, Values: map[string]any{}}
	if action.Target == "selected" {
		selected, ok := r.selected()
		if !ok {
			return call, fmt.Errorf("select a record first")
		}
		call.RecordID = selected.ID
		if r.form != nil {
			call.RecordID = r.form.recordID
		}
	}
	for _, binding := range action.Bindings {
		var value any
		if binding.Source == "literal" {
			if err := json.Unmarshal(binding.Literal, &value); err != nil {
				return call, fmt.Errorf("invalid literal binding")
			}
		} else {
			if r.form == nil {
				return call, fmt.Errorf("form input is unavailable")
			}
			found := false
			for i, in := range r.form.component.Inputs {
				if in.ID == binding.Input {
					field, _ := r.Definition.Field(in.Field)
					text := r.form.inputs[i].Value()
					switch field.Type {
					case "boolean":
						value = text == "true"
					case "date":
						if strings.TrimSpace(text) != "" {
							value = strings.TrimSpace(text)
						}
					default:
						value = text
					}
					found = true
					break
				}
			}
			if !found {
				return call, fmt.Errorf("bound form input is unavailable")
			}
		}
		field, _ := r.Definition.Field(binding.Field)
		if err := app.ValidateValue(field, value); err != nil {
			return call, err
		}
		call.Values[binding.Field] = value
	}
	return call, nil
}
func inputValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
func (r *Runtime) resizeForm() {
	if r.form == nil {
		return
	}
	for i := range r.form.inputs {
		r.form.inputs[i].Width = max(1, r.Width-10)
		r.form.inputs[i], _ = r.form.inputs[i].Update(nil)
	}
}
func (r *Runtime) focusForm() tea.Cmd {
	for i := range r.form.inputs {
		r.form.inputs[i].Blur()
	}
	i := r.form.focus
	if i >= len(r.form.inputs) {
		return nil
	}
	field, _ := r.Definition.Field(r.form.component.Inputs[i].Field)
	if field.Type == "text" || field.Type == "date" {
		return r.form.inputs[i].Focus()
	}
	return nil
}
func (r *Runtime) formKey(msg tea.KeyMsg) tea.Cmd {
	f, key := r.form, msg.String()
	if key == "esc" {
		r.form = nil
		r.message = "Edit canceled."
		return nil
	}
	count := len(f.inputs)
	if key == "tab" || key == "shift+tab" {
		delta := 1
		if key == "shift+tab" {
			delta = -1
		}
		f.focus = (f.focus + delta + count + 2) % (count + 2)
		return r.focusForm()
	}
	if key == "enter" {
		if f.focus == count+1 {
			r.form = nil
			return nil
		}
		action, _ := f.component.Action("submit")
		return r.perform(action)
	}
	if f.focus >= count {
		return nil
	}
	field, _ := r.Definition.Field(f.component.Inputs[f.focus].Field)
	if field.Type == "boolean" {
		if key == " " || key == "left" || key == "right" {
			f.inputs[f.focus].SetValue(fmt.Sprint(f.inputs[f.focus].Value() != "true"))
		}
		return nil
	}
	if field.Type == "choice" {
		if key == "left" || key == "right" || key == " " {
			index := 0
			for j, v := range field.Choices {
				if v == f.inputs[f.focus].Value() {
					index = j
				}
			}
			delta := 1
			if key == "left" {
				delta = -1
			}
			f.inputs[f.focus].SetValue(field.Choices[(index+delta+len(field.Choices))%len(field.Choices)])
		}
		return nil
	}
	var cmd tea.Cmd
	f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	return cmd
}
func recordLabel(list app.Component, row capability.Record) string {
	if len(list.Columns) == 0 {
		return row.ID
	}
	return inputValue(row.Values[list.Columns[0]])
}
func displayed(value any) string {
	if value == nil || value == "" {
		return "—"
	}
	if b, ok := value.(bool); ok {
		if b {
			return "yes"
		}
		return "no"
	}
	return escapeControls(fmt.Sprint(value))
}
func staticText(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = escapeControls(lines[i])
	}
	return strings.Join(lines, "\n")
}

func (r *Runtime) View() string {
	width, height := r.Width, r.Height
	if width < 30 || height < 10 {
		return lipgloss.NewStyle().MaxWidth(width).MaxHeight(height).Render("Enlarge terminal\nfor app controls.\nEsc Back · Ctrl+C Quit")
	}
	panel := max(1, width-4)
	line := lipgloss.NewStyle().Width(panel).MaxWidth(panel).MaxHeight(1)
	header := studioHeader(escapeControls(r.Definition.Name)+" / "+escapeControls(r.Definition.Page.Title), panel)
	status := "Saved · local data"
	if r.Path != "" {
		status += " · " + escapeControls(r.Path)
	}
	rows := []string{header, line.Foreground(welcomeMuted).Render(status)}
	if r.confirmation != nil {
		rows = append(rows, "", "Delete this record?", line.Render(escapeControls(r.confirmationLabel)), line.Foreground(welcomeMuted).Render("ID: "+r.confirmation.RecordID), "", "Enter Confirm deletion · Esc Cancel")
	} else if r.form != nil {
		f := r.form
		rows = append(rows, line.Foreground(welcomeLilac).Bold(true).Render(escapeControls(f.component.Title)))
		var controls []string
		starts, ends := []int{}, []int{}
		for i, in := range f.component.Inputs {
			starts = append(starts, len(controls))
			field, _ := r.Definition.Field(in.Field)
			label := escapeControls(field.Label)
			if field.Required {
				label += " *"
			}
			prefix := "  "
			if i == f.focus {
				prefix = "> "
			}
			controls = append(controls, line.Foreground(welcomeMint).Render(prefix+label))
			value := f.inputs[i].View()
			if field.Type == "choice" {
				value = "  ‹ " + escapeControls(f.inputs[i].Value()) + " ›"
			} else if field.Type == "boolean" {
				value = "  [ ]"
				if f.inputs[i].Value() == "true" {
					value = "  [x]"
				}
			}
			controls = append(controls, value, "")
			ends = append(ends, len(controls))
		}
		action, _ := f.component.Action("submit")
		for i, text := range []string{action.Label, "Cancel"} {
			starts = append(starts, len(controls))
			prefix := "  "
			if f.focus == len(f.inputs)+i {
				prefix = "> "
			}
			controls = append(controls, line.Render(prefix+"[ "+escapeControls(text)+" ]"))
			ends = append(ends, len(controls))
		}
		available := max(1, height-8)
		start := max(0, ends[f.focus]-available)
		start = min(start, starts[f.focus])
		rows = append(rows, controls[start:min(len(controls), start+available)]...)
	} else {
		if height >= 15 {
			remaining := 3
			for _, c := range r.Definition.Page.Components {
				if c.Type == "text" && remaining > 0 {
					text := lipgloss.NewStyle().Width(panel).MaxHeight(min(2, remaining)).Foreground(welcomeMuted).Render(staticText(c.Text))
					rows = append(rows, text)
					remaining -= lipgloss.Height(text)
				}
			}
		}
		list := r.Definition.List()
		columns := list.Columns
		compact := width < 80
		itemHeight := 1
		if compact {
			itemHeight = 2
		}
		capacity := max(1, (height-lipgloss.Height(strings.Join(rows, "\n"))-7)/itemHeight)
		start := max(0, r.Selected-capacity+1)
		if !compact {
			var labels []string
			for _, id := range columns {
				field, _ := r.Definition.Field(id)
				labels = append(labels, field.Label)
			}
			rows = append(rows, r.tableLine(labels, panel))
		}
		if len(r.Rows) == 0 {
			rows = append(rows, "", line.Foreground(welcomeMuted).Render("No records yet. Press a to add one."))
		}
		for i := start; i < min(len(r.Rows), start+capacity); i++ {
			row := r.Rows[i]
			prefix := "  "
			if i == r.Selected {
				prefix = "> "
			}
			var cells []string
			for _, col := range columns {
				cells = append(cells, displayed(row.Values[col]))
			}
			if compact {
				rows = append(rows, line.Render(prefix+cells[0]))
				rows = append(rows, line.Foreground(welcomeMuted).Render("  "+strings.Join(cells[1:], " · ")))
			} else {
				rows = append(rows, prefix+r.tableLine(cells, panel-2))
			}
		}
		var buttons []string
		for i, a := range r.actions() {
			text := "[ " + escapeControls(a.Label) + " ]"
			if i == r.actionFocus {
				text = "> " + text
			}
			buttons = append(buttons, text)
		}
		if compact && r.actionFocus >= 0 {
			buttons = []string{buttons[r.actionFocus], "Tab next"}
		}
		rows = append(rows, line.Foreground(welcomeLilac).Render(strings.Join(buttons, " ")))
	}
	message := r.message
	if message == "" {
		message = fmt.Sprintf("%d records", len(r.Rows))
	}
	rows = append(rows, line.Foreground(welcomePeach).Render(escapeControls(message)))
	help := "↑↓ Select · a Add · e Edit · Space Action · Del Delete · Esc Back"
	if r.form != nil {
		help = "Tab Focus · ←→ Choices · Enter Save · Esc Cancel"
	}
	if r.confirmation != nil {
		help = "Enter Delete · Esc Cancel"
	}
	if height <= 8 {
		help = "a Add · e Edit · Esc Back"
	}
	if panel < 64 {
		help = "↑↓ · a Add · e Edit\nSpace Action · Del Delete · Esc Back"
		if r.form != nil {
			help = "Tab Focus · ←→ Choices\nEnter Save · Esc Cancel"
		}
		if r.confirmation != nil {
			help = "Enter Delete · Esc Cancel"
		}
	}
	if height <= 8 || panel < 26 {
		help = "a Add · Esc Back"
		if r.form != nil || r.confirmation != nil {
			help = "Enter OK · Esc"
		}
	}
	var footer []string
	for _, text := range strings.Split(help, "\n") {
		footer = append(footer, line.Foreground(welcomeMuted).Render(text))
	}
	body := lipgloss.NewStyle().MaxWidth(width).MaxHeight(max(1, height-len(footer)-1)).Render(strings.Join(rows, "\n"))
	return lipgloss.NewStyle().Padding(0, 2).MaxWidth(width).Render(body + "\n" + strings.Join(footer, "\n"))
}
func (r *Runtime) tableLine(cells []string, width int) string {
	if len(cells) == 0 {
		return ""
	}
	unit := max(1, (width-len(cells))/(len(cells)+1))
	var parts []string
	for i, cell := range cells {
		size := unit
		if i == 0 {
			size = unit * 2
		}
		parts = append(parts, lipgloss.NewStyle().Width(size).MaxWidth(size).MaxHeight(1).Render(escapeControls(cell)))
	}
	return strings.Join(parts, " ")
}

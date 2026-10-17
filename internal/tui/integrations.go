package tui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/integrations"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
	"golang.org/x/oauth2"
	"os"
)

type apiModel struct {
	store                                      integrations.Store
	doc                                        integrations.Document
	cursor, action, picker, field, requirement int
	inputs                                     []textinput.Model
	editing                                    integrations.Connection
	authIndex                                  int
	message, operation, url, code              string
	request                                    uint64
	cancelled                                  bool
	binding                                    bool
}
type apiMsg struct {
	request   uint64
	operation string
	doc       integrations.Document
	message   string
	err       error
}
type apiURLMsg struct {
	request   uint64
	url, code string
}

func newAPI() *apiModel {
	s, err := integrations.DefaultStore()
	a := &apiModel{store: s}
	if err != nil {
		a.message = err.Error()
	}
	return a
}
func (a *apiModel) auths() []string {
	if a.editing.Kind == "github" {
		return []string{"oauth", "bearer"}
	}
	return []string{"none", "bearer", "header", "query"}
}
func (a *apiModel) fields() []int {
	fields := []int{0}
	if a.editing.Kind == "custom" {
		fields = append(fields, 1)
	}
	fields = append(fields, 2)
	auth := a.auths()[a.authIndex]
	if auth == "header" || auth == "query" {
		fields = append(fields, 3)
	}
	if auth != "none" && auth != "oauth" {
		fields = append(fields, 4)
	}
	return append(fields, 5)
}
func authLabel(auth string) string {
	switch auth {
	case "oauth":
		return "Browser sign-in"
	case "bearer":
		return "Token (hidden entry)"
	case "none":
		return "No authentication"
	case "header":
		return "API key in header"
	case "query":
		return "API key in URL parameter"
	}
	return auth
}
func (a *apiModel) badge(kind string) string {
	if a.doc.NerdIcons {
		if kind == "github" {
			return "\uf09b"
		}
		return "\uf0c1"
	}
	if kind == "github" {
		return "[GH]"
	}
	return "[+]"
}
func (a *apiModel) selected() (integrations.Connection, bool) {
	if a.cursor < 0 || a.cursor >= len(a.doc.Connections) {
		return integrations.Connection{}, false
	}
	return a.doc.Connections[a.cursor], true
}
func (a *apiModel) form(c integrations.Connection, width int) {
	a.editing = c
	a.authIndex = 0
	a.field = 0
	a.inputs = nil
	values := []string{c.Name, c.BaseURL, c.Key, ""}
	for i, v := range values {
		input := textinput.New()
		input.SetValue(v)
		input.CharLimit = 8192
		input.Width = max(1, studioWidth(width)-6)
		if i == 3 {
			input.EchoMode = textinput.EchoPassword
			input.Placeholder = "Hidden · leave empty to keep current credential"
		}
		a.inputs = append(a.inputs, input)
	}
	for i, auth := range a.auths() {
		if auth == c.Auth {
			a.authIndex = i
		}
	}
	a.inputs[0].Focus()
}
func (a *apiModel) focus() {
	for i := range a.inputs {
		a.inputs[i].Blur()
	}
	switch a.field {
	case 0:
		a.inputs[0].Focus()
	case 1:
		a.inputs[1].Focus()
	case 3:
		a.inputs[2].Focus()
	case 4:
		a.inputs[3].Focus()
	}
}
func (m Model) beginAPI(operation string) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	a := m.api
	if a == nil {
		a = newAPI()
		m.api = a
	}
	if a.store.Path == "" {
		a.message = "Cannot locate the user configuration directory."
		m.screen = "api"
		return m, nil
	}
	a.request++
	request := a.request
	a.operation = operation
	a.cancelled = false
	a.url = ""
	a.code = ""
	a.message = ""
	timeout := 10 * time.Second
	if operation == "signin" {
		timeout = 5 * time.Minute
	}
	if operation == "launch" {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	done := make(chan struct{})
	m.cancel, m.done, m.loading = cancel, done, true
	m.screen = "apibusy"
	selected, _ := a.selected()
	candidate := a.editing
	secret := ""
	if len(a.inputs) > 3 {
		secret = a.inputs[3].Value()
	}
	document := a.doc
	store := a.store
	project := m.codeProject
	clientID := os.Getenv("TUILOR_GITHUB_CLIENT_ID")
	events := make(chan apiURLMsg, 1)
	command := func() tea.Msg {
		defer close(done)
		defer close(events)
		defer cancel()
		result := apiMsg{request: request, operation: operation}
		credentials := integrations.Credentials{}
		switch operation {
		case "load":
			result.doc, result.err = store.Load(ctx)
		case "icons":
			document.NerdIcons = !document.NerdIcons
			result.err = store.Save(ctx, document)
		case "save":
			result.err = candidate.Validate()
			if result.err != nil {
				break
			}
			if secret != "" && candidate.Auth != "none" && candidate.Auth != "oauth" {
				if candidate.Kind == "github" {
					_, result.err = integrations.Identity(ctx, secret)
				}
				if result.err == nil {
					result.err = credentials.Save(ctx, candidate, integrations.Login{Token: oauth2.Token{AccessToken: secret, TokenType: "Bearer"}})
				}
				if result.err != nil {
					break
				}
			}
			old := integrations.Connection{}
			found := false
			for i, c := range document.Connections {
				if c.ID == candidate.ID {
					old = c
					document.Connections[i] = candidate
					found = true
				}
			}
			if !found {
				document.Connections = append(document.Connections, candidate)
			}
			result.err = store.Save(ctx, document)
			if result.err == nil && old.ID != "" && old.Auth != "none" && old.Binding() != candidate.Binding() {
				if err := credentials.Delete(ctx, old); err != nil {
					result.message = "Settings saved; old credentials need cleanup: " + err.Error()
				}
			}
		case "remove":
			if selected.Auth != "none" {
				if result.err = credentials.Delete(ctx, selected); result.err != nil {
					break
				}
			}
			var keep []integrations.Connection
			for _, c := range document.Connections {
				if c.ID != selected.ID {
					keep = append(keep, c)
				}
			}
			document.Connections = keep
			result.err = store.Save(ctx, document)
		case "signout":
			if selected.Auth != "none" {
				result.err = credentials.Delete(ctx, selected)
			}
			result.message = "Local credentials removed. Provider-side access may remain authorized."
		case "check":
			var login integrations.Login
			login, result.err = integrations.ReadyLogin(ctx, selected, credentials)
			if result.err == nil && selected.Kind == "github" {
				var name string
				name, result.err = integrations.Identity(ctx, login.Token.AccessToken)
				result.message = "Last check passed · GitHub account: " + name
			} else if result.err == nil {
				result.message = "Configured. Custom API access is checked when an app makes a request."
			}
		case "signin":
			result.err = integrations.SignIn(ctx, selected, clientID, credentials, func(url, code string) {
				select {
				case events <- apiURLMsg{request, url, code}:
				case <-ctx.Done():
					return
				}
				browserCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				defer stop()
				_ = mcpclient.OpenBrowser(browserCtx, url)
			})
		case "launch":
			result.doc, result.err = store.Load(ctx)
			if result.err != nil {
				break
			}
			_, result.err = integrations.Runtime(ctx, project.Spec.Requirements, project.Bindings)
			if result.err == nil {
				release, err := project.Acquire()
				result.err = err
				if err == nil {
					result.err = project.SaveBindings(ctx)
					release()
				}
			}
		}
		if result.err == nil && operation != "load" {
			result.doc, result.err = store.Load(ctx)
		}
		return result
	}
	if operation == "signin" {
		return m, tea.Batch(command, func() tea.Msg {
			event, ok := <-events
			if !ok {
				return nil
			}
			return event
		})
	}
	return m, command
}
func (m Model) apiResult(msg apiMsg) (tea.Model, tea.Cmd) {
	a := m.api
	if a == nil || msg.request != a.request {
		return m, nil
	}
	m.loading = false
	m.cancel = nil
	m.done = nil
	if m.quitting {
		return m, tea.Quit
	}
	if a.cancelled {
		a.message = "Operation canceled."
		if a.operation == "save" {
			m.screen = "apiform"
		} else {
			m.screen = "api"
		}
		return m, nil
	}
	if msg.doc.Version == 1 {
		a.doc = msg.doc
	}
	if msg.err != nil {
		a.message = msg.err.Error()
		switch msg.operation {
		case "save":
			m.screen = "apiform"
		case "launch":
			m.screen = "apibind"
			a.requirement = 0
		default:
			m.screen = "apidetail"
			if len(a.doc.Connections) == 0 {
				m.screen = "api"
			}
		}
		return m, nil
	}
	a.doc = msg.doc
	a.message = msg.message
	a.cursor = max(0, min(a.cursor, len(a.doc.Connections)-1))
	switch msg.operation {
	case "launch":
		m.launchCode = true
		return m, tea.Quit
	case "save":
		for i, c := range a.doc.Connections {
			if c.ID == a.editing.ID {
				a.cursor = i
			}
		}
		if len(a.inputs) > 3 {
			a.inputs[3].SetValue("")
		}
		m.screen = "apidetail"
		if a.message == "" {
			a.message = "Connection saved. Sign in or check access below."
		}
	case "load":
		a.action = 0
		if a.binding {
			m.screen = "apibind"
		} else {
			m.screen = "api"
		}
	case "remove", "icons":
		m.screen = "api"
	default:
		m.screen = "apidetail"
	}
	return m, nil
}
func (m Model) prepareCodeLaunch() (tea.Model, tea.Cmd) {
	if len(m.codeProject.Spec.Requirements) == 0 {
		m.launchCode = true
		return m, tea.Quit
	}
	m.api = newAPI()
	m.api.binding = true
	if m.codeProject.Bindings == nil {
		m.codeProject.Bindings = map[string]string{}
	}
	bound := true
	for _, r := range m.codeProject.Spec.Requirements {
		if m.codeProject.Bindings[r.ID] == "" {
			bound = false
		}
	}
	if bound {
		return m.beginAPI("launch")
	}
	return m.beginAPI("load")
}
func (m Model) bindingChoices() ([]string, []integrations.Connection) {
	a := m.api
	if m.codeProject == nil || a.requirement >= len(m.codeProject.Spec.Requirements) {
		return nil, nil
	}
	r := m.codeProject.Spec.Requirements[a.requirement]
	var labels []string
	var matches []integrations.Connection
	for _, c := range a.doc.Connections {
		if c.Matches(r) {
			suffix := ""
			if m.codeProject.Bindings[r.ID] == c.ID {
				suffix = " · selected"
			}
			labels = append(labels, a.badge(c.Kind)+" "+escapeControls(c.Name)+suffix)
			matches = append(matches, c)
		}
	}
	labels = append(labels, "+ Add connection", "Manage connections")
	return labels, matches
}
func (m Model) apiKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := m.api
	key := msg.String()
	if m.screen == "apibusy" {
		if key == "esc" {
			a.cancelled = true
			m.cancel()
			a.message = "Canceling…"
		}
		return m, nil
	}
	if key == "esc" {
		switch m.screen {
		case "apiform":
			if a.binding {
				m.screen = "apibind"
				a.action = 0
			} else {
				m.screen = "api"
			}
		case "apipicker":
			m.screen = "api"
		case "apidetail", "apiremove":
			if a.binding {
				m.screen = "apibind"
				a.action = 0
			} else {
				m.screen = "api"
			}
		case "api":
			if a.binding {
				m.screen = "apibind"
				a.action = 0
			} else {
				m.screen = "settings"
			}
		case "apibind":
			m.screen = "codereview"
		}
		return m, nil
	}
	switch m.screen {
	case "api":
		switch key {
		case "up":
			a.cursor = max(0, a.cursor-1)
		case "down":
			a.cursor = min(len(a.doc.Connections), a.cursor+1)
		case "n":
			a.picker = 0
			m.screen = "apipicker"
		case "i":
			return m.beginAPI("icons")
		case "r":
			return m.beginAPI("load")
		case "enter":
			if a.cursor >= len(a.doc.Connections) {
				m.screen = "apipicker"
				a.picker = 0
			} else {
				m.screen = "apidetail"
				a.action = 0
			}
		}
	case "apipicker":
		switch key {
		case "up", "down", "tab", "shift+tab":
			a.picker = 1 - a.picker
		case "enter":
			c := integrations.Connection{ID: integrations.ID(), Kind: "custom", Auth: "none"}
			if a.picker == 0 {
				c.Kind = "github"
				c.Name = "GitHub"
				c.BaseURL = "https://api.github.com"
				c.Auth = "oauth"
				if os.Getenv("TUILOR_GITHUB_CLIENT_ID") == "" {
					c.Auth = "bearer"
				}
			}
			a.form(c, m.width)
			m.screen = "apiform"
			return m, textinput.Blink
		}
	case "apibind":
		labels, matches := m.bindingChoices()
		if len(labels) == 0 {
			m.screen = "codereview"
			return m, nil
		}
		switch key {
		case "up":
			a.action = max(0, a.action-1)
		case "down":
			a.action = min(len(labels)-1, a.action+1)
		case "enter":
			if a.action < len(matches) {
				r := m.codeProject.Spec.Requirements[a.requirement]
				m.codeProject.Bindings[r.ID] = matches[a.action].ID
				a.requirement++
				a.action = 0
				if a.requirement == len(m.codeProject.Spec.Requirements) {
					return m.beginAPI("launch")
				}
				return m, nil
			}
			if a.action == len(matches) && a.requirement < len(m.codeProject.Spec.Requirements) {
				r := m.codeProject.Spec.Requirements[a.requirement]
				c := integrations.Connection{ID: integrations.ID(), Name: r.Name, Kind: r.Kind, BaseURL: r.BaseURL, Auth: r.Auth, Key: r.Key}
				if c.Kind == "github" && os.Getenv("TUILOR_GITHUB_CLIENT_ID") == "" {
					c.Auth = "bearer"
				}
				a.form(c, m.width)
				m.screen = "apiform"
				return m, textinput.Blink
			}
			if a.action == len(matches)+1 {
				m.screen = "api"
				a.cursor = 0
			}
		}
	case "apidetail":
		actions := []string{"Check access", "Edit", "Reconnect / Sign in", "Sign out", "Remove"}
		switch key {
		case "up":
			a.action = max(0, a.action-1)
		case "down":
			a.action = min(len(actions)-1, a.action+1)
		case "enter":
			c, ok := a.selected()
			if !ok {
				m.screen = "api"
				return m, nil
			}
			switch a.action {
			case 0:
				return m.beginAPI("check")
			case 1:
				a.form(c, m.width)
				m.screen = "apiform"
				return m, textinput.Blink
			case 2:
				if c.Auth == "oauth" {
					return m.beginAPI("signin")
				}
				a.form(c, m.width)
				a.field = 4
				a.focus()
				m.screen = "apiform"
				return m, textinput.Blink
			case 3:
				return m.beginAPI("signout")
			case 4:
				m.screen = "apiremove"
			}
		}
	case "apiremove":
		if key == "enter" {
			return m.beginAPI("remove")
		}
	case "apiform":
		if key == "tab" || key == "shift+tab" {
			delta := 1
			if key == "shift+tab" {
				delta = -1
			}
			fields := a.fields()
			index := 0
			for i, f := range fields {
				if f == a.field {
					index = i
				}
			}
			a.field = fields[(index+delta+len(fields))%len(fields)]
			a.focus()
			return m, textinput.Blink
		}
		if a.field == 2 && (key == "left" || key == "right" || key == " ") {
			delta := 1
			if key == "left" {
				delta = -1
			}
			a.authIndex = (a.authIndex + delta + len(a.auths())) % len(a.auths())
			return m, nil
		}
		if key == "ctrl+s" || (a.field == 5 && key == "enter") {
			a.editing.Name = a.inputs[0].Value()
			a.editing.BaseURL = strings.TrimRight(a.inputs[1].Value(), "/")
			a.editing.Auth = a.auths()[a.authIndex]
			a.editing.Key = a.inputs[2].Value()
			if a.editing.Auth != "header" && a.editing.Auth != "query" {
				a.editing.Key = ""
			}
			return m.beginAPI("save")
		}
	}
	if m.screen == "apiform" {
		return m, m.updateAPIInputs(msg)
	}
	return m, nil
}
func (m *Model) updateAPIInputs(msg tea.Msg) tea.Cmd {
	a := m.api
	var commands []tea.Cmd
	for i := range a.inputs {
		var cmd tea.Cmd
		a.inputs[i], cmd = a.inputs[i].Update(msg)
		commands = append(commands, cmd)
	}
	return tea.Batch(commands...)
}
func (m Model) apiView() string {
	a := m.api
	width := studioWidth(m.width)
	if m.width < 28 || m.height < 10 {
		return m.welcomeCanvas("Connections\nEnlarge terminal · Esc Back")
	}
	rows := []string{studioHeader("Connections", width), ""}
	help := "↑↓ Choose · Enter Open · n Add · i Icons · Esc Back"
	if a.message != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(welcomePeach).Width(width).MaxHeight(3).Render(escapeControls(a.message)), "")
	}
	switch m.screen {
	case "api":
		labels := []string{}
		for _, c := range a.doc.Connections {
			labels = append(labels, a.badge(c.Kind)+" "+escapeControls(c.Name)+" · "+authLabel(c.Auth))
		}
		labels = append(labels, "+ Add integration")
		rows = append(rows, menu(labels, a.cursor, m.height-5))
		mode := "Text badges"
		if a.doc.NerdIcons {
			mode = "Nerd Font icons (requires a compatible terminal font)"
		}
		rows = append(rows, mode)
	case "apipicker":
		rows = append(rows, "Add integration", "", menu([]string{a.badge("github") + " GitHub · Browser sign-in or token", a.badge("custom") + " Custom API · Your URL and credentials"}, a.picker, m.height-5))
		help = "↑↓ Choose · Enter Continue · Esc Back"
	case "apibusy":
		rows = append(rows, "Working: "+a.operation+"…")
		if a.url != "" {
			rows = append(rows, "", "Open: "+escapeControls(a.url), "Enter code: "+escapeControls(a.code))
		}
		help = "Esc Cancel · Ctrl+C Quit"
	case "apidetail":
		if c, ok := a.selected(); ok {
			rows = append(rows, a.badge(c.Kind)+" "+escapeControls(c.Name), escapeControls(c.BaseURL), "Authentication: "+authLabel(c.Auth), "", menu([]string{"Check access", "Edit", "Reconnect / Sign in", "Sign out", "Remove"}, a.action, m.height-8))
			if c.Auth == "oauth" && os.Getenv("TUILOR_GITHUB_CLIENT_ID") == "" {
				rows = append(rows, "Browser sign-in needs TUILOR_GITHUB_CLIENT_ID. Token entry is available in Edit.")
			}
		}
	case "apiremove":
		rows = append(rows, "Remove this connection and its local credentials?", "Existing apps may need another connection.")
		help = "Enter Remove · Esc Cancel"
	case "apibind":
		if a.requirement < len(m.codeProject.Spec.Requirements) {
			r := m.codeProject.Spec.Requirements[a.requirement]
			labels, _ := m.bindingChoices()
			rows = append(rows, "Allow this app to use a connection for:", escapeControls(r.Name), escapeControls(r.BaseURL), "", menu(labels, a.action, m.height-9))
			help = "↑↓ Choose · Enter Approve connection · Esc Keep candidate"
		}
	case "apiform":
		labels := []string{"Name", "API base URL", "Authentication (←/→)", "Credential header/query name", "Credential (hidden)", "Save connection"}
		var controls []string
		visible := a.fields()
		focused := 0
		for position, i := range visible {
			label := labels[i]
			if a.field == i {
				focused = position
			}
			marker := "  "
			if a.field == i {
				marker = lipgloss.NewStyle().Foreground(welcomeMint).Render("> ")
			}
			value := ""
			switch i {
			case 0:
				value = a.inputs[0].View()
			case 1:
				value = a.inputs[1].View()
			case 2:
				value = authLabel(a.auths()[a.authIndex])
			case 3:
				value = a.inputs[2].View()
			case 4:
				value = a.inputs[3].View()
			}
			controls = append(controls, marker+label+"\n  "+value)
		}
		// Scroll the form by controls to keep focus visible on short terminals.
		capacity := max(1, (m.height-lipgloss.Height(strings.Join(rows, "\n"))-4)/3)
		start := max(0, focused-capacity+1)
		rows = append(rows, strings.Join(controls[start:min(len(controls), start+capacity)], "\n\n"))
		help = "Tab Focus · ←→ Auth · Ctrl+S Save · Esc Back"
	}
	rows = append(rows, "", help, "Ctrl+C Quit")
	return m.welcomeCanvas(lipgloss.NewStyle().Width(width).MaxWidth(width).MaxHeight(max(1, m.height-2)).Render(strings.Join(rows, "\n")))
}

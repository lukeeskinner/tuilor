package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lukeeskinner/tuilor/internal/connections"
	"github.com/lukeeskinner/tuilor/internal/mcpclient"
)

type connectionModel struct {
	store                                     connections.Store
	credentials                               connections.Credentials
	servers                                   []connections.Server
	statuses                                  map[string]string
	tools                                     map[string][]mcpclient.Tool
	checked                                   map[string]bool
	cursor, action                            int
	form                                      *connectionForm
	viewport                                  viewport.Model
	viewportText                              string
	viewportWidth                             int
	message, operation, returnScreen, authURL string
	failed, cancelled                         bool
	request                                   uint64
}
type connectionMsg struct {
	request   uint64
	operation string
	servers   []connections.Server
	server    connections.Server
	tools     []mcpclient.Tool
	err       error
}
type connectionURLMsg struct {
	request uint64
	url     string
	events  <-chan string
}

func newConnections() *connectionModel {
	store, err := connections.DefaultStore()
	c := &connectionModel{store: store, statuses: map[string]string{}, tools: map[string][]mcpclient.Tool{}, checked: map[string]bool{}, viewport: viewport.New(1, 1)}
	if err != nil {
		c.message = err.Error()
		c.failed = true
	}
	return c
}
func (c *connectionModel) selected() (connections.Server, bool) {
	if c.cursor < 0 || c.cursor >= len(c.servers) {
		return connections.Server{}, false
	}
	return c.servers[c.cursor], true
}
func (c *connectionModel) actions() []string {
	s, ok := c.selected()
	if !ok {
		return nil
	}
	a := []string{"Check Connection", "View Tools", "Edit", "Remove"}
	if s.Kind == "http" && s.Auth == "oauth" {
		a = append(a, "Sign In", "Sign Out")
	}
	return a
}

// Background commands own their context. The model retains only cancellation and
// completion signals, not a context that could outlive the operation.
func (m Model) beginConnection(operation string, server connections.Server) (tea.Model, tea.Cmd) {
	c := m.mcp
	if m.loading {
		return m, nil
	}
	if c.store.Path == "" {
		c.failed = true
		c.message = "Cannot locate the user configuration directory."
		return m, nil
	}
	c.request++
	request := c.request
	c.operation, c.cancelled, c.authURL = operation, false, ""
	c.returnScreen = m.screen
	c.message, c.failed = "", false
	duration := 10 * time.Second
	if operation == "signin" {
		duration = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	done := make(chan struct{})
	events := make(chan string, 1)
	m.cancel, m.done, m.loading = cancel, done, true
	m.screen = "mcpbusy"
	store, credentials := c.store, c.credentials
	cmd := func() tea.Msg {
		defer close(done)
		defer close(events)
		defer cancel()
		result := connectionMsg{request: request, operation: operation, server: server}
		switch operation {
		case "load":
			result.servers, result.err = store.Load(ctx)
		case "save":
			// Delete the old login before changing its binding. On failure, settings and
			// the editable form remain intact; no new endpoint can inherit the old token.
			old, err := store.Load(ctx)
			result.err = err
			if err == nil {
				for _, s := range old {
					if s.ID == server.ID && s.Auth == "oauth" && s.Binding() != server.Binding() {
						result.err = credentials.Delete(ctx, s)
						break
					}
				}
				if result.err == nil {
					result.err = store.Put(ctx, server)
				}
			}
		case "remove":
			if server.Auth == "oauth" {
				result.err = credentials.Delete(ctx, server)
			}
			if result.err == nil {
				result.err = store.Remove(ctx, server.ID)
			}
		case "check":
			result.tools, result.err = mcpclient.Discover(ctx, server, credentials)
		case "signin":
			result.err = mcpclient.SignIn(ctx, server, credentials, func(raw string) {
				select {
				case events <- raw:
				case <-ctx.Done():
					return
				}
				browserCtx, stop := context.WithTimeout(ctx, 5*time.Second)
				defer stop()
				_ = mcpclient.OpenBrowser(browserCtx, raw)
			})
		case "signout":
			result.err = credentials.Delete(ctx, server)
		}
		if result.err == nil && (operation == "save" || operation == "remove") {
			result.servers, result.err = store.Load(ctx)
		}
		return result
	}
	if operation == "signin" {
		return m, tea.Batch(cmd, waitConnectionURL(request, events))
	}
	return m, cmd
}
func waitConnectionURL(request uint64, events <-chan string) tea.Cmd {
	return func() tea.Msg {
		raw, ok := <-events
		if !ok {
			return nil
		}
		return connectionURLMsg{request, raw, events}
	}
}
func (m Model) connectionResult(msg connectionMsg) (tea.Model, tea.Cmd) {
	c := m.mcp
	if c == nil || msg.request != c.request {
		return m, nil
	}
	m.loading, m.cancel, m.done = false, nil, nil
	if m.quitting {
		return m, tea.Quit
	}
	// A canceled operation cannot reopen a screen. Successful config commits may
	// already be durable, so reload the list on the next visit.
	if c.cancelled {
		c.message = "Operation canceled."
		return m, nil
	}
	m.screen = c.returnScreen
	if m.screen == "mcpbusy" || m.screen == "home" {
		m.screen = "mcp"
	}
	c.authURL = ""
	if msg.err != nil {
		c.failed = true
		c.message = msg.err.Error()
		if len(c.message) > 1400 {
			c.message = c.message[:1400] + "…"
		}
		if msg.operation == "check" {
			delete(c.tools, msg.server.ID)
			delete(c.checked, msg.server.ID)
			status := "Check failed"
			if errors.Is(msg.err, connections.ErrNoLogin) {
				status = "Sign-in required"
			}
			c.statuses[msg.server.ID] = status
		}
		return m, nil
	}
	c.failed = false
	switch msg.operation {
	case "load", "save", "remove":
		c.servers = msg.servers
		sort.SliceStable(c.servers, func(i, j int) bool { return strings.ToLower(c.servers[i].Name) < strings.ToLower(c.servers[j].Name) })
		c.cursor = max(0, min(c.cursor, len(c.servers)-1))
		m.screen = "mcp"
		if msg.operation == "save" {
			for i, s := range c.servers {
				if s.ID == msg.server.ID {
					c.cursor = i
				}
			}
			delete(c.statuses, msg.server.ID)
			delete(c.tools, msg.server.ID)
			delete(c.checked, msg.server.ID)
			c.form = nil
			c.message = "Connection saved."
		}
		if msg.operation == "remove" {
			delete(c.statuses, msg.server.ID)
			delete(c.tools, msg.server.ID)
			delete(c.checked, msg.server.ID)
			c.message = "Connection removed."
		}
	case "check":
		c.tools[msg.server.ID], c.checked[msg.server.ID] = msg.tools, true
		c.statuses[msg.server.ID] = "Last check passed"
		c.message = fmt.Sprintf("Last check passed · %d tools · connection closed", len(msg.tools))
	case "signin":
		c.statuses[msg.server.ID] = "Not checked"
		c.message = "Signed in. Choose Check Connection to discover tools."
	case "signout":
		delete(c.tools, msg.server.ID)
		delete(c.checked, msg.server.ID)
		c.statuses[msg.server.ID] = "Sign-in required"
		c.message = "Local login removed. Provider-side authorization may remain; revoke it with the provider if needed."
	}
	return m, nil
}

func (m Model) connectionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.mcp
	key := msg.String()
	if key == "esc" {
		if m.loading {
			m.cancel()
			c.cancelled = true
			c.message = "Canceling…"
			m.screen = c.returnScreen
			return m, nil
		}
		c.message = ""
		c.failed = false
		switch m.screen {
		case "mcp":
			m.screen = "settings"
		case "mcpform", "mcpdetail":
			m.screen = "mcp"
		default:
			m.screen = "mcpdetail"
		}
		return m, nil
	}
	if m.loading {
		if m.screen == "mcpbusy" && c.authURL != "" {
			var cmd tea.Cmd
			c.viewport, cmd = c.viewport.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	if m.screen == "mcpform" {
		return m.connectionFormKey(msg)
	}
	if m.screen == "mcptools" {
		var cmd tea.Cmd
		c.viewport, cmd = c.viewport.Update(msg)
		return m, cmd
	}
	if m.screen == "mcpremove" {
		if key == "enter" {
			s, _ := c.selected()
			return m.beginConnection("remove", s)
		}
		return m, nil
	}
	if m.screen == "mcptrust" {
		if key == "enter" {
			s, _ := c.selected()
			m.screen = "mcpdetail"
			return m.beginConnection("check", s)
		}
		return m, nil
	}
	if m.screen == "mcp" {
		switch key {
		case "up", "shift+tab":
			c.cursor = max(0, c.cursor-1)
		case "down", "tab":
			c.cursor = min(max(0, len(c.servers)-1), c.cursor+1)
		case "n":
			c.message, c.failed = "", false
			c.form = newConnectionForm(connections.Server{ID: connections.ID(), Kind: "http", Auth: "oauth"})
			m.screen = "mcpform"
			return m, c.form.focus(m.width)
		case "r":
			return m.beginConnection("load", connections.Server{})
		case "enter":
			if _, ok := c.selected(); ok {
				c.action = 0
				c.message = ""
				m.screen = "mcpdetail"
			}
		}
		return m, nil
	}
	actions := c.actions()
	switch key {
	case "up", "shift+tab":
		c.action = (c.action + len(actions) - 1) % len(actions)
	case "down", "tab":
		c.action = (c.action + 1) % len(actions)
	case "enter":
		s, ok := c.selected()
		if !ok {
			return m, nil
		}
		switch actions[c.action] {
		case "Check Connection":
			if s.Kind == "stdio" {
				m.screen = "mcptrust"
				return m, nil
			}
			return m.beginConnection("check", s)
		case "View Tools":
			if !c.checked[s.ID] {
				c.failed = true
				c.message = "Check this connection first."
				return m, nil
			}
			c.viewportText = toolText(c.tools[s.ID])
			c.viewportWidth = 0
			c.viewport.GotoTop()
			m.screen = "mcptools"
		case "Edit":
			c.message, c.failed = "", false
			c.form = newConnectionForm(s)
			m.screen = "mcpform"
			return m, c.form.focus(m.width)
		case "Remove":
			m.screen = "mcpremove"
		case "Sign In":
			return m.beginConnection("signin", s)
		case "Sign Out":
			return m.beginConnection("signout", s)
		}
	}
	return m, nil
}

func toolText(tools []mcpclient.Tool) string {
	if len(tools) == 0 {
		return "This server exposes no tools."
	}
	var b strings.Builder
	for _, t := range tools {
		fmt.Fprintf(&b, "%s\n%s\n\nInput schema:\n%s\n", escapeControls(t.Name), escapeControls(t.Description), prettySchema(t.InputSchema))
		if len(t.OutputSchema) > 0 {
			fmt.Fprintf(&b, "Output schema:\n%s\n", prettySchema(t.OutputSchema))
		}
		b.WriteString("\n────────────────────\n\n")
	}
	return b.String()
}
func prettySchema(raw []byte) string {
	var b bytes.Buffer
	if json.Indent(&b, raw, "", "  ") != nil {
		return escapeControls(string(raw))
	}
	// JSON string values encode control characters; indentation is host-written.
	return b.String()
}

func (m Model) connectionsView() string {
	c := m.mcp
	width := studioWidth(m.width)
	if m.width < 28 || m.height < 10 {
		return m.welcomeCanvas("MCP Servers\nEnlarge terminal\nEsc Back · Ctrl+C Quit")
	}
	var rows []string
	rows = append(rows, studioHeader("MCP Servers", width))
	if c.message != "" {
		color := welcomeMint
		if c.failed {
			color = welcomePeach
		}
		rows = append(rows, lipgloss.NewStyle().Foreground(color).Width(width).MaxHeight(2).Render(escapeControls(c.message)))
	}
	help := "Esc Back · q Quit"
	switch m.screen {
	case "mcp":
		if len(c.servers) == 0 {
			rows = append(rows, "", "No saved connections yet.")
		}
		labels := make([]string, len(c.servers))
		for i, s := range c.servers {
			status := c.statuses[s.ID]
			if status == "" {
				status = "Not checked"
			}
			labels[i] = lipgloss.NewStyle().MaxWidth(width - 2).MaxHeight(1).Render(
				fmt.Sprintf("[%s] %s · %s · %s", s.ID[:6], escapeControls(s.Name), connectionChoice(fieldKind, s.Kind), status))
		}
		rows = append(rows, menu(labels, c.cursor, m.height-3))
		help = "n Add · r Reload · ↑↓ Select · Enter Open · Esc Back · q Quit"
	case "mcpform":
		return m.connectionFormView(width)
	case "mcpbusy":
		text := map[string]string{"load": "Reading connections…", "save": "Saving connection…", "remove": "Removing connection…", "check": "Checking connection…", "signin": "Waiting for browser sign-in…", "signout": "Removing local login…"}[c.operation]
		rows = append(rows, "", lipgloss.NewStyle().Foreground(welcomeLilac).Render(text))
		if c.operation == "check" {
			rows = append(rows, "Discovery only · no tool calls")
		}
		if c.authURL != "" {
			c.resizeViewport(width, max(1, m.height-lipgloss.Height(strings.Join(rows, "\n"))-5))
			rows = append(rows, "", c.viewport.View())
		}
		help = "Esc Cancel · Ctrl+C Quit"
		if c.authURL != "" {
			help = "PgUp/PgDn Scroll URL · Esc Cancel · Ctrl+C Quit"
		}
	case "mcptrust":
		s, _ := c.selected()
		rows = append(rows, "", "Start this trusted local server?", escapeControls(s.Command), "", "Runs with your OS permissions. This is not a sandbox.", "Only initialization and tool discovery will be requested.", "", "> Enter Start and check")
	case "mcpremove":
		s, _ := c.selected()
		rows = append(rows, "", "Remove "+escapeControls(s.Name)+"?", "This removes its settings and local OAuth login.", "", "> Enter Remove · Esc Cancel")
	case "mcptools":
		s, _ := c.selected()
		rows = append(rows, escapeControls(s.Name)+" · discovered tools")
		c.resizeViewport(width, max(1, m.height-lipgloss.Height(strings.Join(rows, "\n"))-4))
		rows = append(rows, c.viewport.View())
		help = "↑↓ / PgUp PgDn Scroll · Esc Back · q Quit"
	default:
		s, ok := c.selected()
		if !ok {
			return m.welcomeCanvas("No selected connection. Esc Back")
		}
		detail := s.URL
		if s.Kind == "stdio" {
			detail = s.Command + "\nArguments: " + escapeControls(strings.Join(s.Args, " | ")) + "\nDirectory: " + escapeControls(s.Directory) + "\nForward env names: " + escapeControls(strings.Join(s.Env, ", "))
		} else {
			detail = escapeControls(detail) + "\nSign-in: " + connectionChoice(fieldAuth, s.Auth)
		}
		status := c.statuses[s.ID]
		if status == "" {
			status = "Not checked"
		}
		detail = lipgloss.NewStyle().Width(width).MaxHeight(max(1, min(5, m.height/4))).Render(detail)
		rows = append(rows, "", escapeControls(s.Name)+" · "+status, detail, "", menu(c.actions(), c.action, m.height-lipgloss.Height(detail)-5))
		help = "Tab / ↑↓ Choose · Enter Select · Esc Back · q Quit"
	}
	body := lipgloss.NewStyle().Width(width).MaxWidth(width).MaxHeight(max(1, m.height-3)).Render(strings.Join(rows, "\n"))
	footer := lipgloss.NewStyle().Foreground(welcomeMuted).Width(width).MaxHeight(2).Render(help)
	return m.welcomeCanvas(body + "\n\n" + footer)
}

func (c *connectionModel) resizeViewport(width, height int) {
	c.viewport.Width, c.viewport.Height = width, height
	if c.viewportWidth != width {
		c.viewport.SetContent(lipgloss.NewStyle().Width(width).Render(c.viewportText))
		c.viewportWidth = width
	}
}

type connectionForm struct {
	original   connections.Server
	fields     []textinput.Model
	args, env  textarea.Model
	focusIndex int
	advanced   bool
}

const (
	fieldName = iota
	fieldKind
	fieldCommand
	fieldArgs
	fieldDir
	fieldEnv
	fieldURL
	fieldAuth
	fieldToken
	fieldClient
	fieldPort
	fieldSave
	fieldAdvanced
)

var fieldLabels = []string{"Name", "Connection (←/→)", "Program to run", "Arguments (one per line)", "Working folder (optional)", "Environment variable names (one per line)", "MCP server URL", "Sign-in (←/→)", "Token environment variable name", "Registered client ID (optional)", "Callback port (0 = automatic)"}

func connectionChoice(field int, value string) string {
	if field == fieldKind {
		if value == "stdio" {
			return "Local program"
		}
		return "Remote service"
	}
	switch value {
	case "oauth":
		return "Browser sign-in"
	case "bearer":
		return "API token"
	default:
		return "No sign-in"
	}
}

func newConnectionForm(s connections.Server) *connectionForm {
	f := &connectionForm{original: s, fields: make([]textinput.Model, fieldSave), args: textarea.New(), env: textarea.New()}
	values := []string{s.Name, s.Kind, s.Command, "", s.Directory, "", s.URL, s.Auth, s.TokenEnv, s.ClientID, strconv.Itoa(s.CallbackPort)}
	for i := range f.fields {
		t := textinput.New()
		t.CharLimit = 8192
		t.Prompt = "> "
		t.SetValue(values[i])
		f.fields[i] = t
	}
	f.fields[fieldName].Placeholder = "e.g. Work tools"
	f.fields[fieldURL].Placeholder = "https://your-mcp-provider.example/mcp"
	f.fields[fieldCommand].Placeholder = "Path to an installed MCP server program"
	for _, a := range []*textarea.Model{&f.args, &f.env} {
		a.ShowLineNumbers = false
		a.Prompt = "> "
		a.SetHeight(2)
		a.CharLimit = 128 * 8192
	}
	f.args.SetValue(strings.Join(s.Args, "\n"))
	f.env.SetValue(strings.Join(s.Env, "\n"))
	return f
}
func (f *connectionForm) visible() []int {
	v := []int{fieldName, fieldKind}
	if f.fields[fieldKind].Value() == "stdio" {
		v = append(v, fieldCommand, fieldAdvanced)
		if f.advanced {
			v = append(v, fieldArgs, fieldDir, fieldEnv)
		}
		return append(v, fieldSave)
	}
	v = append(v, fieldURL, fieldAuth)
	switch f.fields[fieldAuth].Value() {
	case "bearer":
		v = append(v, fieldToken)
	case "oauth":
		v = append(v, fieldAdvanced)
		if f.advanced {
			v = append(v, fieldClient, fieldPort)
		}
	}
	return append(v, fieldSave)
}
func (f *connectionForm) focus(width int) tea.Cmd {
	for i := range f.fields {
		f.fields[i].Blur()
		f.fields[i].Width = max(1, studioWidth(width)-5)
	}
	f.args.Blur()
	f.env.Blur()
	f.args.SetWidth(max(1, studioWidth(width)-2))
	f.env.SetWidth(max(1, studioWidth(width)-2))
	i := f.visible()[f.focusIndex]
	switch i {
	case fieldArgs:
		return f.args.Focus()
	case fieldEnv:
		return f.env.Focus()
	case fieldSave, fieldAdvanced, fieldKind, fieldAuth:
		return nil
	default:
		return f.fields[i].Focus()
	}
}
func (f *connectionForm) server() (connections.Server, error) {
	s := connections.Server{ID: f.original.ID, Name: strings.TrimSpace(f.fields[fieldName].Value()), Kind: f.fields[fieldKind].Value()}
	if s.Kind == "stdio" {
		s.Command = strings.TrimSpace(f.fields[fieldCommand].Value())
		s.Directory = strings.TrimSpace(f.fields[fieldDir].Value())
		if f.args.Value() != "" {
			s.Args = strings.Split(f.args.Value(), "\n")
		}
		for _, name := range strings.Split(f.env.Value(), "\n") {
			if name = strings.TrimSpace(name); name != "" {
				s.Env = append(s.Env, name)
			}
		}
	} else {
		s.URL = strings.TrimSpace(f.fields[fieldURL].Value())
		s.Auth = f.fields[fieldAuth].Value()
		if s.Auth == "bearer" {
			s.TokenEnv = strings.TrimSpace(f.fields[fieldToken].Value())
		}
		if s.Auth == "oauth" {
			s.ClientID = strings.TrimSpace(f.fields[fieldClient].Value())
			var err error
			s.CallbackPort, err = strconv.Atoi(f.fields[fieldPort].Value())
			if err != nil {
				return s, fmt.Errorf("callback port must be a number; use 0 for automatic")
			}
		}
	}
	return s, s.Validate()
}
func (m Model) connectionFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c, f := m.mcp, m.mcp.form
	key := msg.String()
	visible := f.visible()
	i := visible[f.focusIndex]
	if i == fieldAdvanced && (key == "enter" || key == " ") {
		f.advanced = !f.advanced
		return m, f.focus(m.width)
	}
	if key == "ctrl+s" || (key == "enter" && i == fieldSave) {
		s, err := f.server()
		if err != nil {
			c.failed = true
			c.message = err.Error()
			return m, nil
		}
		return m.beginConnection("save", s)
	}
	if key == "tab" || key == "shift+tab" || (key == "enter" && i != fieldArgs && i != fieldEnv) {
		delta := 1
		if key == "shift+tab" {
			delta = -1
		}
		f.focusIndex = (f.focusIndex + delta + len(visible)) % len(visible)
		return m, f.focus(m.width)
	}
	if (i == fieldKind || i == fieldAuth) && (key == "left" || key == "right" || key == " ") {
		values := []string{"stdio", "http"}
		if i == fieldAuth {
			values = []string{"none", "bearer", "oauth"}
		}
		n := 0
		for j, v := range values {
			if f.fields[i].Value() == v {
				n = j
			}
		}
		delta := 1
		if key == "left" {
			delta = -1
		}
		f.fields[i].SetValue(values[(n+delta+len(values))%len(values)])
		if i == fieldKind && f.fields[fieldAuth].Value() == "" {
			f.fields[fieldAuth].SetValue("oauth")
		}
		return m, f.focus(m.width)
	}
	var cmd tea.Cmd
	switch i {
	case fieldArgs:
		f.args, cmd = f.args.Update(msg)
	case fieldEnv:
		f.env, cmd = f.env.Update(msg)
	case fieldKind, fieldAuth, fieldSave, fieldAdvanced:
	default:
		f.fields[i], cmd = f.fields[i].Update(msg)
	}
	return m, cmd
}
func (m Model) connectionFormView(width int) string {
	c, f := m.mcp, m.mcp.form
	f.resize(m.width)
	title := "Add connection"
	if f.original.Name != "" {
		title = "Edit connection"
	}
	rows := []string{studioHeader(title, width)}
	hint := "Paste the MCP server URL from your provider, not its website address."
	if f.fields[fieldKind].Value() == "stdio" {
		hint = "Choose an installed MCP server. Extra options are under Advanced."
	}
	rows = append(rows, lipgloss.NewStyle().Foreground(welcomeMuted).Width(width).MaxHeight(2).Render(hint), "")
	if c.message != "" {
		rows = append(rows, lipgloss.NewStyle().Foreground(welcomePeach).Width(width).MaxHeight(2).Render(escapeControls(c.message)))
	}
	var content []string
	starts := map[int]int{}
	ends := map[int]int{}
	for _, i := range f.visible() {
		starts[i] = len(content)
		if i == fieldSave || i == fieldAdvanced {
			label := "Save connection"
			if i == fieldAdvanced {
				label = "+ Advanced settings"
				if f.advanced {
					label = "− Hide advanced settings"
				}
			}
			button := "  [ " + label + " ]"
			if i == f.visible()[f.focusIndex] {
				button = "> [ " + label + " ]"
			}
			content = append(content, button)
		} else {
			label := fieldLabels[i]
			style := lipgloss.NewStyle().Foreground(welcomeMuted)
			if i == f.visible()[f.focusIndex] {
				style = style.Foreground(welcomeMint).Bold(true)
				label = "> " + label
			} else {
				label = "  " + label
			}
			content = append(content, style.Render(label))
			value := ""
			switch i {
			case fieldArgs:
				value = f.args.View()
			case fieldEnv:
				value = f.env.View()
			case fieldKind, fieldAuth:
				value = "  ‹ " + connectionChoice(i, f.fields[i].Value()) + " ›"
			default:
				value = f.fields[i].View()
			}
			content = append(content, strings.Split(value, "\n")...)
		}
		content = append(content, "")
		ends[i] = len(content)
	}
	available := max(1, m.height-lipgloss.Height(strings.Join(rows, "\n"))-4)
	focused := f.visible()[f.focusIndex]
	start := max(0, ends[focused]-available)
	start = min(start, starts[focused])
	rows = append(rows, strings.Join(content[start:min(len(content), start+available)], "\n"))
	help := lipgloss.NewStyle().Foreground(welcomeMuted).Width(width).MaxHeight(2).Render("Tab Focus · ←→ Choices · Ctrl+S Save · Esc Back · Ctrl+C Quit")
	return m.welcomeCanvas(strings.Join(rows, "\n") + "\n" + help)
}

func (f *connectionForm) resize(width int) {
	for i := range f.fields {
		f.fields[i].Width = max(1, studioWidth(width)-5)
	}
	f.args.SetWidth(max(1, studioWidth(width)-2))
	f.env.SetWidth(max(1, studioWidth(width)-2))
	// Refresh the focused widget's horizontal/vertical scroll after resizing.
	f.update(nil)
}
func (f *connectionForm) update(msg tea.Msg) tea.Cmd {
	i := f.visible()[f.focusIndex]
	var cmd tea.Cmd
	switch i {
	case fieldArgs:
		f.args, cmd = f.args.Update(msg)
	case fieldEnv:
		f.env, cmd = f.env.Update(msg)
	case fieldSave, fieldAdvanced, fieldKind, fieldAuth:
	default:
		f.fields[i], cmd = f.fields[i].Update(msg)
	}
	return cmd
}

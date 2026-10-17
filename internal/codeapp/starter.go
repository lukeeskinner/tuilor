package codeapp

// Starter is host-owned. AI supplies only app.go and newModel. These helpers
// provide conventions, not a sandbox for generated Go code.
const Starter = `package main

import (
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 tea "github.com/charmbracelet/bubbletea"
 "github.com/charmbracelet/lipgloss"
)

var dataBroken bool

// loadData returns zero data for a new app, but never resets malformed data.
func loadData(value any) error {
 data, err := os.ReadFile(filepath.Join("data", "state.json"))
 if errors.Is(err, os.ErrNotExist) { return nil }
 if err == nil { err = json.Unmarshal(data, value) }
 if err != nil { dataBroken = true; return fmt.Errorf("load app data (preserved): %w", err) }
 return nil
}

func saveData(value any) error {
 if dataBroken { return fmt.Errorf("data is invalid; original file has been preserved") }
 data, err := json.MarshalIndent(value, "", "  ")
 if err != nil { return err }
 if err := os.MkdirAll("data", 0700); err != nil { return err }
 file, err := os.CreateTemp("data", ".write-")
 if err != nil { return err }
 defer os.Remove(file.Name())
 defer file.Close()
 if _, err := file.Write(append(data, '\n')); err != nil { return err }
 if err := file.Sync(); err != nil { return err }
 if err := file.Close(); err != nil { return err }
 return os.Rename(file.Name(), filepath.Join("data", "state.json"))
}

type frame struct { child tea.Model; width, height int }
func (m frame) Init() tea.Cmd { return m.child.Init() }
func (m frame) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
 if key, ok := msg.(tea.KeyMsg); ok && (key.String()=="esc" || key.String()=="ctrl+c") { return m, tea.Quit }
 if size, ok := msg.(tea.WindowSizeMsg); ok {
  m.width, m.height = max(1,size.Width), max(1,size.Height)
  msg=tea.WindowSizeMsg{Width:m.width,Height:max(1,m.height-1)}
 }
 child, cmd := m.child.Update(msg)
 if child != nil { m.child=child }
 return m,cmd
}
func (m frame) View() string {
 if m.width<20 || m.height<6 { return lipgloss.NewStyle().MaxWidth(max(1,m.width)).MaxHeight(max(1,m.height)).Render("Enlarge terminal. Esc returns.") }
 body:=lipgloss.NewStyle().Width(m.width).Height(m.height-1).MaxWidth(m.width).MaxHeight(m.height-1).Render(m.child.View())
 return body+"\n"+lipgloss.NewStyle().MaxWidth(m.width).Render("Esc / Ctrl+C Return to Tuilor")
}
func run() error {
 raw,err:=os.ReadFile(filepath.Join("data","state.json"))
 if err!=nil && !errors.Is(err,os.ErrNotExist) { return fmt.Errorf("read data (preserved): %w",err) }
 if err==nil && !json.Valid(raw) { return fmt.Errorf("invalid app data; original file preserved") }
 model,err:=newModel()
 if err!=nil { return err }
 if model==nil { return fmt.Errorf("newModel returned no model") }
 _,err=tea.NewProgram(frame{child:model,width:80,height:24},tea.WithAltScreen()).Run()
 return err
}
func main() {
 if err:=run();err!=nil {
  // Diagnostics are plain text even when the generated error contains controls.
  fmt.Fprintln(os.Stderr,strings.Map(func(r rune) rune { if r<32 || (r>=127 && r<=159) { return ' ' };return r },err.Error()))
  os.Exit(1)
 }
}
`

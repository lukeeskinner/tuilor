package generator

// A compiling host-contract reference with three deliberately different visual
// compositions. This is source context, never a generated application template.
const codeReference = `package main
import (
 "crypto/rand"
 "fmt"
 "strings"
 "github.com/charmbracelet/bubbles/textarea"
 "github.com/charmbracelet/bubbles/textinput"
 tea "github.com/charmbracelet/bubbletea"
 "github.com/charmbracelet/lipgloss"
)
type referenceItem struct { ID, Text string; Stage int }
type referenceModel struct {
 items []referenceItem
 input textinput.Model
 draft textarea.Model
 width, height, selected, layout int
 message string
}
func newModel() (tea.Model, error) {
 m := referenceModel{input:textinput.New(),draft:textarea.New()}
 m.draft.ShowLineNumbers=false
 if err := loadData(&m.items); err != nil { return nil,err }
 m.input.Placeholder = "Write an item; Enter saves"
 m.input.Focus()
 return m,nil
}
func (m referenceModel) Init() tea.Cmd { return textinput.Blink }
func (m referenceModel) Update(msg tea.Msg) (tea.Model,tea.Cmd) {
 if size,ok:=msg.(tea.WindowSizeMsg); ok {
  m.width,m.height=size.Width,size.Height
  m.input.Width=max(1,m.width-4)
  editorWidth:=m.width-4; if m.width>=65 { editorWidth=m.width-max(20,m.width/4)-5 }
  m.draft.SetWidth(max(1,editorWidth));m.draft.SetHeight(max(1,m.height-11))
  return m,nil
 }
 if key,ok:=msg.(tea.KeyMsg); ok {
  switch key.String() {
  case "f1": m.layout=0;m.draft.Blur();m.input.Focus(); return m,textinput.Blink
  case "f2": m.layout=1;m.draft.Blur();m.input.Focus(); return m,textinput.Blink
  case "f3":
   m.layout=2;m.input.Blur();m.draft.Focus()
   if len(m.items)>0 { m.draft.SetValue(m.items[m.selected].Text) }
   return m,textarea.Blink
  case "ctrl+s":
   if m.layout!=2 || strings.TrimSpace(m.draft.Value())=="" { return m,nil }
   next:=append([]referenceItem{},m.items...)
   if len(next)==0 { next=append(next,referenceItem{ID:rand.Text()});m.selected=0 }
   next[m.selected].Text=m.draft.Value()
   if err:=saveData(next);err!=nil { m.message=err.Error();return m,nil }
   m.items,m.message=next,"Document saved";return m,nil
  }
  if m.layout==2 { var cmd tea.Cmd;m.draft,cmd=m.draft.Update(msg);return m,cmd }
  switch key.String() {
  case "up": m.selected=max(0,m.selected-1); return m,nil
  case "down": m.selected=min(max(0,len(m.items)-1),m.selected+1); return m,nil
  case "ctrl+t":
   if len(m.items)==0 { return m,nil }
   next:=append([]referenceItem{},m.items...)
   next[m.selected].Stage=(next[m.selected].Stage+1)%3
   if err:=saveData(next); err!=nil { m.message=err.Error(); return m,nil }
   m.items,m.message=next,"Moved and saved"; return m,nil
  case "enter":
   value:=strings.TrimSpace(m.input.Value())
   if value=="" { m.message="Enter some text"; return m,nil }
   next:=append(append([]referenceItem{},m.items...),referenceItem{ID:rand.Text(),Text:value})
   if err:=saveData(next); err!=nil { m.message=err.Error(); return m,nil }
   m.items,m.selected,m.message=next,len(next)-1,"Saved locally"
   m.input.SetValue(""); return m,nil
  }
 }
 var cmd tea.Cmd
 m.input,cmd=m.input.Update(msg)
 return m,cmd
}
func (m referenceModel) View() string {
 if m.width<24 || m.height<9 { return uiFit("Reference workspace\nEnlarge terminal · Esc returns",m.width,m.height) }
 palette:=[][2]string{{"#F6AE82","#BBDDAD"},{"#CDB6F8","#EAC46F"},{"#90D6E7","#EDB2C9"}}
 t:=uiPalette(palette[m.layout][0],palette[m.layout][1])
 m.input.PromptStyle=uiPaint(t.Secondary)
 m.input.TextStyle=uiPaint(t.Ink)
 m.input.PlaceholderStyle=uiPaint(t.Muted)
 m.input.CompletionStyle=uiPaint(t.Muted)
 m.input.Cursor.Style=uiPaint(t.Accent)
 title:=uiTitle(t,"COMPOSITION STUDY · REFERENCE ONLY",[]string{"Review desk","Work in motion","Writing room"}[m.layout],"",m.width)
 tabs:=uiTabs(t,[]string{"Workspace","Board","Editor"},m.layout,m.width)
 help:=uiHelp(t,[]uiKey{{"Enter","add"},{"Up/Down","select"},{"Ctrl+T","move"},{"F1/F2/F3","layout"},{"Esc","return"}},m.width)
 if m.layout==2 { help=uiHelp(t,[]uiKey{{"Ctrl+S","save document"},{"F1/F2","browse"},{"Esc","return"}},m.width) }
 status:=uiStatus(t,"info",fmt.Sprintf("%d saved items · %s",len(m.items),m.message),m.width)
 available:=max(1,m.height-lipgloss.Height(title)-lipgloss.Height(tabs)-lipgloss.Height(help)-lipgloss.Height(status)-1)
 body:=""
 switch m.layout {
 case 0: body=m.workspace(t,m.width,available)
 case 1: body=m.board(t,m.width,available)
 case 2: body=m.editor(t,m.width,available)
 }
 entry:=m.input.View();if m.layout==2 { entry=uiPaint(t.Muted).Render("LOCAL DOCUMENT / edits save with Ctrl+S") }
 return uiFit(strings.Join([]string{title,tabs,body,entry,status,help},"\n"),m.width,m.height)
}
func (m referenceModel) rows() [][]string {
 rows:=[][]string{}
 for _,item:=range m.items { rows=append(rows,[]string{item.Text,[]string{"Queued","Active","Done"}[item.Stage]}) }
 return rows
}
func (m referenceModel) detail() string {
 if len(m.items)==0 { return "Nothing selected.\nAdd your first item below." }
 return uiText(m.items[m.selected].Text)+"\n\nState: "+[]string{"Queued","Active","Done"}[m.items[m.selected].Stage]+"\nCtrl+T moves this item to the next lane."
}
// Workspace: compact queue and a contrasting detail area.
func (m referenceModel) workspace(t uiTheme,w,h int) string {
 if w<65 { return uiTable(t,[]string{"Item","State"},m.rows(),w,h,m.selected) }
 left:=max(24,w*2/5)
 queue:=uiPanel(t,"QUEUE / "+fmt.Sprint(len(m.items)),uiTable(t,[]string{"Item","State"},m.rows(),left-4,h-3,m.selected),left,h,true)
 detail:=uiPanel(t,"SELECTED / DETAILS",m.detail(),w-left-1,h,false)
 return lipgloss.JoinHorizontal(lipgloss.Top,queue," ",detail)
}
// Board: horizontal lanes become a single focused lane on narrow terminals.
func (m referenceModel) board(t uiTheme,w,h int) string {
 names:=[]string{"Queued","Active","Done"}; stage:=0
 if len(m.items)>0 { stage=m.items[m.selected].Stage }
 lane:=func(s,width,height int) string {
  rows:=[][]string{}; selected:=-1
  for i,item:=range m.items { if item.Stage==s { if i==m.selected { selected=len(rows) }; rows=append(rows,[]string{item.Text}) } }
  return uiPanel(t,names[s]+" / "+fmt.Sprint(len(rows)),uiTable(t,[]string{"Items"},rows,width-4,height-3,selected),width,height,s==stage)
 }
 if w<78 { return uiTabs(t,names,stage,w)+"\n"+lane(stage,w,max(1,h-1)) }
 laneWidth:=(w-2)/3
 return lipgloss.JoinHorizontal(lipgloss.Top,lane(0,laneWidth,h)," ",lane(1,laneWidth,h)," ",lane(2,w-2-2*laneWidth,h))
}
// Editor: document rail, quiet writing surface and a compact metadata strip.
func (m referenceModel) editor(t uiTheme,w,h int) string {
 style:=textarea.Style{Base:uiPaint(t.Ink),Text:uiPaint(t.Ink),Placeholder:uiPaint(t.Muted),Prompt:uiPaint(t.Secondary),CursorLine:uiPaint(t.Ink)}
 m.draft.FocusedStyle,m.draft.BlurredStyle=style,style;m.draft.Cursor.Style=uiPaint(t.Accent)
 if w<65 { return uiPanel(t,"DOCUMENT",m.draft.View(),w,h,true) }
 railWidth:=max(20,w/4)
 rail:=uiPanel(t,"DOCUMENTS",uiTable(t,[]string{"Title"},m.rows(),railWidth-4,h-3,m.selected),railWidth,h,false)
 document:=uiPanel(t,"WRITING / SELECTED ITEM",m.draft.View(),w-railWidth-1,h,true)
 return lipgloss.JoinHorizontal(lipgloss.Top,rail," ",document)
}
`

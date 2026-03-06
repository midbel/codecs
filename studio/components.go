package studio

import (
	"path/filepath"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type strItem string

func (s strItem) Title() string {
	return string(s)
}

func (s strItem) Description() string {
	return ""
}

func (s strItem) FilterValue() string {
	return ""
}

type Screen interface {
	Init() tea.Cmd
	Update(tea.Msg) (Screen, tea.Cmd)
	View() string
	Resize(int, int)
	Mode() Mode
}

type queryScreen struct {
	ring   *FocusRing
	width  int
	height int

	uiDoc   *scrollText
	uiRes   *scrollText
	uiQuery *textInput

	err  error
	doc  string
	file string
}

func newQueryScreen(file string) Screen {
	mod := &queryScreen{
		file:    file,
		uiDoc:   newScroll(""),
		uiRes:   newScroll(""),
		uiQuery: newText("/root/element[@attr='value']"),
		ring:    new(FocusRing),
	}
	mod.uiQuery.Focus()

	mod.ring.Push(mod.uiQuery)
	mod.ring.Push(mod.uiDoc)
	mod.ring.Push(mod.uiRes)
	return mod
}

func (queryScreen) Mode() Mode {
	return ModeNone
}

func (m *queryScreen) Init() tea.Cmd {
	return nil
}

func (m *queryScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	var (
		cmd  tea.Cmd
		list []tea.Cmd
	)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd = m.updateKeys(msg)
		if cmd != nil {
			list = append(list, cmd)
		}
	case resultMsg:
		m.err = msg.err
		if msg.err != nil {
			m.uiRes.ClearValue()
		} else {
			m.uiRes.SetValue(msg.result)
		}
		m.uiRes.Reset()
		m.uiDoc.Reset()
	case queryMsg:
		m.uiRes.ClearValue()
	case documentMsg:
		m.doc = msg.doc
		m.err = msg.err
		m.uiDoc.SetValue(m.doc)
		m.uiDoc.Reset()
	}
	return m, tea.Batch(list...)
}

func (m *queryScreen) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	screen := lipgloss.JoinVertical(lipgloss.Top, m.headerView(), m.bodyView(), m.footerView())
	return screen
}

func (m *queryScreen) Resize(width, height int) {
	headerHeight := lipgloss.Height(m.headerView())
	footerHeight := lipgloss.Height(m.footerView())
	verticalMarginHeight := headerHeight + footerHeight

	m.uiQuery.Resize(width*60/100, 0)

	space := width * 60 / 100

	m.uiDoc.Resize(space, height-verticalMarginHeight)
	m.uiDoc.SetY(headerHeight)
	m.uiRes.Resize(width-space, height-verticalMarginHeight)
	m.uiRes.SetY(headerHeight)

	m.width = width
	m.height = height
}

func (m *queryScreen) headerView() string {
	var (
		header string
		style  = lipgloss.NewStyle().MarginBottom(1)
	)
	if m.err != nil {
		header = lipgloss.JoinHorizontal(lipgloss.Left, m.uiQuery.View(), m.err.Error())
	} else {
		header = m.uiQuery.View()
	}
	return style.Render(header)
}

func (m *queryScreen) bodyView() string {
	return lipgloss.JoinHorizontal(lipgloss.Left, m.uiDoc.View(), m.uiRes.View())
}

func (m *queryScreen) footerView() string {
	left := filepath.Base(m.file)

	right := lipgloss.NewStyle().Width(max(0, m.width-lipgloss.Width(left))).Align(lipgloss.Right)

	line := lipgloss.JoinHorizontal(lipgloss.Top, left, right.Render("esc to quit"))
	return lipgloss.NewStyle().MarginTop(1).Render(line)
}

func (m *queryScreen) updateKeys(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch msg.String() {
	case "tab":
		cmd = m.ring.Next()
	case "shift+tab":
		cmd = m.ring.Prev()
	default:
		cmd = m.ring.Update(msg)
	}
	return cmd
}

type historyScreen struct {
	width  int
	height int

	list  list.Model
	style lipgloss.Style
}

func newHistoryScreen(history []string) Screen {
	sc := &historyScreen{
		list:  list.New(nil, list.NewDefaultDelegate(), 0, 0),
		style: lipgloss.NewStyle().Border(lipgloss.RoundedBorder()),
	}

	var items []list.Item
	for i := len(history) - 1; i >= 0; i-- {
		it := strItem(history[i])
		items = append(items, it)
	}
	sc.list.SetItems(items)
	sc.list.Title = ""

	return sc
}

func (*historyScreen) Mode() Mode {
	return ModeHistory
}

func (m *historyScreen) Init() tea.Cmd {
	return nil
}

func (m *historyScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *historyScreen) View() string {
	return m.style.Width(m.width).Render(m.list.View())
}

func (m *historyScreen) Resize(width int, height int) {
	frameW := m.style.GetHorizontalFrameSize()
	frameH := m.style.GetVerticalFrameSize()

	m.list.SetWidth(width - frameW)
	m.list.SetHeight(height - frameH)

	m.width = width
	m.height = height
}

type scrollText struct {
	scroll viewport.Model
	doc    textarea.Model
	style  lipgloss.Style
}

func newScroll(placeholder string) *scrollText {
	sc := &scrollText{
		scroll: viewport.New(),
		doc:    textarea.New(),
		style:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()),
	}
	sc.doc.ShowLineNumbers = false
	sc.doc.CharLimit = 0
	sc.doc.Placeholder = placeholder

	sc.scroll.Style = lipgloss.NewStyle().Padding(1, 2)

	sc.SyncContent()

	return sc
}

func (m *scrollText) SetY(y int) {
	m.scroll.YPosition = y
}

func (m *scrollText) Focus() tea.Cmd {
	return m.doc.Focus()
}

func (m *scrollText) Blur() {
	m.doc.Blur()
}

func (m *scrollText) Resize(width, height int) {
	frameW := m.style.GetHorizontalFrameSize()
	frameH := m.style.GetVerticalFrameSize()

	m.scroll.SetWidth(width - frameW)
	m.scroll.SetHeight(height - frameH)
	m.doc.SetWidth(width - frameW)
	m.doc.SetHeight(height - frameH)
}

func (m *scrollText) ClearValue() {
	m.SetValue("")
}

func (m *scrollText) SetValue(value string) {
	m.doc.SetValue(value)
}

func (m *scrollText) Reset() {
	m.doc.MoveToBegin()
	m.scroll.GotoTop()
}

func (m *scrollText) SyncContent() {
	m.scroll.SetContent(m.doc.View())
}

func (m *scrollText) Update(msg tea.Msg) tea.Cmd {
	var (
		list []tea.Cmd
		cmd  tea.Cmd
	)

	m.doc, cmd = m.doc.Update(msg)
	if cmd != nil {
		list = append(list, cmd)
	}
	m.scroll, _ = m.scroll.Update(msg)

	return tea.Batch(list...)
}

func (m *scrollText) View() string {
	m.SyncContent()
	return m.style.Render(m.scroll.View())
}

type textInput struct {
	text textinput.Model
}

func newText(placeholder string) *textInput {
	in := textInput{
		text: textinput.New(),
	}
	in.text.Placeholder = placeholder
	return &in
}

func (m *textInput) Resize(width, _ int) {
	m.text.SetWidth(width)
}

func (m *textInput) Focus() tea.Cmd {
	return m.text.Focus()
}

func (m *textInput) Blur() {
	m.text.Blur()
}

func (m *textInput) Update(msg tea.Msg) tea.Cmd {
	if !m.text.Focused() {
		return nil
	}
	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "enter" {
		return m.Submit()
	}
	var cmd tea.Cmd
	m.text, cmd = m.text.Update(msg)
	return cmd
}

func (m *textInput) View() string {
	return m.text.View()
}

func (m *textInput) Submit() tea.Cmd {
	return func() tea.Msg {
		return queryMsg{
			query: m.text.Value(),
		}
	}
}

package studio

import (
	tea "charm.land/bubbletea/v2"
)

type QueryApp struct {
	width  int
	height int

	file    string
	stack   *Stack
	history *History[string]
}

func NewQueryApp(file string) *QueryApp {
	mod := &QueryApp{
		stack:   NewStack(),
		history: NewHistory[string](50),
		file:    file,
	}
	mod.stack.Push(newQueryScreen(file))
	return mod
}

func (m *QueryApp) Init() tea.Cmd {
	return tea.Batch(parseDocument(m.file))
}

func (m *QueryApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd = m.updateKeys(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.stack.Current().Resize(m.width, m.height)
	case queryMsg:
		m.history.Push(msg.query)
		cmd = executeQuery(m.file, msg.query)
	}
	scmd := m.stack.Update(msg)
	return m, tea.Batch(cmd, scmd)
}

func (m *QueryApp) View() tea.View {
	var (
		view = tea.NewView("")
		curr = m.stack.Current()
	)
	view.AltScreen = true
	view.SetContent(curr.View())
	return view
}

func (m *QueryApp) updateKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "ctrl+q":
		return tea.Quit
	case "backspace":
		curr := m.stack.Current()
		if curr == nil || curr.Mode() == ModeNone {
			break
		}
		m.stack.Pop()
	case "f1":
	case "f2":
	case "f9":
		curr := m.stack.Current()
		if curr != nil && curr.Mode() == ModeHistory {
			m.stack.Pop()
			break
		}
		s := newHistoryScreen(m.history.All())
		s.Resize(m.width, m.height)
		m.stack.Push(s)
	default:
	}
	return nil
}

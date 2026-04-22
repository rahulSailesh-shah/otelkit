package tui

import (
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type procOutModel struct {
	path    string
	vp      viewport.Model
	width   int
	height  int
	tailing bool
}

func newProcOutModel(path string) procOutModel {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	return procOutModel{
		path:    path,
		vp:      vp,
		tailing: true,
	}
}

func (m procOutModel) setSize(w, h int) procOutModel {
	m.width, m.height = w, h
	m.vp.SetWidth(max(40, w-4))
	m.vp.SetHeight(max(5, h-4))
	return m
}

func (m procOutModel) View() string {
	header := titleStyle.Render("Process Output") +
		helpStyle.Render("  ↑/↓ scroll · updates every 500ms · "+m.path)
	body := borderStyle.Render(m.vp.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func (m procOutModel) tick() procOutModel {
	content := readFileSafe(m.path)
	m.vp.SetContent(content)
	if m.tailing {
		m.vp.GotoBottom()
	}
	return m
}

func (m procOutModel) Update(msg tea.Msg) (procOutModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func procOutTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return procOutTickMsg(t) })
}

type procOutTickMsg time.Time

func readFileSafe(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return lipgloss.NewStyle().Foreground(colorMuted).Italic(true).
			Render("waiting for output... (" + err.Error() + ")")
	}
	if len(b) == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Italic(true).
			Render("(no output yet)")
	}
	s := string(b)
	// Drop trailing blank line for tidier display.
	s = strings.TrimRight(s, "\n")
	return s
}

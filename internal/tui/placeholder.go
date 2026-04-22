package tui

import "charm.land/lipgloss/v2"

type placeholderModel struct {
	msg    string
	width  int
	height int
}

func newPlaceholder(msg string) placeholderModel {
	return placeholderModel{msg: msg}
}

func (m placeholderModel) View() string {
	box := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(colorMuted).
		Italic(true)
	return box.Render(m.msg)
}

func (m placeholderModel) setSize(w, h int) placeholderModel {
	m.width, m.height = w, h
	return m
}

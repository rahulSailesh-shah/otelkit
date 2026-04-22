package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	colorBrand   = lipgloss.Color("#7D56F4")
	colorMuted   = lipgloss.Color("#626880")
	colorText    = lipgloss.Color("#E5E7EB")
	colorError   = lipgloss.Color("#F38BA8")
	colorSuccess = lipgloss.Color("#A6E3A1")
	colorAccent  = lipgloss.Color("#F9E2AF")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrand)

	tabActive = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true).
			Foreground(colorText).
			Background(colorBrand)
	tabInactive = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(colorMuted)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

	waterfallEmptyStyle  = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	waterfallCursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("#313244")).Foreground(colorText)

	barNormal = lipgloss.NewStyle().Foreground(colorBrand)
	barError  = lipgloss.NewStyle().Foreground(colorError)
	barRoot   = lipgloss.NewStyle().Foreground(colorSuccess)
	barSlow   = lipgloss.NewStyle().Foreground(colorAccent)

	attrKey = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	attrVal = lipgloss.NewStyle().Foreground(colorText)
)

func stylesForBarLG(s BarStyle) lipgloss.Style {
	switch s {
	case BarError:
		return barError
	case BarRoot:
		return barRoot
	case BarSlow:
		return barSlow
	default:
		return barNormal
	}
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

func joinTabs(labels []string, active int) string {
	cells := make([]string, len(labels))
	for i, l := range labels {
		if i == active {
			cells[i] = tabActive.Render(l)
		} else {
			cells[i] = tabInactive.Render(l)
		}
	}
	return strings.Join(cells, " ")
}

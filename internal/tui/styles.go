package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var (
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

	waterfallEmptyStyle    = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	waterfallCursorStyle   = lipgloss.NewStyle().Background(colorSelect).Foreground(colorText)
	waterfallDurationStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	barNormal = lipgloss.NewStyle().Foreground(colorBrand)
	barError  = lipgloss.NewStyle().Foreground(colorError)
	barRoot   = lipgloss.NewStyle().Foreground(colorSuccess)
	barSlow   = lipgloss.NewStyle().Foreground(colorAccent)

	attrKey = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	attrVal = lipgloss.NewStyle().Foreground(colorText)

	kpiCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 2).
			MarginRight(1)
)

func stylesForBarLG(s BarStyle) lipgloss.Style {
	switch s {
	case BarError:
		return barError
	case BarRoot:
		return barRoot
	default:
		return barNormal
	}
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

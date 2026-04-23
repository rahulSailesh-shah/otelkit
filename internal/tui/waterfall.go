package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

type BarStyle int

const (
	BarNormal BarStyle = iota
	BarError
	BarRoot
)

type Bar struct {
	Offset int
	Width  int
	Style  BarStyle
}

func layoutBars(spans []repo.Span, width int) []Bar {
	if len(spans) == 0 {
		return nil
	}
	if width <= 0 {
		width = 1
	}

	minStart := spans[0].StartTimeNs
	maxEnd := spans[0].EndTimeNs
	for _, s := range spans[1:] {
		if s.StartTimeNs < minStart {
			minStart = s.StartTimeNs
		}
		if s.EndTimeNs > maxEnd {
			maxEnd = s.EndTimeNs
		}
	}
	span := maxEnd - minStart
	if span <= 0 {
		span = 1
	}

	bars := make([]Bar, len(spans))
	for i, s := range spans {
		startOff := s.StartTimeNs - minStart
		dur := s.EndTimeNs - s.StartTimeNs
		offset := int(float64(startOff) / float64(span) * float64(width))
		w := int(float64(dur) / float64(span) * float64(width))
		if w < 1 {
			w = 1
		}
		if offset+w > width {
			w = width - offset
			if w < 1 {
				w = 1
			}
		}
		bars[i] = Bar{
			Offset: offset,
			Width:  w,
			Style:  barStyleFor(s),
		}
	}
	return bars
}

func barStyleFor(s repo.Span) BarStyle {
	if s.StatusCode == 2 {
		return BarError
	}
	if s.ParentSpanID == nil {
		return BarRoot
	}
	return BarNormal
}

// waterfallModel renders the waterfall under the trace list.
type waterfallModel struct {
	width  int
	height int
	spans  []repo.Span
	cursor int
}

func (m waterfallModel) Init() tea.Cmd { return nil }

func (m waterfallModel) Update(msg tea.Msg) (waterfallModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.spans)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m waterfallModel) View() string {
	if len(m.spans) == 0 {
		return waterfallEmptyStyle.Render("select a trace to see its waterfall")
	}
	trackWidth := max(10, m.width-40)
	bars := layoutBars(m.spans, trackWidth)

	var b strings.Builder
	for i, s := range m.spans {
		label := fmt.Sprintf("%-22s %-16s", truncate(s.ServiceName, 22), truncate(s.Name, 16))
		dur := fmtDuration(s.EndTimeNs - s.StartTimeNs)
		track := renderTrack(bars[i], trackWidth, dur)
		line := fmt.Sprintf("%s %s", label, track)
		if i == m.cursor {
			line = waterfallCursorStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// renderTrack draws the bar on a `width`-wide track and inlines the duration
// label adjacent to the bar. It prefers the leading edge (just before the bar),
// falls back to the trailing edge, and as a last resort overlays the label at
// the start of the bar itself (used for root spans that consume the entire
// track). This matches the Jaeger-style affordance where the duration floats
// next to its bar.
func renderTrack(bar Bar, width int, label string) string {
	if width <= 0 {
		width = 1
	}
	leftSpaces := bar.Offset
	rightSpaces := max(0, width-bar.Offset-bar.Width)
	labelLen := len(label)

	leftStr := strings.Repeat(" ", leftSpaces)
	rightStr := strings.Repeat(" ", rightSpaces)
	filled := stylesForBarLG(bar.Style).Render(strings.Repeat("█", bar.Width))

	switch {
	case leftSpaces >= labelLen+1:
		leftStr = strings.Repeat(" ", leftSpaces-labelLen-1) +
			waterfallDurationStyle.Render(label) + " "
	case rightSpaces >= labelLen+1:
		rightStr = " " + waterfallDurationStyle.Render(label) +
			strings.Repeat(" ", rightSpaces-labelLen-1)
	case bar.Width >= labelLen+2:
		remaining := bar.Width - labelLen - 1
		filled = waterfallDurationStyle.Render(label) + " " +
			stylesForBarLG(bar.Style).Render(strings.Repeat("█", remaining))
	}

	return leftStr + filled + rightStr
}

func (m waterfallModel) selectedSpan() (repo.Span, bool) {
	if m.cursor < 0 || m.cursor >= len(m.spans) {
		return repo.Span{}, false
	}
	return m.spans[m.cursor], true
}

func (m *waterfallModel) setSize(w, h int) { m.width, m.height = w, h }

func (m *waterfallModel) setSpans(spans []repo.Span) {
	m.spans = spans
	if m.cursor >= len(spans) {
		m.cursor = 0
	}
}

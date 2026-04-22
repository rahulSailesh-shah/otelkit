package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

// BarStyle selects a colour/theme for a bar in the waterfall.
type BarStyle int

const (
	BarNormal BarStyle = iota
	BarError
	BarSlow
	BarRoot
)

// Bar is a laid-out span on the waterfall track.
type Bar struct {
	Offset int
	Width  int
	Style  BarStyle
}

// layoutBars normalises span start/end against the overall trace window
// and returns per-span offsets and widths that fit within `width` columns.
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
		return waterfallEmpty.Render("select a trace to see its waterfall")
	}
	bars := layoutBars(m.spans, max(10, m.width-40))

	var b strings.Builder
	for i, s := range m.spans {
		label := fmt.Sprintf("%-22s %-16s", truncate(s.ServiceName, 22), truncate(s.Name, 16))
		track := renderTrack(bars[i], max(10, m.width-40))
		line := fmt.Sprintf("%s %s %s", label, track, fmtDuration(s.EndTimeNs-s.StartTimeNs))
		if i == m.cursor {
			line = waterfallCursor.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

func renderTrack(bar Bar, width int) string {
	if width <= 0 {
		width = 1
	}
	left := strings.Repeat(" ", bar.Offset)
	filled := strings.Repeat("█", bar.Width)
	right := strings.Repeat(" ", max(0, width-bar.Offset-bar.Width))
	return left + stylesForBar(bar.Style).Render(filled) + right
}

// selectedSpan returns the currently highlighted span, if any.
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

// temporary stubs until Task 10 lands — replaced by styles.go contents
var (
	waterfallEmpty  = stubStyle{}
	waterfallCursor = stubStyle{}
)

type stubStyle struct{}

func (stubStyle) Render(s ...string) string { return strings.Join(s, "") }

func stylesForBar(_ BarStyle) stubStyle { return stubStyle{} }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func fmtDuration(ns int64) string {
	if ns >= int64(1e9) {
		return fmt.Sprintf("%.2fs", float64(ns)/1e9)
	}
	if ns >= int64(1e6) {
		return fmt.Sprintf("%dms", ns/int64(1e6))
	}
	if ns >= int64(1e3) {
		return fmt.Sprintf("%dµs", ns/int64(1e3))
	}
	return fmt.Sprintf("%dns", ns)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

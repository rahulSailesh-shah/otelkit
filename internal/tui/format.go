package tui

import (
	"fmt"
	"time"
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
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

func fmtFloat(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.3g", v)
}

func nowFmt() string { return time.Now().Format("15:04:05") }

package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

type tabID int

const (
	tabTraces tabID = iota
	tabMetrics
	tabLogs
	tabProcOut
)

type Tab interface {
	Label() string
	Update(ctx context.Context, q *repo.Queries, msg tea.Msg) (Tab, tea.Cmd)
	View() string
	HelpKeys() []TabKey
	SetSize(w, h int)
	RefreshCmd(ctx context.Context, q *repo.Queries) tea.Cmd
	OnLeave()
	ConsumesTab() bool
	ConsumesEnter() bool
}

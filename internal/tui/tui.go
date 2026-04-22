package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

// Options configures the TUI entry point.
type Options struct {
	// Queries is a read-only (or read/write) repo.Queries. REQUIRED.
	Queries *repo.Queries
	// RefreshInterval controls the polling cadence; defaults to 2s when zero.
	RefreshInterval time.Duration
	// ChildLogPath, when non-empty, enables the Process Output tab that tails this file.
	ChildLogPath string
}

// Run blocks until the TUI exits (user quits or ctx is cancelled).
func Run(ctx context.Context, opts Options) error {
	if opts.RefreshInterval <= 0 {
		opts.RefreshInterval = 2 * time.Second
	}
	m := newAppModel(ctx, opts)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

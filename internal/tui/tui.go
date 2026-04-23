package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

type Options struct {
	Queries         *repo.Queries
	RefreshInterval time.Duration
	ChildLogPath    string
}

func Run(ctx context.Context, opts Options) error {
	if opts.RefreshInterval <= 0 {
		opts.RefreshInterval = 2 * time.Second
	}
	m := newAppModel(ctx, opts)
	p := tea.NewProgram(m, tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

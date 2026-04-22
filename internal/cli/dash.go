package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/rahulSailesh-shah/otelkit/internal/store/db"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
	"github.com/rahulSailesh-shah/otelkit/internal/tui"
)

type dashOptions struct {
	Refresh time.Duration
}

func newDashCmd() *cobra.Command {
	var opts dashOptions
	cmd := &cobra.Command{
		Use:   "dash",
		Short: "Open the otelkit TUI dashboard against an existing database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return dashExecute(cmd.Context(), opts)
		},
	}
	cmd.Flags().DurationVar(&opts.Refresh, "refresh", 2*time.Second, "TUI refresh interval")
	return cmd
}

func dashExecute(ctx context.Context, opts dashOptions) error {
	database := db.NewSQLiteDB(ctx, globalOpts.DBPath)
	if err := database.Connect(); err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	queries := repo.New(database.GetReadDB())
	return tui.Run(ctx, tui.Options{
		Queries:         queries,
		RefreshInterval: opts.Refresh,
	})
}

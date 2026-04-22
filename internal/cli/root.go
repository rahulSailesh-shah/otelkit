package cli

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

type globalOptions struct {
	DBPath   string
	LogLevel string
}

var globalOpts globalOptions

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "otelkit",
		Short: "Launcher-mode observability toolkit",
		Long: "otelkit runs an OTLP receiver, stores telemetry locally in SQLite, " +
			"and renders a TUI dashboard.",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&globalOpts.DBPath, "db", "otelkit.db", "path to SQLite database")
	cmd.PersistentFlags().StringVar(&globalOpts.LogLevel, "log-level", "info", "log level (debug, info, warn, error)")

	cmd.AddCommand(newRunCmd(), newDashCmd(), newVersionCmd())
	return cmd
}

// Execute builds the root command wired to a Ctrl-C cancellable context and runs it.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return newRootCmd().ExecuteContext(ctx)
}

// temporary stubs until subcommand files land
func newRunCmd() *cobra.Command     { return &cobra.Command{Use: "run"} }
func newDashCmd() *cobra.Command    { return &cobra.Command{Use: "dash"} }
func newVersionCmd() *cobra.Command { return &cobra.Command{Use: "version"} }

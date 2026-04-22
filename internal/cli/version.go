package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is overridable via -ldflags at build time, e.g.
//   go build -ldflags "-X github.com/rahulSailesh-shah/otelkit/internal/cli.Version=v0.1.0"
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the otelkit version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "otelkit %s\n", Version)
			return err
		},
	}
}

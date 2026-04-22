package launcher

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// IOConfig optionally overrides child stdio. Nil fields fall back to os.Stdin/out/err.
type IOConfig struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run spawns command[0] with command[1:] as arguments, appends extraEnv to the
// inherited environment, wires stdio per IOConfig (falling back to the parent
// process), forwards SIGINT/SIGTERM to the child, and returns the child's exit
// code.
func Run(_ context.Context, command []string, extraEnv []string, io IOConfig) (int, error) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdin = firstReader(io.Stdin, os.Stdin)
	cmd.Stdout = firstWriter(io.Stdout, os.Stdout)
	cmd.Stderr = firstWriter(io.Stderr, os.Stderr)

	if err := cmd.Start(); err != nil {
		return 0, err
	}

	// Forward SIGINT and SIGTERM to child so it can shut down gracefully.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigCh {
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
		}
	}()

	err := cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

func firstReader(r io.Reader, fallback io.Reader) io.Reader {
	if r != nil {
		return r
	}
	return fallback
}

func firstWriter(w io.Writer, fallback io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return fallback
}

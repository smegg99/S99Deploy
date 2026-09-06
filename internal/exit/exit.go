// internal/exit/exit.go

// Package exit is the exit-code contract both binaries share.
package exit

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// The three codes, so a script can tell a usage mistake from work that failed.
const (
	OK    = 0
	Error = 1
	Usage = 2
)

// workError marks an error a command's own work returned.
type workError struct{ err error }

func (e workError) Error() string { return e.err.Error() }
func (e workError) Unwrap() error { return e.err }

// Work wraps a RunE body so its failures classify as Error.
func Work(fn func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	// Everything cobra itself returns reaches Run unwrapped, as a Usage error.
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			return workError{err}
		}
		return nil
	}
}

// Texts are the two strings Run writes itself, already localized.
type Texts struct{ Name, Interrupted string }

// Run executes root and turns its error into a process exit code.
func Run(ctx context.Context, root *cobra.Command, stderr io.Writer, texts Texts) int {
	cmd, err := root.ExecuteContextC(ctx)
	switch {
	case err == nil:
		return OK
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(stderr, texts.Interrupted)
		return Error
	}

	var work workError
	if errors.As(err, &work) {
		fmt.Fprintln(stderr, texts.Name+":", err)
		return Error
	}
	if cmd == nil {
		cmd = root
	}
	fmt.Fprintln(stderr, texts.Name+":", err)
	fmt.Fprint(stderr, cmd.UsageString())
	return Usage
}

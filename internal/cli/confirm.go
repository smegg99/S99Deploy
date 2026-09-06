// internal/cli/confirm.go

package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// confirmName makes the destructive path type the app's own name.
func confirmName(in io.Reader, out io.Writer, name, root string, purge bool) error {
	// It reads the command's input rather than os.Stdin, so a test can answer it.
	fmt.Fprintf(out, "This disables and removes the %s service.\n", name)
	if purge {
		fmt.Fprintf(out, "--purge also deletes %s (checkout, .env, logs) and the %s account.\n", root, name)
	}
	fmt.Fprintf(out, "Type %q to confirm: ", name)

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != name {
		return fmt.Errorf("aborted")
	}
	return nil
}

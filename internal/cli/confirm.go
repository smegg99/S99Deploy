// internal/cli/confirm.go

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/smegg99/s99deploy/internal/messages"
)

// confirmName makes the destructive path type the app's own name.
func confirmName(in io.Reader, out io.Writer, words *i18n.Localizer, name, root string, purge bool) error {
	// It reads the command's input rather than os.Stdin, so a test can answer it.
	fmt.Fprintln(out, messages.CliConfirmRemovesService(words,
		messages.CliConfirmRemovesServiceParams{Name: name}))
	if purge {
		fmt.Fprintln(out, messages.CliConfirmPurgeAlso(words,
			messages.CliConfirmPurgeAlsoParams{Root: root, Name: name}))
	}
	fmt.Fprint(out, messages.CliConfirmTypeToConfirm(words,
		messages.CliConfirmTypeToConfirmParams{Name: name}))

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != name {
		return errors.New(messages.CliConfirmAborted(words))
	}
	return nil
}

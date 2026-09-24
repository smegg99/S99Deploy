// internal/cli/console.go

package cli

import (
	"errors"
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/smegg99/s99term"

	"github.com/smegg99/s99deploy/internal/messages"
)

// newConsole is the one writer, on stderr.
func newConsole(errOut io.Writer, profile colorprofile.Profile, words *i18n.Localizer) *s99term.Console {
	// Every human line, every spinner frame and every log record goes through
	// it; stdout stays for machine-readable output.
	return s99term.New(s99term.Options{
		Out:     errOut,
		Env:     os.Environ(),
		Profile: profile,
		Words:   messages.StatusWords(words),
	})
}

// profileFor turns --color into the profile the console downgrades to.
func profileFor(choice string, words *i18n.Localizer) (colorprofile.Profile, error) {
	// auto detects, which is what honours NO_COLOR, CLICOLOR_FORCE and TERM=dumb.
	switch choice {
	case "", "auto":
		return colorprofile.Unknown, nil
	case "always":
		return colorprofile.TrueColor, nil
	case "never":
		return colorprofile.NoTTY, nil
	default:
		return colorprofile.Unknown, errors.New(messages.CliCommandInvalidColor(
			words, messages.CliCommandInvalidColorParams{Value: choice}))
	}
}

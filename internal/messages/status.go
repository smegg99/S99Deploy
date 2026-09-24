// internal/messages/status.go

package messages

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/smegg99/s99term"
)

// StatusWords is the catalog's name for each render-layer state.
func StatusWords(words *i18n.Localizer) s99term.Words {
	// The render layer holds no human sentence of its own, and both binaries
	// read these from here.
	return s99term.Words{
		s99term.StatusOK:      CliStatusOk(words),
		s99term.StatusFailed:  CliStatusFailed(words),
		s99term.StatusSkipped: CliStatusSkipped(words),
		s99term.StatusWaiting: CliStatusWaiting(words),
		s99term.StatusRunning: CliStatusRunning(words),
		s99term.StatusWarning: CliStatusWarning(words),
		s99term.StatusStale:   CliStatusStale(words),
	}
}

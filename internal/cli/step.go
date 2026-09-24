// internal/cli/step.go

package cli

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/smegg99/s99term"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/messages"
)

// Stepper turns the domain's stage boundaries into lines on the console.
type Stepper struct {
	console *s99term.Console
	words   *i18n.Localizer
}

// NewStepper returns a deploy.Progress over console.
func NewStepper(console *s99term.Console, words *i18n.Localizer) *Stepper {
	return &Stepper{console: console, words: words}
}

// Begin announces a stage and returns the function that closes it.
func (s *Stepper) Begin(step deploy.Step, detail string) func(error) {
	// Only the health wait animates: every other stage streams a child process
	// on this same stream, and a frame redrawn under it would tear.
	label := s.label(step, detail)
	if step != deploy.StepHealth {
		s.console.Line(s99term.StatusRunning, label)
		return func(err error) { s.console.Line(statusFor(err), label) }
	}

	live := s.console.Begin(label)
	return func(err error) { live.End(statusFor(err)) }
}

// label is the stage's sentence plus the one detail that identifies it.
func (s *Stepper) label(step deploy.Step, detail string) string {
	word := s.word(step)
	if detail == "" {
		return word
	}
	return word + " " + detail
}

func (s *Stepper) word(step deploy.Step) string {
	switch step {
	case deploy.StepClone:
		return messages.LogsCloning(s.words)
	case deploy.StepPull:
		return messages.LogsPulling(s.words)
	case deploy.StepBuild:
		return messages.LogsBuilding(s.words)
	case deploy.StepRestart:
		return messages.LogsRestarting(s.words)
	case deploy.StepHealth:
		return messages.LogsHealthCheck(s.words)
	default:
		return string(step)
	}
}

// statusFor maps a stage's outcome onto the shared state vocabulary.
func statusFor(err error) s99term.Status {
	if err != nil {
		return s99term.StatusFailed
	}
	return s99term.StatusOK
}

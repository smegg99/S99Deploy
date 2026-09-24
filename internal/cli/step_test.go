// internal/cli/step_test.go

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/smegg99/s99term"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/messages"
)

// testStepper writes to a buffer with colour and animation both off.
func testStepper(t *testing.T) (*Stepper, *bytes.Buffer) {
	t.Helper()
	// That is what a pipe and a CI log get. NoTTY and not ASCII: ASCII strips
	// colour but leaves bare resets, and this test asserts on the exact bytes.
	out := &bytes.Buffer{}
	off := false
	words := messages.Localizer("en")
	console := s99term.New(s99term.Options{
		Out: out, Env: []string{"LANG=C"}, Profile: colorprofile.NoTTY, Animate: &off,
		Words: messages.StatusWords(words),
	})
	t.Cleanup(console.Close)
	return NewStepper(console, words), out
}

// A stage that streams a child's output gets a start line and a finished line.
func TestStepperPrintsOneLinePerStageBoundary(t *testing.T) {
	// And no control bytes at all on a stream that cannot animate.
	stepper, out := testStepper(t)

	end := stepper.Begin(deploy.StepBuild, "go build ./...")
	end(nil)

	got := out.String()
	if strings.Contains(got, "\r") || strings.Contains(got, "\x1b") {
		t.Errorf("a non-animated stage wrote control bytes: %q", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "[*] running") {
		t.Errorf("start line = %q, want it to open with the running state", lines[0])
	}
	if !strings.HasPrefix(lines[1], "[+] ok") {
		t.Errorf("finished line = %q, want it to open with the ok state", lines[1])
	}
	for _, line := range lines {
		if !strings.Contains(line, "go build ./...") {
			t.Errorf("line %q does not name the command", line)
		}
	}
}

func TestStepperReportsAFailedStage(t *testing.T) {
	stepper, out := testStepper(t)

	end := stepper.Begin(deploy.StepPull, "myapp")
	end(errors.New("not a fast-forward"))

	if !strings.Contains(out.String(), "[x] failed") {
		t.Errorf("a failed stage did not say so:\n%s", out)
	}
}

// The state words come from the catalog, so a Polish run reads in Polish.
func TestStatusWordsComeFromTheCatalog(t *testing.T) {
	words := messages.StatusWords(messages.Localizer("pl"))

	if got := words[s99term.StatusFailed]; got != "błąd" {
		t.Errorf("the Polish word for failed = %q", got)
	}
}

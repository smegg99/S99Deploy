// internal/cli/binary_test.go

package cli_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smegg99/s99deploy/internal/messages"
)

// The table again, through a real process.
func TestExitCodesFromTheCompiledBinary(t *testing.T) {
	// An exit code is a property of the process, not of a function returning an int.
	if testing.Short() {
		t.Skip("builds a binary")
	}
	binary := filepath.Join(t.TempDir(), "s99deploy")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/s99deploy")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	for _, c := range []struct {
		name string
		args []string
		code int
		says string
	}{
		{name: "help", args: []string{"--help"}, code: 0, says: "Usage:"},
		{name: "version", args: []string{"--version"}, code: 0, says: "s99deploy 1.0.0"},
		{name: "unknown flag", args: []string{"--nope"}, code: 2, says: "unknown flag"},
		{name: "unknown command", args: []string{"bogus"}, code: 2, says: "unknown command"},
		{name: "missing argument", args: []string{"up"}, code: 2, says: "accepts 1 arg(s)"},
		{name: "not root", args: []string{"up", "nothing-by-this-name"}, code: 1, says: "must run as root"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(binary, c.args...)
			cmd.Env = append(os.Environ(), "S99DEPLOY_LANG="+messages.DefaultLocale)
			out, err := cmd.CombinedOutput()

			code := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else if err != nil {
				t.Fatal(err)
			}
			if code != c.code {
				t.Errorf("code = %d, want %d\n%s", code, c.code, out)
			}
			if !strings.Contains(string(out), c.says) {
				t.Errorf("output does not carry %q:\n%s", c.says, out)
			}
		})
	}
}

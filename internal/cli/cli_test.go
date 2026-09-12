// internal/cli/cli_test.go

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/deploy/deploytest"
)

// testOptions is the real tree over a faked box.
func testOptions(t *testing.T, in string) (*rootOptions, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cfg, _, _, _, _ := deploytest.NewConfig(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return &rootOptions{
		deployer: deploy.New(cfg),
		in:       strings.NewReader(in),
		out:      stdout,
		errOut:   stderr,
	}, stdout, stderr
}

func TestExitCodeTable(t *testing.T) {
	for _, c := range []struct {
		name   string
		args   []string
		code   int
		stdout string
		stderr string
		usage  bool
	}{
		{name: "help", args: []string{"--help"}, code: 0, stdout: "Usage:"},
		{name: "version", args: []string{"--version"}, code: 0, stdout: "s99deploy 1.0.0"},
		// Not nil: cobra reads os.Args[1:] for a nil argument list, which in a
		// test is the test binary's own flags.
		{name: "no args", args: []string{}, code: 0, stdout: "Usage:"},
		{name: "unknown flag", args: []string{"--nope"}, code: 2,
			stderr: "unknown flag: --nope", usage: true},
		{name: "unknown command", args: []string{"bogus"}, code: 2,
			stderr: `unknown command "bogus"`, usage: true},
		{name: "up with no name", args: []string{"up"}, code: 2,
			stderr: "accepts 1 arg(s), received 0", usage: true},
		{name: "up with two names", args: []string{"up", "a", "b"}, code: 2,
			stderr: "accepts 1 arg(s), received 2", usage: true},
		{name: "up with an unknown flag", args: []string{"up", "--nope", "myapp"}, code: 2,
			stderr: "unknown flag: --nope", usage: true},
		{name: "up on a box with no checkout", args: []string{"up", "myapp"}, code: 1,
			stderr: "s99deploy: no checkout at"},
		{name: "help for a command", args: []string{"help", "up"}, code: 0, stdout: "health check"},
		{name: "completion", args: []string{"completion", "bash"}, code: 0, stdout: "bash completion"},
	} {
		t.Run(c.name, func(t *testing.T) {
			opts, stdout, stderr := testOptions(t, "")

			code := run(context.Background(), opts, c.args)

			if code != c.code {
				t.Errorf("code = %d, want %d\nstdout: %s\nstderr: %s", code, c.code, stdout, stderr)
			}
			if c.stdout != "" && !strings.Contains(stdout.String(), c.stdout) {
				t.Errorf("stdout = %q, want it to carry %q", stdout, c.stdout)
			}
			if c.stderr != "" && !strings.Contains(stderr.String(), c.stderr) {
				t.Errorf("stderr = %q, want it to carry %q", stderr, c.stderr)
			}
			if got := strings.Contains(stderr.String(), "Usage:"); got != c.usage {
				t.Errorf("usage block on stderr = %v, want %v", got, c.usage)
			}
		})
	}
}

// A failed deploy is one line on stderr, with no usage block above it.
func TestFailedWorkIsOneLine(t *testing.T) {
	opts, _, stderr := testOptions(t, "")

	if code := run(context.Background(), opts, []string{"up", "myapp"}); code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if lines := strings.Count(strings.TrimSpace(stderr.String()), "\n"); lines != 0 {
		t.Errorf("stderr has %d extra lines:\n%s", lines, stderr)
	}
}

// --yes skips the prompt, so an unattended purge is possible and testable.
func TestUninstallPromptAndYes(t *testing.T) {
	// The fake box has no unit for myapp, so a run that reached Uninstall would
	// also exit 1. Only the abort line tells the two apart.
	opts, _, stderr := testOptions(t, "wrong-name\n")
	if code := run(context.Background(), opts, []string{"uninstall", "myapp"}); code != 1 {
		t.Error("a mistyped name did not abort")
	}
	if !strings.Contains(stderr.String(), "s99deploy: aborted") {
		t.Errorf("stderr = %q, want the abort line", stderr)
	}

	opts, stdout, _ := testOptions(t, "")
	code := run(context.Background(), opts, []string{"uninstall", "--yes", "myapp"})
	if code != 1 {
		t.Errorf("code = %d, want 1: there is no unit for myapp on a fake box", code)
	}
	if strings.Contains(stdout.String(), "Type") {
		t.Error("--yes still printed the prompt")
	}
}

// A bad name is refused before the prompt, so the warning never names /opt itself.
func TestUninstallRefusesABadNameBeforePrompting(t *testing.T) {
	opts, stdout, stderr := testOptions(t, "")
	if code := run(context.Background(), opts, []string{"uninstall", "--purge", "../x"}); code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if strings.Contains(stdout.String(), "Type") {
		t.Error("the prompt ran for an invalid name")
	}
	if !strings.Contains(stderr.String(), "invalid service name") {
		t.Errorf("stderr = %q, want the name refused", stderr)
	}
}

// An interrupted run says one thing and exits 1.
func TestInterruptedRun(t *testing.T) {
	opts, _, stderr := testOptions(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if code := run(ctx, opts, []string{"up", "myapp"}); code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "interrupted") {
		t.Errorf("stderr = %q, want the interrupted line", stderr)
	}
}

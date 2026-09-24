// internal/cli/cli_test.go

package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/smegg99/s99logger"
	"github.com/smegg99/s99term"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/deploy/deploytest"
	"github.com/smegg99/s99deploy/internal/messages"
)

// testOptions is the real tree over a faked box.
func testOptions(t *testing.T, in string) (*rootOptions, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	return testOptionsOn(t, in, nil)
}

// testOptionsOn is testOptions over a box the caller changes first.
func testOptionsOn(t *testing.T, in string, box func(*deploy.Config)) (*rootOptions, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	// Without this the language comes from the developer's own shell, and every
	// row of the exit-code table that asserts an English label fails on a Polish
	// machine. S99DEPLOY_LANG is first in the precedence for exactly this.
	t.Setenv("S99DEPLOY_LANG", messages.DefaultLocale)

	cfg, _, _, _, _ := deploytest.NewConfig(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	words := messages.Localizer(messages.DefaultLocale)

	off := false
	console := s99term.New(s99term.Options{
		Out: stderr, Env: []string{"LANG=C"}, Profile: colorprofile.NoTTY, Animate: &off,
		Words: messages.StatusWords(words),
	})
	t.Cleanup(console.Close)

	steps := NewStepper(console, words)
	cfg.Progress = steps
	if box != nil {
		box(&cfg)
	}
	return &rootOptions{
		lang:     messages.DefaultLocale,
		color:    "never",
		words:    words,
		console:  console,
		steps:    steps,
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
		{name: "version", args: []string{"--version"}, code: 0, stdout: "depl 1.0.0"},
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
			stderr: "depl: no checkout at"},
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
	if !strings.Contains(stderr.String(), "depl: aborted") {
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

// --verbose is the only thing that lets a debug record reach the console.
func TestVerboseLowersTheLogLevel(t *testing.T) {
	// newRootOptions installs a process-wide default logger over a console this
	// test then closes, so put a harmless one back before leaving.
	t.Cleanup(func() {
		s99logger.SetDefault(s99logger.New(s99logger.NewConsoleSink(io.Discard), s99logger.Options{}))
	})

	for _, verbose := range []bool{false, true} {
		errOut := &bytes.Buffer{}
		opts, err := newRootOptions(deploy.Config{}, messages.DefaultLocale, "never", verbose,
			strings.NewReader(""), &bytes.Buffer{}, errOut)
		if err != nil {
			t.Fatal(err)
		}

		// newRootOptions calls s99logger.SetDefault, which is what makes this
		// observable without threading the logger back out.
		s99logger.Debug(s99logger.NewEvent(messages.KeyLogsBuilding))
		opts.console.Close()

		if got := strings.Contains(errOut.String(), "running a build command"); got != verbose {
			t.Errorf("--verbose=%v: the debug record reached the console = %v, want %v\n%s",
				verbose, got, verbose, errOut)
		}
	}
}

// --color is resolved before the tree exists, so a wrong value is a usage error.
func TestProfileForMapsTheColorFlag(t *testing.T) {
	words := messages.Localizer(messages.DefaultLocale)
	for _, c := range []struct {
		name    string
		choice  string
		want    colorprofile.Profile
		wantErr string
	}{
		{name: "unset", choice: "", want: colorprofile.Unknown},
		{name: "auto", choice: "auto", want: colorprofile.Unknown},
		{name: "always", choice: "always", want: colorprofile.TrueColor},
		{name: "never", choice: "never", want: colorprofile.NoTTY},
		{name: "bogus", choice: "bogus", want: colorprofile.Unknown, wantErr: "--color bogus"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := profileFor(c.choice, words)

			if got != c.want {
				t.Errorf("profileFor(%q) = %v, want %v", c.choice, got, c.want)
			}
			switch {
			case c.wantErr == "" && err != nil:
				t.Errorf("profileFor(%q) returned %v, want no error", c.choice, err)
			case c.wantErr != "" && err == nil:
				t.Errorf("profileFor(%q) returned no error, want one naming the value", c.choice)
			case c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr):
				t.Errorf("error = %q, want it to carry %q", err, c.wantErr)
			}
		})
	}
}

// The resolved profile, not the environment, decides whether escapes are written.
func TestConsoleHonoursTheResolvedProfile(t *testing.T) {
	// newConsole hands s99term os.Environ(), so this also proves an explicit
	// profile beats detection: a developer with NO_COLOR set in their shell
	// still gets escapes out of --color always.
	words := messages.Localizer(messages.DefaultLocale)
	for _, c := range []struct {
		name    string
		profile colorprofile.Profile
		want    bool
	}{
		{name: "always", profile: colorprofile.TrueColor, want: true},
		{name: "never", profile: colorprofile.NoTTY},
	} {
		t.Run(c.name, func(t *testing.T) {
			errOut := &bytes.Buffer{}
			console := newConsole(errOut, c.profile, words)
			console.Line(s99term.StatusOK, "deployed myapp")
			console.Close()

			got := strings.Contains(errOut.String(), "\x1b[")
			if got != c.want {
				t.Errorf("an escape reached the stream = %v, want %v: %q", got, c.want, errOut)
			}
			if !strings.Contains(errOut.String(), "deployed myapp") {
				t.Errorf("the line lost its label: %q", errOut)
			}
		})
	}
}

// internal/exit/exit_test.go

package exit_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/internal/exit"
)

// tree is a root with one subcommand that fails.
func tree(fail error) *cobra.Command {
	// That is every shape the classification has to tell apart.
	root := &cobra.Command{
		Use: "tool", Args: cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.AddCommand(&cobra.Command{
		Use: "work <name>", Args: cobra.ExactArgs(1),
		RunE: exit.Work(func(*cobra.Command, []string) error { return fail }),
	})
	return root
}

func TestClassification(t *testing.T) {
	for _, c := range []struct {
		name  string
		args  []string
		fail  error
		code  int
		says  string
		usage bool
	}{
		{name: "success", args: []string{"work", "x"}, code: exit.OK},
		{name: "work failed", args: []string{"work", "x"}, fail: errors.New("build failed"),
			code: exit.Error, says: "tool: build failed"},
		{name: "unknown flag", args: []string{"--nope"}, code: exit.Usage,
			says: "unknown flag: --nope", usage: true},
		{name: "unknown command", args: []string{"bogus"}, code: exit.Usage,
			says: `unknown command "bogus"`, usage: true},
		{name: "wrong arg count", args: []string{"work"}, code: exit.Usage,
			says: "accepts 1 arg(s), received 0", usage: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			root := tree(c.fail)
			root.SetOut(&bytes.Buffer{})
			root.SetErr(stderr)
			root.SetArgs(c.args)

			code := exit.Run(context.Background(), root, stderr,
				exit.Texts{Name: "tool", Interrupted: "tool: interrupted"})

			if code != c.code {
				t.Errorf("code = %d, want %d (stderr: %s)", code, c.code, stderr)
			}
			if c.says != "" && !strings.Contains(stderr.String(), c.says) {
				t.Errorf("stderr = %q, want it to carry %q", stderr, c.says)
			}
			if got := strings.Contains(stderr.String(), "Usage:"); got != c.usage {
				t.Errorf("usage block present = %v, want %v", got, c.usage)
			}
		})
	}
}

// An interrupted run says so once, and does not print a usage block for it.
func TestCancelledRunIsInterruptedAndNotUsage(t *testing.T) {
	stderr := &bytes.Buffer{}
	root := tree(context.Canceled)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(stderr)
	root.SetArgs([]string{"work", "x"})

	code := exit.Run(context.Background(), root, stderr,
		exit.Texts{Name: "tool", Interrupted: "tool: interrupted"})

	if code != exit.Error {
		t.Errorf("code = %d, want %d", code, exit.Error)
	}
	if got := stderr.String(); got != "tool: interrupted\n" {
		t.Errorf("stderr = %q, want exactly the interrupted line", got)
	}
}

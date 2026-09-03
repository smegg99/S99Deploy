// internal/deploy/exec.go

package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// liveRunner starts real processes on this process's own descriptors.
type liveRunner struct{ out, errOut io.Writer }

func (r *liveRunner) Run(ctx context.Context, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout, cmd.Stderr = r.out, r.errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func (r *liveRunner) Output(ctx context.Context, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *liveRunner) RunAsUser(ctx context.Context, a AsUser, command string, env []string) error {
	cmd := exec.CommandContext(ctx, "runuser", "-u", a.Name, "--", "bash", "-lc", command)
	cmd.Dir = a.Dir
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = r.out, r.errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("as %s: %s: %w", a.Name, command, err)
	}
	return nil
}

func (r *liveRunner) OutputAsUser(ctx context.Context, a AsUser, command string, env []string) (string, error) {
	cmd := exec.CommandContext(ctx, "runuser", "-u", a.Name, "--", "bash", "-lc", command)
	cmd.Dir = a.Dir
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("as %s: %s: %w", a.Name, command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveArgv0 turns the manifest run argv[0] into an executable path.
func resolveArgv0(appDir, argv0 string) (string, error) {
	// Absolute stays, a path with a separator is relative to the app dir, and a bare name goes through PATH.
	if filepath.IsAbs(argv0) {
		return argv0, nil
	}
	if strings.Contains(argv0, "/") {
		return filepath.Join(appDir, argv0), nil
	}
	return exec.LookPath(argv0)
}

// internal/deploy/exec.go

package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// liveRunner starts real processes on this process's own descriptors.
type liveRunner struct{ out, errOut io.Writer }

// killGrace is how long a build group gets to leave after SIGTERM.
const killGrace = 10 * time.Second

func (r *liveRunner) Run(ctx context.Context, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout, cmd.Stderr = r.out, r.errOut
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), cause(ctx, err))
	}
	return nil
}

func (r *liveRunner) Output(ctx context.Context, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), cause(ctx, err))
	}
	return strings.TrimSpace(string(out)), nil
}

// RunAsUser runs one build command as the service account, in its own group.
func (r *liveRunner) RunAsUser(ctx context.Context, a AsUser, command string, env []string) error {
	cmd := r.asUser(ctx, a, command, env)
	cmd.Stdout, cmd.Stderr = r.out, r.errOut
	disarm := groupCancel(cmd, killGrace)
	err := cmd.Run()
	disarm()

	if err != nil {
		return fmt.Errorf("as %s: %s: %w", a.Name, command, cause(ctx, err))
	}
	return nil
}

func (r *liveRunner) OutputAsUser(ctx context.Context, a AsUser, command string, env []string) (string, error) {
	cmd := r.asUser(ctx, a, command, env)
	cmd.Stderr = r.errOut
	disarm := groupCancel(cmd, killGrace)
	out, err := cmd.Output()
	disarm()

	if err != nil {
		return "", fmt.Errorf("as %s: %s: %w", a.Name, command, cause(ctx, err))
	}
	return strings.TrimSpace(string(out)), nil
}

// asUser builds the login shell every build command runs in.
func (r *liveRunner) asUser(ctx context.Context, a AsUser, command string, env []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = a.Dir
	cmd.Env = asUserEnv(a, env)
	// Stdin is nil: a build must not read the terminal. In the background group
	// groupCancel puts this in, a terminal read would be stopped by SIGTTIN anyway.
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: a.UID, Gid: a.GID, Groups: a.Groups},
	}
	return cmd
}

// asUserEnv gives the account its own HOME, USER, LOGNAME and SHELL.
func asUserEnv(a AsUser, base []string) []string {
	// runuser used to set all four, and git needs them to find .gitconfig and
	// .ssh. os/exec keeps the last value for a duplicated key, so these win.
	if base == nil {
		base = os.Environ()
	}
	return append(append([]string{}, base...),
		"HOME="+a.Home, "USER="+a.Name, "LOGNAME="+a.Name, "SHELL=/bin/bash",
		"GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
}

// cause reports the cancellation when there was one.
func cause(ctx context.Context, err error) error {
	// Cmd.Wait prefers the process's own error, so a child killed by the cancel
	// returns "signal: terminated" and the caller can no longer tell an
	// interrupt from a failure.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// groupCancel makes a cancelled context reach every process in cmd's group.
func groupCancel(cmd *exec.Cmd, grace time.Duration) (disarm func()) {
	var mu sync.Mutex
	var cancelled bool

	// The kill below names a group, so arming is what creates one. Leaving this
	// to the caller is how kill(-pid) ends up naming a group nobody is in.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true

	// os/exec signals the direct child only, so the group gets its own SIGTERM.
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				// The group is already gone, which os/exec treats as no error at
				// all rather than as a failed interrupt.
				return os.ErrProcessDone
			}
			return err
		}
		mu.Lock()
		cancelled = true
		mu.Unlock()
		return nil
	}
	// WaitDelay force-kills the direct child if it outlives the cancel; the rest
	// of the group is disarm's job below.
	cmd.WaitDelay = grace + time.Second

	// disarm runs after Wait. The leader has exited, but a grandchild that
	// ignored SIGTERM outlives it, so a cancelled run waits out the grace and
	// then SIGKILLs whatever is left of the group.
	return func() {
		mu.Lock()
		fired := cancelled
		mu.Unlock()
		if !fired {
			return
		}
		pgid := cmd.Process.Pid
		deadline := time.Now().Add(grace)
		for time.Now().Before(deadline) {
			if err := syscall.Kill(-pgid, 0); errors.Is(err, syscall.ESRCH) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
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

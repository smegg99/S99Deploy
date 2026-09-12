// internal/deploy/exec_test.go

package deploy

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestResolveArgv0(t *testing.T) {
	if got, _ := resolveArgv0("/opt/x/app", "/usr/bin/env"); got != "/usr/bin/env" {
		t.Errorf("absolute: %q", got)
	}
	if got, _ := resolveArgv0("/opt/x/app", "bin/server"); got != "/opt/x/app/bin/server" {
		t.Errorf("relative: %q", got)
	}
	got, err := resolveArgv0("/opt/x/app", "sh")
	if err != nil || !strings.HasSuffix(got, "/sh") {
		t.Errorf("bare name via PATH: %q, %v", got, err)
	}
}

// git resolves .gitconfig and .ssh from HOME.
func TestAsUserEnvCarriesTheAccountHome(t *testing.T) {
	// A private-repo pull that runs with root's HOME looks in /root/.ssh as an
	// unprivileged uid and fails. os/exec keeps the last value for a duplicated
	// key, so appending is what decides.
	env := asUserEnv(
		AsUser{Name: "myapp", Home: "/opt/myapp", Dir: "/opt/myapp/app"},
		[]string{"HOME=/root", "PATH=/usr/bin"},
	)

	for key, want := range map[string]string{
		"HOME": "/opt/myapp", "USER": "myapp", "LOGNAME": "myapp",
		"SHELL": "/bin/bash", "PATH": "/usr/bin",
		"GIT_TERMINAL_PROMPT": "0", "GIT_SSH_COMMAND": "ssh -o BatchMode=yes",
	} {
		if got := lastValue(env, key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

// lastValue is what os/exec's dedup does: the last entry for a key wins.
func lastValue(env []string, key string) string {
	value := ""
	for _, entry := range env {
		if name, found, ok := strings.Cut(entry, "="); ok && name == key {
			value = found
		}
	}
	return value
}

// A cancelled build must reach the caller as context.Canceled.
func TestCancelledProcessReportsContextCanceled(t *testing.T) {
	// Cmd.Wait prefers the process error, so the signal that killed the child
	// would otherwise be all the caller ever sees.
	runner := &liveRunner{out: io.Discard, errOut: io.Discard}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx, nil, "sleep", "60") }()
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v, want it to wrap context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

// The reason for Setpgid, asserted rather than assumed.
func TestGroupCancelReachesAGrandchild(t *testing.T) {
	// os/exec signals the direct child only, so a compiler started by the shell
	// would survive the cancel and race the next run. Nothing here sets Setpgid:
	// arming the cancel must be what creates the group it kills.
	pidFile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc", "sleep 60 & echo $! > "+pidFile+"; wait")
	disarm := groupCancel(cmd, time.Second)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("arming the cancel did not put the command in its own group")
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	pid := waitForPID(t, pidFile)
	cancel()
	_ = cmd.Wait()
	disarm()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("grandchild %d survived the cancel", pid)
}

// A grandchild that ignores SIGTERM is still killed once the grace runs out.
func TestGroupCancelEscalatesToSIGKILL(t *testing.T) {
	// The subshell ignores TERM and execs sleep, which keeps the ignore across
	// execve, so only the SIGKILL from disarm ends it.
	pidFile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc",
		"( trap '' TERM; exec sleep 60 ) & echo $! > "+pidFile+"; wait")
	disarm := groupCancel(cmd, 300*time.Millisecond)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	pid := waitForPID(t, pidFile)
	cancel()
	_ = cmd.Wait()
	disarm()

	// disarm has sent SIGKILL by the time it returns; init then reaps the zombie.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the TERM-ignoring grandchild %d survived the cancel", pid)
}

// waitForPID reads the pid the shell wrote, once it has written it.
func waitForPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(path)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(body))); err == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never carried a pid", path)
	return 0
}

// requireAgent fires for an SSH URL and for nothing else.
func TestRequireAgentFiresOnlyForSSH(t *testing.T) {
	// sudo's env_reset drops SSH_AUTH_SOCK, so the documented command arrives
	// with no agent and the clone would hang on a prompt nothing can answer.
	t.Setenv("SSH_AUTH_SOCK", "")
	for _, c := range []struct {
		url  string
		want bool
	}{
		{"git@example.com:myapp.git", true},
		{"ssh://git@github.com/smegg99/MyApp.git", true},
		{"https://github.com/smegg99/MyApp.git", false},
		{"/srv/mirrors/myapp.git", false},
	} {
		err := requireAgent(c.url)
		if (err != nil) != c.want {
			t.Errorf("requireAgent(%q) = %v, want an error: %v", c.url, err, c.want)
		}
		if c.want && err != nil && !strings.Contains(err.Error(), "--preserve-env=SSH_AUTH_SOCK") {
			t.Errorf("requireAgent(%q) does not say how to fix it: %v", c.url, err)
		}
	}
}

func TestRequireAgentAcceptsARealSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Setenv("SSH_AUTH_SOCK", path)

	if err := requireAgent("git@example.com:myapp.git"); err != nil {
		t.Errorf("a real agent socket was rejected: %v", err)
	}
}

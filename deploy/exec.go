// deploy/exec.go

// Thin wrappers around the external commands the deploy flow shells out to.
// Stdio is inherited so git and build output land in the terminal untouched.
package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// runAsUser runs a shell command as user in dir. A login shell so the service
// user's profile PATH (corepack shims and the like) applies, matching the old
// per-app deploy scripts. A nil env inherits the caller's.
func runAsUser(user, dir, command string, env []string) error {
	cmd := exec.Command("runuser", "-u", user, "--", "bash", "-lc", command)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("as %s: %s: %w", user, command, err)
	}
	return nil
}

func outputAsUser(user, dir, command string) (string, error) {
	cmd := exec.Command("runuser", "-u", user, "--", "bash", "-lc", command)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("as %s: %s: %w", user, command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func requireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root: sudo s99deploy ...")
	}
	return nil
}

// resolveArgv0 turns the manifest run argv[0] into an executable path:
// absolute stays, a path with a separator is relative to the app dir, a bare
// name goes through PATH.
func resolveArgv0(appDir, argv0 string) (string, error) {
	if filepath.IsAbs(argv0) {
		return argv0, nil
	}
	if strings.Contains(argv0, "/") {
		return filepath.Join(appDir, argv0), nil
	}
	return exec.LookPath(argv0)
}

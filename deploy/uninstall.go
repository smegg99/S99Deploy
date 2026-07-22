// deploy/uninstall.go

package deploy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/smegg99/s99logger"
)

// Uninstall removes the unit after confirmation. The checkout, .env, and
// service user survive unless purge is set.
func Uninstall(name string, purge bool) error {
	if err := requireRoot(); err != nil {
		return err
	}
	unitPath := filepath.Join("/etc/systemd/system", name+".service")
	if _, err := os.Stat(unitPath); err != nil {
		return fmt.Errorf("no unit at %s -- is %s installed?", unitPath, name)
	}

	fmt.Printf("This disables and removes the %s service.\n", name)
	if purge {
		fmt.Printf("--purge also deletes /opt/%s (checkout, .env, logs) and the %s user.\n", name, name)
	}
	fmt.Printf("Type %q to confirm: ", name)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() || scanner.Text() != name {
		return fmt.Errorf("aborted")
	}

	if err := run("systemctl", "disable", "--now", name); err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil {
		return err
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if purge {
		if err := run("userdel", "-r", name); err != nil {
			return err
		}
		s99logger.Info(s99logger.NewEvent("purged", s99logger.String("app", name)))
		return nil
	}
	s99logger.Info(s99logger.NewEvent("uninstalled", s99logger.String("kept", filepath.Join("/opt", name))))
	return nil
}

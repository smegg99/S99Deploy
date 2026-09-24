// internal/deploy/selfpath.go

package deploy

import (
	"fmt"
	"path/filepath"
	"strings"
)

// protectedHomes are the trees the unit's own sandbox empties for the service:
// ProtectHome=true for the first three, PrivateTmp=true for the last two. A
// binary that lives in one of them is simply not there when systemd execs the
// unit, and the only thing the journal reports is 203/EXEC.
var protectedHomes = []string{"/home", "/root", "/run/user", "/tmp", "/var/tmp"}

// SelfPathError is a SelfPath the generated unit could never exec. Blocked is
// the ProtectHome tree that hides it, and is empty when the path is not absolute.
type SelfPathError struct{ Path, Blocked string }

// Error is what a log or a test reads; the command line prints the catalog
// sentence for the same error, which is the one an operator sees.
func (e *SelfPathError) Error() string {
	if e.Blocked == "" {
		return fmt.Sprintf("unit ExecStart %q is not an absolute path", e.Path)
	}
	return fmt.Sprintf("unit ExecStart %q is under %s, which ProtectHome=true hides from the service", e.Path, e.Blocked)
}

// checkSelfPath refuses a binary path the unit's own sandbox puts out of reach.
func checkSelfPath(path string) error {
	// Compared by path component: /homebrew and /rootfs are neither /home nor
	// /root, and a string prefix would refuse both.
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return &SelfPathError{Path: path}
	}
	for _, home := range protectedHomes {
		if clean == home || strings.HasPrefix(clean, home+"/") {
			return &SelfPathError{Path: path, Blocked: home}
		}
	}
	return nil
}

// version.go

// Package s99deploy holds the version; the tools are under cmd/.
package s99deploy

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

// versionFile is embedded, not copied into a constant, so there is one place to update.
//
//go:embed VERSION
var versionFile string

// Declared returns the version written in VERSION.
func Declared() string { return strings.TrimSpace(versionFile) }

// Version is the declared version, plus the commit it was built from.
func Version() string {
	rev, dirty, ok := revision()
	if !ok {
		return Declared()
	}
	return format(Declared(), rev, dirty)
}

// format is separate from Version because a test binary carries no VCS stamps.
func format(declared, rev string, dirty bool) string {
	if rev == "" {
		return declared
	}
	if dirty {
		return declared + " (" + rev + ", dirty)"
	}
	return declared + " (" + rev + ")"
}

func revision() (rev string, dirty, ok bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false, false
	}

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if len(rev) < 7 {
		return "", false, false
	}
	return rev[:7], dirty, true
}

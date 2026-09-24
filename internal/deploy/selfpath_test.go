// internal/deploy/selfpath_test.go

package deploy

import (
	"errors"
	"testing"
)

func TestCheckSelfPath(t *testing.T) {
	for _, c := range []struct {
		path    string
		blocked string
		refused bool
	}{
		{path: "/usr/local/bin/depl"},
		{path: "/opt/depl/bin/depl"},
		// Neither of these is the tree ProtectHome hides, so a string prefix
		// would refuse an install that works.
		{path: "/homebrew/bin/depl"},
		{path: "/rootfs/bin/depl"},
		{path: "/home/dev/S99Deploy/bin/depl", blocked: "/home", refused: true},
		{path: "/root/S99Deploy/bin/depl", blocked: "/root", refused: true},
		{path: "/run/user/1000/depl", blocked: "/run/user", refused: true},
		// PrivateTmp=true gives the service its own empty /tmp and /var/tmp, so
		// a binary left in either is hidden the same way ProtectHome hides /home.
		{path: "/tmp/depl", blocked: "/tmp", refused: true},
		{path: "/var/tmp/depl", blocked: "/var/tmp", refused: true},
		{path: "/tmpfiles/bin/depl"},
		// A relative ExecStart is not a path systemd resolves at all.
		{path: "bin/depl", refused: true},
		{path: "", refused: true},
	} {
		t.Run(c.path, func(t *testing.T) {
			err := checkSelfPath(c.path)
			if !c.refused {
				if err != nil {
					t.Fatalf("checkSelfPath(%q) = %v, want nil", c.path, err)
				}
				return
			}

			var bad *SelfPathError
			if !errors.As(err, &bad) {
				t.Fatalf("checkSelfPath(%q) = %v, want a *SelfPathError", c.path, err)
			}
			if bad.Path != c.path {
				t.Errorf("Path = %q, want %q", bad.Path, c.path)
			}
			if bad.Blocked != c.blocked {
				t.Errorf("Blocked = %q, want %q", bad.Blocked, c.blocked)
			}
		})
	}
}

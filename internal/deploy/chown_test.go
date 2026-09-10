// internal/deploy/chown_test.go

package deploy

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// altGID is a group this process may chown to that is not the one it has.
// deploytest carries the same helper for the packages that can import it.
func altGID(t *testing.T) int {
	t.Helper()
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatal(err)
	}
	for _, gid := range groups {
		if gid != os.Getgid() {
			return gid
		}
	}
	t.Skip("this account is in no group but its own, so a chown cannot be observed")
	return 0
}

// A symlink in a fresh clone must not redirect the walk.
func TestChownTreeChangesLinksAndNotTargets(t *testing.T) {
	// os.Chown resolves the link and would fail on a dangling one; os.Lchown
	// changes the link itself.
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "app", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "app", "link")); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(root, "app", "dangling")
	if err := os.Symlink(filepath.Join(root, "never-created"), dangling); err != nil {
		t.Fatal(err)
	}

	// The difference the fix is about, asserted rather than assumed.
	if err := os.Chown(dangling, os.Getuid(), os.Getgid()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("os.Chown on a dangling symlink = %v, want a not-exist error", err)
	}
	// A group this process may chown to but is not already in, so a walk that
	// set no ownership at all is visible.
	gid := altGID(t)
	if err := chownTree(root, os.Getuid(), gid); err != nil {
		t.Fatalf("chownTree: %v", err)
	}

	for _, path := range []string{root, filepath.Join(root, "app"),
		filepath.Join(root, "app", "sub"), filepath.Join(root, "app", "file"),
		filepath.Join(root, "app", "link"), dangling} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if stat := info.Sys().(*syscall.Stat_t); int(stat.Gid) != gid {
			t.Errorf("%s is in group %d, want %d", path, stat.Gid, gid)
		}
	}

	if _, err := os.Stat(outside); err != nil {
		t.Errorf("the file the link pointed at was disturbed: %v", err)
	}
	// The link changed; what it points at did not.
	target, err := os.Lstat(outside)
	if err != nil {
		t.Fatal(err)
	}
	if stat := target.Sys().(*syscall.Stat_t); int(stat.Gid) == gid {
		t.Errorf("the walk followed the link and chowned %s", outside)
	}
}

func TestChownTreeNamesTheFailingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	err := chownTree(missing, os.Getuid(), os.Getgid())
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err = %v, want it to name %s", err, missing)
	}
}

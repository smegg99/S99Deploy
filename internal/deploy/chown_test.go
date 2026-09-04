// internal/deploy/chown_test.go

package deploy

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	if err := chownTree(root, os.Getuid(), os.Getgid()); err != nil {
		t.Fatalf("chownTree: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("the file the link pointed at was disturbed: %v", err)
	}
}

func TestChownTreeNamesTheFailingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")

	err := chownTree(missing, os.Getuid(), os.Getgid())
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err = %v, want it to name %s", err, missing)
	}
}

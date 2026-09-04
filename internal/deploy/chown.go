// internal/deploy/chown.go

package deploy

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// chownTree sets ownership on root and everything under it, following nothing.
func chownTree(root string, uid, gid int) error {
	// WalkDir does not descend into symlinks and Lchown changes the link itself.
	return filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := os.Lchown(path, uid, gid); err != nil {
			return fmt.Errorf("chown %s: %w", path, err)
		}
		return nil
	})
}

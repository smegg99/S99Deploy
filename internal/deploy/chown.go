// internal/deploy/chown.go

package deploy

import (
	"fmt"
	"os"
	"path/filepath"
)

// chownTree sets ownership on root and everything under it, following nothing.
func chownTree(root string, uid, gid int) error {
	// The account owns root and can swap a directory under it for a link, but
	// root's own parent (/opt) is root-owned, so root cannot be swapped. Opening
	// the parent as an os.Root and walking root through it means every step is
	// re-resolved with no escape and no symlink is followed at root itself.
	parent := filepath.Dir(root)
	dir, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer dir.Close()
	return chownWithin(dir, parent, filepath.Base(root), uid, gid)
}

// chownWithin chowns name (relative to dir) and, when name is a real directory, its entries.
func chownWithin(dir *os.Root, parent, name string, uid, gid int) error {
	if err := dir.Lchown(name, uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", filepath.Join(parent, name), err)
	}
	// Lstat, so a symlinked directory is chowned but not descended into.
	info, err := dir.Lstat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	handle, err := dir.Open(name)
	if err != nil {
		return err
	}
	entries, err := handle.ReadDir(-1)
	handle.Close()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := chownWithin(dir, parent, filepath.Join(name, entry.Name()), uid, gid); err != nil {
			return err
		}
	}
	return nil
}

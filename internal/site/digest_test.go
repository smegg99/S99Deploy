// internal/site/digest_test.go

package site

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The digest follows the file.
func TestDigestFollowsTheFileOnDisk(t *testing.T) {
	// A build that replaced the binary and then failed its second build command
	// must not leave the site quoting the old checksum.
	path := filepath.Join(t.TempDir(), "depl")
	if err := os.WriteFile(path, []byte("first"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := &binDigest{path: path}

	first, size, err := bin.get()
	if err != nil {
		t.Fatal(err)
	}
	if want := sum(t, "first"); first != want || size != 5 {
		t.Fatalf("digest = %s (%d bytes), want %s (5 bytes)", first, size, want)
	}

	if err := os.WriteFile(path, []byte("second build"), 0o755); err != nil {
		t.Fatal(err)
	}
	second, size, err := bin.get()
	if err != nil {
		t.Fatal(err)
	}
	if want := sum(t, "second build"); second != want || size != 12 {
		t.Errorf("digest = %s (%d bytes), want %s (12 bytes)", second, size, want)
	}

	// Same length as the write before it, with the mtime forced distinct so the
	// key changes even if the two writes shared a coarse filesystem tick: a
	// size-only key would keep serving the checksum above.
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("second BUILD"), 0o755); err != nil {
		t.Fatal(err)
	}
	distinct := before.ModTime().Add(time.Hour)
	if err := os.Chtimes(path, distinct, distinct); err != nil {
		t.Fatal(err)
	}
	third, size, err := bin.get()
	if err != nil {
		t.Fatal(err)
	}
	if want := sum(t, "second BUILD"); third != want || size != 12 {
		t.Errorf("digest = %s (%d bytes), want %s (12 bytes)", third, size, want)
	}
}

func TestDigestFailsForAMissingFile(t *testing.T) {
	bin := &binDigest{path: filepath.Join(t.TempDir(), "gone")}

	if _, _, err := bin.get(); err == nil {
		t.Fatal("a missing binary produced a digest")
	}
}

func sum(t *testing.T, body string) string {
	t.Helper()
	hash := sha256.Sum256([]byte(body))
	return hex.EncodeToString(hash[:])
}

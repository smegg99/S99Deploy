// internal/site/digest.go

package site

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sync"
	"time"
)

// binDigest is the sha256 of the served file as it is on disk right now.
type binDigest struct {
	path string

	mu    sync.Mutex
	size  int64
	mtime time.Time
	sum   string
}

// get returns the digest and size of the file, rehashing it when it changed.
func (b *binDigest) get() (string, int64, error) {
	info, err := os.Stat(b.path)
	if err != nil {
		return "", 0, err
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.sum != "" && info.Size() == b.size && info.ModTime().Equal(b.mtime) {
		return b.sum, b.size, nil
	}

	sum, size, err := digestFile(b.path)
	if err != nil {
		return "", 0, err
	}
	b.sum, b.size, b.mtime = sum, size, info.ModTime()
	return sum, size, nil
}

// digestFile hashes the whole file.
func digestFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

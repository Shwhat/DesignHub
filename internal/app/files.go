package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// FileStore keeps attachment content addressed by SHA-256, so a file used by
// several revisions is stored once. Git records reference files by checksum.
type FileStore struct {
	dir string
}

var shaRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// NewFileStore opens or creates the store.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(filepath.Join(dir, "tmp"), 0o755); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir}, nil
}

// Dir returns the store folder.
func (s *FileStore) Dir() string { return s.dir }

func (s *FileStore) path(sum string) string {
	return filepath.Join(s.dir, sum[:2], sum)
}

// Put stores the content of r and returns its checksum and size.
func (s *FileStore) Put(r io.Reader, max int64) (string, int64, error) {
	tmp, err := os.CreateTemp(filepath.Join(s.dir, "tmp"), "upload-*")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, max+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", 0, err
	}
	if n > max {
		return "", 0, fmt.Errorf("file is larger than %d MB", max>>20)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	dst := s.path(sum)
	if _, err := os.Stat(dst); err == nil {
		return sum, n, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", 0, err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return "", 0, err
	}
	return sum, n, nil
}

// Open returns the file with the given checksum.
func (s *FileStore) Open(sum string) (*os.File, error) {
	if !shaRe.MatchString(sum) {
		return nil, errors.New("invalid checksum")
	}
	return os.Open(s.path(sum))
}

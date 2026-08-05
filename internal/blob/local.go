// Package blob provides byte-stream persistence for attachments,
// vehicle photos, IMAP raw messages, and any other opaque payloads
// wdw-fleet needs to keep beside the database.
//
// The v1 implementation is a plain local-filesystem store rooted at
// a configured path (WDW_BLOB_ROOT). The interface is deliberately
// narrow so an S3-compatible implementation can drop in later
// without touching handler code.
package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned by [Local.Open] and [Local.Delete] when the
// requested key has no bytes on disk. Callers can compare via
// errors.Is; the underlying error is [os.ErrNotExist].
var ErrNotFound = errors.New("blob: not found")

// Local persists blobs beneath a single root directory on the local
// filesystem. Keys are relative paths (forward slashes) rooted at
// Root; the resolver rejects any key that would escape Root via
// leading slashes, "..", or absolute paths.
type Local struct {
	// Root is the absolute filesystem path under which all keys
	// resolve. Created on first Put if it doesn't exist.
	Root string
}

// New returns a Local rooted at the given directory. The directory
// is not created eagerly; the first Put creates it (and any missing
// parent directories) with mode 0o755.
func New(root string) *Local {
	return &Local{Root: root}
}

// Put writes r's bytes to key, overwriting any prior contents. The
// key's parent directories are created as needed. Callers should
// pass a size-bounded reader (io.LimitReader) if the source is
// untrusted; Put itself imposes no limit.
func (l *Local) Put(ctx context.Context, key string, r io.Reader) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	full, err := l.resolve(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, fmt.Errorf("blob: mkdir: %w", err)
	}
	// Write to a temp file in the same directory, then atomic rename
	// so a concurrent Open never sees a half-written blob.
	tmp, err := os.CreateTemp(filepath.Dir(full), ".wdwblob-*")
	if err != nil {
		return 0, fmt.Errorf("blob: create temp: %w", err)
	}
	tmpName := tmp.Name()
	n, copyErr := io.Copy(tmp, r)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("blob: write: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("blob: close temp: %w", closeErr)
	}
	if err := os.Rename(tmpName, full); err != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("blob: rename: %w", err)
	}
	return n, nil
}

// Open returns a ReadCloser for key. Returns [ErrNotFound] wrapped
// with the underlying [os.ErrNotExist] when the key doesn't exist.
// The caller is responsible for closing the returned stream.
func (l *Local) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	full, err := l.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return nil, fmt.Errorf("blob: open: %w", err)
	}
	return f, nil
}

// Delete removes key. Returns [ErrNotFound] when there was nothing
// to delete; other filesystem errors are wrapped and returned as-is.
func (l *Local) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full, err := l.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotFound, key)
		}
		return fmt.Errorf("blob: delete: %w", err)
	}
	return nil
}

// resolve joins key onto Root and enforces the containment
// invariant: no leading slash, no absolute path, no path segment of
// "..". Returns the cleaned absolute path or an error.
func (l *Local) resolve(key string) (string, error) {
	if key == "" {
		return "", errors.New("blob: empty key")
	}
	if filepath.IsAbs(key) || strings.HasPrefix(key, "/") {
		return "", fmt.Errorf("blob: key must be relative: %s", key)
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == ".." {
			return "", fmt.Errorf("blob: key must not escape root: %s", key)
		}
	}
	full := filepath.Join(l.Root, key)
	// Belt-and-braces after Join, which normalizes .. segments and
	// would let a crafted key that survived the string check above
	// still climb out.
	rel, err := filepath.Rel(l.Root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("blob: key escapes root: %s", key)
	}
	return full, nil
}

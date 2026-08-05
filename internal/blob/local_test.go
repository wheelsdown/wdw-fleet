package blob

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestLocalRoundTrip(t *testing.T) {
	l := New(t.TempDir())
	ctx := context.Background()

	payload := []byte("hello, wdw-fleet")
	n, err := l.Put(ctx, "vehicles/abc.jpg", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if n != int64(len(payload)) {
		t.Errorf("put returned %d bytes, want %d", n, len(payload))
	}

	rc, err := l.Open(ctx, "vehicles/abc.jpg")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("read back %q, want %q", got, payload)
	}

	if err := l.Delete(ctx, "vehicles/abc.jpg"); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := l.Open(ctx, "vehicles/abc.jpg"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete, open err = %v, want ErrNotFound", err)
	}
}

func TestLocalRejectsEscapingKeys(t *testing.T) {
	l := New(t.TempDir())
	ctx := context.Background()
	cases := []string{
		"",
		"/absolute/path",
		"../../etc/passwd",
		"a/../../../etc/passwd",
	}
	for _, k := range cases {
		if _, err := l.Put(ctx, k, bytes.NewReader(nil)); err == nil {
			t.Errorf("Put(%q) should have failed", k)
		}
	}
}

func TestLocalDeleteNotFound(t *testing.T) {
	l := New(t.TempDir())
	err := l.Delete(context.Background(), "nope.bin")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete(missing) err = %v, want ErrNotFound", err)
	}
}

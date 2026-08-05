package api

import (
	"io"
	"log/slog"
)

// newDiscardLogger returns a slog.Logger whose output is thrown away.
// For handler tests that don't want log noise in the go test output.
func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// Package version exposes build-time metadata injected via -ldflags at
// compile time. Values default to "dev"/"unknown" for `go build` without
// flags; the justfile and CI pipeline override them with real values.
package version

import "fmt"

// Default values are overridden at build time via:
//
//	-ldflags "-X github.com/wheelsdown/wdw-fleet/internal/version.Version=v0.1.0
//	          -X github.com/wheelsdown/wdw-fleet/internal/version.Commit=abc1234
//	          -X github.com/wheelsdown/wdw-fleet/internal/version.BuildDate=2026-04-21T20:00:00Z"
var (
	// Version is the semver tag (or "dev" for untagged builds).
	Version = "dev"
	// Commit is the short git SHA the binary was built from.
	Commit = "unknown"
	// BuildDate is the RFC 3339 timestamp of the build.
	BuildDate = "unknown"
)

// String returns a human-readable build identifier suitable for startup
// logging or a --version flag.
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildDate)
}

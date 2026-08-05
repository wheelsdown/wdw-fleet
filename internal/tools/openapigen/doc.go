// Command openapigen generates internal/server/api/spec/openapi.{yaml,json}
// from the Go route table returned by [api.Routes].
//
// Invocation: `just generate` (which runs `go generate ./...`, which
// triggers the //go:generate directive on internal/server/api/routes.go).
//
// Contract: the generator will not write anywhere that lacks a marker
// file (internal/server/api/spec/spec.go). This is deliberate --
// preventing a fat-fingered -out from splattering generated YAML
// somewhere unexpected.
//
// Drift is caught in CI by `just generate-check`: it reruns the
// generator and fails on any `git diff` in the spec directory.
package main

// Package spec holds the generated OpenAPI document for wdw-fleet.
//
// openapi.yaml and openapi.json are written here by
// internal/tools/openapigen. They must not be hand-edited: the
// generator overwrites them on every `just generate`, and CI fails
// (see `just generate-check`) if the committed files drift from a
// fresh generation.
//
// This spec.go serves two purposes:
//
//  1. It gives the package a Go presence so `go build ./...` and
//     `go vet ./...` see it -- the .yaml/.json files alone are
//     invisible to those tools.
//  2. Its existence is checked by openapigen before the generator
//     will write anything under this directory. A stray -out flag
//     pointed at the wrong path fails fast rather than splattering
//     generated files across the repo.
package spec

import _ "embed"

// YAML is the generated OpenAPI 3.1 document, embedded so the running
// binary can serve it at /openapi.yaml without shelling out.
//
//go:embed openapi.yaml
var YAML []byte

// JSON is the same document as [YAML], serialized as JSON for tools
// that consume OpenAPI more happily in JSON form.
//
//go:embed openapi.json
var JSON []byte

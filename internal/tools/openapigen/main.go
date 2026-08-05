// See doc.go for the command's purpose.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/wheelsdown/wdw-fleet/internal/server/api"
)

// Root document ------------------------------------------------------

// Document is the subset of the OpenAPI 3.1 root object this generator
// emits. Field order matches OpenAPI convention; both json and yaml
// packages preserve struct declaration order, giving us stable output
// without ordered maps.
type Document struct {
	OpenAPI    string               `json:"openapi" yaml:"openapi"`
	Info       Info                 `json:"info" yaml:"info"`
	Tags       []Tag                `json:"tags,omitempty" yaml:"tags,omitempty"`
	Paths      map[string]*PathItem `json:"paths" yaml:"paths"`
	Components Components           `json:"components,omitempty" yaml:"components,omitempty"`
}

type Info struct {
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string   `json:"version" yaml:"version"`
	License     *License `json:"license,omitempty" yaml:"license,omitempty"`
}

type License struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url,omitempty" yaml:"url,omitempty"`
}

type Tag struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type PathItem struct {
	Get    *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Post   *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put    *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Patch  *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
	Delete *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
}

type Operation struct {
	OperationID string               `json:"operationId" yaml:"operationId"`
	Summary     string               `json:"summary,omitempty" yaml:"summary,omitempty"`
	Tags        []string             `json:"tags,omitempty" yaml:"tags,omitempty"`
	Parameters  []Parameter          `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	RequestBody *RequestBody         `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
	Responses   map[string]*Response `json:"responses" yaml:"responses"`
	// XWdwAuth is emitted as `x-wdw-auth` so the enforced auth
	// posture is visible on the documented operation. Not part of
	// the OpenAPI spec proper; consumers can ignore it.
	XWdwAuth string `json:"x-wdw-auth,omitempty" yaml:"x-wdw-auth,omitempty"`
}

type Parameter struct {
	Name        string  `json:"name" yaml:"name"`
	In          string  `json:"in" yaml:"in"`
	Required    bool    `json:"required,omitempty" yaml:"required,omitempty"`
	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
}

type RequestBody struct {
	Required bool                 `json:"required,omitempty" yaml:"required,omitempty"`
	Content  map[string]MediaType `json:"content" yaml:"content"`
}

type Response struct {
	Description string               `json:"description" yaml:"description"`
	Content     map[string]MediaType `json:"content,omitempty" yaml:"content,omitempty"`
}

type MediaType struct {
	Schema *Schema `json:"schema,omitempty" yaml:"schema,omitempty"`
}

type Components struct {
	Schemas map[string]*Schema `json:"schemas,omitempty" yaml:"schemas,omitempty"`
}

// Runtime --------------------------------------------------------------

// httpStatusText maps status codes to the descriptive strings we
// emit as Response.Description. Extend when new statuses appear in
// the route table; unknown statuses fall back to http.StatusText.
var httpStatusText = map[int]string{
	http.StatusOK:                  "OK",
	http.StatusCreated:             "Created",
	http.StatusAccepted:            "Accepted",
	http.StatusNoContent:           "No content",
	http.StatusBadRequest:          "Bad request",
	http.StatusUnauthorized:        "Unauthorized",
	http.StatusForbidden:           "Forbidden",
	http.StatusNotFound:            "Not found",
	http.StatusConflict:            "Conflict",
	http.StatusServiceUnavailable:  "Service unavailable",
	http.StatusInternalServerError: "Internal server error",
}

func main() {
	out := flag.String("out", "internal/server/api/spec", "directory to write openapi.{yaml,json}")
	flag.Parse()

	if err := run(*out); err != nil {
		log.Fatalf("openapigen: %v", err)
	}
}

func run(out string) error {
	// Marker check: refuse to write anywhere that lacks the spec.go
	// sentinel. Prevents a fat-fingered -out from creating YAML in
	// random directories.
	marker := filepath.Join(out, "spec.go")
	if _, err := os.Stat(marker); err != nil {
		return fmt.Errorf("marker %s not found: %w (refusing to write; is -out correct?)", marker, err)
	}

	routes := api.Routes()
	if err := api.ValidateRoutes(routes); err != nil {
		return fmt.Errorf("route table invalid: %w", err)
	}

	doc, err := buildDocument(routes)
	if err != nil {
		return err
	}

	yamlPath := filepath.Join(out, "openapi.yaml")
	jsonPath := filepath.Join(out, "openapi.json")

	yamlBytes, err := marshalYAML(doc)
	if err != nil {
		return fmt.Errorf("yaml marshal: %w", err)
	}
	if err := os.WriteFile(yamlPath, yamlBytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", yamlPath, err)
	}
	jsonBytes, err := marshalJSON(doc)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonBytes, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", jsonPath, err)
	}

	log.Printf("openapigen: wrote %d paths to %s and %s", len(doc.Paths), yamlPath, jsonPath)
	return nil
}

func buildDocument(routes []api.Route) (*Document, error) {
	doc := &Document{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       "wdw-fleet",
			Description: "API-first vehicle fleet management service (FleetAware).",
			Version:     api.ContractVersion,
			License:     &License{Name: "MIT", URL: "https://opensource.org/licenses/MIT"},
		},
		Paths:      map[string]*PathItem{},
		Components: Components{Schemas: map[string]*Schema{}},
	}

	for _, tag := range api.RouteTags() {
		doc.Tags = append(doc.Tags, Tag{Name: tag.Name, Description: tag.Description})
	}

	// Shared Problem envelope. Every operation gets a `default` response
	// referencing this.
	doc.Components.Schemas["Problem"] = problemSchema()

	for _, rt := range routes {
		if err := addRoute(doc, rt); err != nil {
			return nil, fmt.Errorf("route %s %s: %w", rt.Method, rt.Path, err)
		}
	}

	return doc, nil
}

func addRoute(doc *Document, rt api.Route) error {
	pi, ok := doc.Paths[rt.Path]
	if !ok {
		pi = &PathItem{}
		doc.Paths[rt.Path] = pi
	}

	op := &Operation{
		OperationID: rt.OperationID,
		Summary:     rt.Summary,
		Tags:        rt.Tags,
		Responses:   map[string]*Response{},
		XWdwAuth:    string(rt.Auth),
	}

	for _, param := range api.PathParams(rt.Path) {
		op.Parameters = append(op.Parameters, Parameter{
			Name:     param,
			In:       "path",
			Required: true,
			Schema:   &Schema{Type: "string"},
		})
	}

	if rt.Request != nil {
		reqSchema, err := SchemaFromType(reflect.TypeOf(rt.Request))
		if err != nil {
			return fmt.Errorf("request body: %w", err)
		}
		op.RequestBody = &RequestBody{
			Required: !rt.RequestOptional,
			Content:  map[string]MediaType{"application/json": {Schema: reqSchema}},
		}
	}

	for _, status := range rt.ResponseStatuses {
		resp := &Response{Description: statusDescription(status)}
		if rt.Response != nil {
			schema, err := SchemaFromType(reflect.TypeOf(rt.Response))
			if err != nil {
				return fmt.Errorf("response body: %w", err)
			}
			resp.Content = map[string]MediaType{"application/json": {Schema: schema}}
		}
		op.Responses[fmt.Sprintf("%d", status)] = resp
	}

	// Every operation shares the Problem envelope for unlisted statuses.
	op.Responses["default"] = &Response{
		Description: "Error (RFC 7807 problem+json).",
		Content:     map[string]MediaType{"application/problem+json": {Schema: &Schema{Ref: "#/components/schemas/Problem"}}},
	}

	switch rt.Method {
	case http.MethodGet:
		pi.Get = op
	case http.MethodPost:
		pi.Post = op
	case http.MethodPut:
		pi.Put = op
	case http.MethodPatch:
		pi.Patch = op
	case http.MethodDelete:
		pi.Delete = op
	}
	return nil
}

func statusDescription(code int) string {
	if s, ok := httpStatusText[code]; ok {
		return s
	}
	return http.StatusText(code)
}

func problemSchema() *Schema {
	return &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"type":     {Type: "string", Format: "uri", Description: "Problem type URI."},
			"title":    {Type: "string", Description: "Short human-readable summary."},
			"status":   {Type: "integer", Format: "int32", Description: "HTTP status code."},
			"detail":   {Type: "string", Description: "Human-readable explanation."},
			"instance": {Type: "string", Format: "uri", Description: "URI identifying this occurrence."},
			"code":     {Type: "string", Description: "Machine-readable error code."},
		},
		Required: []string{"title", "status", "code"},
	}
}

// Marshaling ---------------------------------------------------------

// marshalYAML emits the document with 2-space indentation. yaml.v3
// preserves struct field order and sorts map keys alphabetically,
// giving stable output without an ordered-map layer.
func marshalYAML(doc *Document) ([]byte, error) {
	var buf strings.Builder
	buf.WriteString("# GENERATED by internal/tools/openapigen. DO NOT EDIT.\n")
	buf.WriteString("# Run `just generate` after changing internal/server/api routes.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// marshalJSON emits pretty-printed JSON with a trailing newline.
// json.Marshal preserves struct field order and sorts map keys, so
// output is stable.
func marshalJSON(doc *Document) ([]byte, error) {
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(buf, '\n'), nil
}

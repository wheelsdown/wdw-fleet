package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Schema is the subset of OpenAPI 3.1 Schema Object emitted by this
// generator. Additional fields will be added as they become needed.
type Schema struct {
	Type                 string             `json:"type,omitempty" yaml:"type,omitempty"`
	Format               string             `json:"format,omitempty" yaml:"format,omitempty"`
	Description          string             `json:"description,omitempty" yaml:"description,omitempty"`
	Ref                  string             `json:"$ref,omitempty" yaml:"$ref,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Required             []string           `json:"required,omitempty" yaml:"required,omitempty"`
	Items                *Schema            `json:"items,omitempty" yaml:"items,omitempty"`
	Nullable             bool               `json:"nullable,omitempty" yaml:"nullable,omitempty"`
	Enum                 []string           `json:"enum,omitempty" yaml:"enum,omitempty"`
	ReadOnly             bool               `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	AdditionalProperties any                `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
}

// SchemaFromType reflects on a Go type and returns the OpenAPI schema
// that describes JSON values a caller would send or receive. Types
// unknown to the generator produce an error rather than a placeholder
// -- the generator is meant to fail loudly on unsupported shapes so
// they get modeled explicitly rather than silently smuggled through.
func SchemaFromType(t reflect.Type) (*Schema, error) {
	// Peel pointer indirection but remember it: pointer means nullable.
	nullable := false
	if t.Kind() == reflect.Pointer {
		nullable = true
		t = t.Elem()
	}

	// Special-cased named types checked before Kind() so time.Time
	// (a struct) and uuid.UUID (an array) don't fall into the generic
	// struct / array branches.
	switch t {
	case reflect.TypeOf(time.Time{}):
		return &Schema{Type: "string", Format: "date-time", Nullable: nullable}, nil
	case reflect.TypeOf(uuid.UUID{}):
		return &Schema{Type: "string", Format: "uuid", Nullable: nullable}, nil
	case reflect.TypeOf(json.RawMessage{}):
		return &Schema{Type: "object", AdditionalProperties: true, Nullable: nullable}, nil
	}

	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string", Nullable: nullable}, nil
	case reflect.Bool:
		return &Schema{Type: "boolean", Nullable: nullable}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s := &Schema{Type: "integer", Nullable: nullable}
		switch t.Kind() {
		case reflect.Int64, reflect.Uint32, reflect.Uint64:
			s.Format = "int64"
		default:
			s.Format = "int32"
		}
		return s, nil
	case reflect.Float32, reflect.Float64:
		s := &Schema{Type: "number", Nullable: nullable}
		if t.Kind() == reflect.Float32 {
			s.Format = "float"
		} else {
			s.Format = "double"
		}
		return s, nil
	case reflect.Slice, reflect.Array:
		// []byte would normally emit as an array of ints; treat as
		// base64 string, matching stdlib json.Marshal behavior.
		if t.Elem().Kind() == reflect.Uint8 {
			return &Schema{Type: "string", Format: "byte", Nullable: nullable}, nil
		}
		items, err := SchemaFromType(t.Elem())
		if err != nil {
			return nil, fmt.Errorf("array element %s: %w", t.Elem(), err)
		}
		return &Schema{Type: "array", Items: items, Nullable: nullable}, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key must be string, got %s", t.Key())
		}
		val, err := SchemaFromType(t.Elem())
		if err != nil {
			return nil, fmt.Errorf("map value %s: %w", t.Elem(), err)
		}
		return &Schema{Type: "object", AdditionalProperties: val, Nullable: nullable}, nil
	case reflect.Struct:
		return structSchema(t, nullable)
	case reflect.Interface:
		// `any` in a payload -> untyped object.
		return &Schema{Type: "object", AdditionalProperties: true, Nullable: nullable}, nil
	}
	return nil, fmt.Errorf("unsupported type %s (kind %s)", t, t.Kind())
}

// structSchema walks the exported fields of t and emits an object
// schema. Field name comes from the `json` tag (or the field name
// verbatim); a `json:"-"` field is skipped; a `,omitempty` tag or a
// pointer field is optional (not added to `required`).
func structSchema(t reflect.Type, nullable bool) (*Schema, error) {
	s := &Schema{Type: "object"}
	if nullable {
		s.Nullable = true
	}
	props := map[string]*Schema{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, opts := parseJSONTag(f.Tag.Get("json"), f.Name)
		if name == "-" {
			continue
		}
		fieldSchema, err := SchemaFromType(f.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s: %w", t.Name(), f.Name, err)
		}
		// The `openapi` tag lets a field carry OpenAPI-only metadata
		// the json tag can't express.
		applyOpenAPITag(fieldSchema, f.Tag.Get("openapi"))
		props[name] = fieldSchema
		if !opts.omitempty && f.Type.Kind() != reflect.Pointer && !fieldSchema.Nullable {
			required = append(required, name)
		}
	}
	s.Properties = props
	if len(required) > 0 {
		s.Required = required
	}
	return s, nil
}

type jsonTagOpts struct {
	omitempty bool
}

func parseJSONTag(tag, fallback string) (string, jsonTagOpts) {
	if tag == "" {
		return fallback, jsonTagOpts{}
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = fallback
	}
	var opts jsonTagOpts
	for _, p := range parts[1:] {
		if p == "omitempty" {
			opts.omitempty = true
		}
	}
	return name, opts
}

// applyOpenAPITag folds the `openapi:"..."` tag's key=value pairs
// onto the schema. Supported keys: format, enum, readOnly,
// description. Unknown keys are silently ignored to keep the surface
// small; extend as needed.
func applyOpenAPITag(s *Schema, tag string) {
	if tag == "" {
		return
	}
	for _, part := range strings.Split(tag, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 0 {
			continue
		}
		key := kv[0]
		val := ""
		if len(kv) == 2 {
			val = kv[1]
		}
		switch key {
		case "format":
			s.Format = val
		case "readOnly":
			s.ReadOnly = true
		case "description":
			s.Description = val
		case "enum":
			s.Enum = strings.Split(val, "|")
		}
	}
}

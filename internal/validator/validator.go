// Package validator provides the core STAC object validation logic.
// It inspects the `type` and `stac_version` fields of a STAC document to
// resolve the appropriate base JSON Schema, then additionally validates against
// every URL listed in the `stac_extensions` field.  This design allows the
// same logic to be used by both the HTTP server and the CLI tool.
package validator

import (
	"errors"
	"fmt"

	"github.com/StacLabs/gostac-validator/internal/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// baseSchemaURL returns the canonical stacspec.org schema URL for a given
// STAC object type and version.
func baseSchemaURL(stacType, version string) (string, error) {
	switch stacType {
	case "Feature":
		return fmt.Sprintf("https://schemas.stacspec.org/v%s/item-spec/json-schema/item.json", version), nil
	case "Catalog":
		return fmt.Sprintf("https://schemas.stacspec.org/v%s/catalog-spec/json-schema/catalog.json", version), nil
	case "Collection":
		return fmt.Sprintf("https://schemas.stacspec.org/v%s/collection-spec/json-schema/collection.json", version), nil
	default:
		return "", fmt.Errorf("unknown STAC type %q: expected Feature, Catalog, or Collection", stacType)
	}
}

// Error is the structured result of a single schema validation failure.
type Error struct {
	Path      string `json:"path"`
	Message   string `json:"message"`
	SchemaURL string `json:"schema_url,omitempty"`
}

// Result is returned by Validate and describes whether the object is valid
// along with any validation errors.
type Result struct {
	Valid  bool    `json:"valid"`
	Errors []Error `json:"errors,omitempty"`
}

// STAC is the entry-point for all STAC validation.  It wraps a schema cache
// so that compiled schemas are reused across requests.
type STAC struct {
	cache *schemas.Cache
}

// New returns a STAC validator backed by the supplied schema cache.
func New(cache *schemas.Cache) *STAC {
	return &STAC{cache: cache}
}

// Validate validates instance (already decoded from JSON using
// jsonschema.UnmarshalJSON to preserve number precision) against all
// applicable STAC schemas.
//
// The schemas are determined automatically:
//   - The base schema is derived from the `type` and `stac_version` fields.
//   - Each URL in the `stac_extensions` array is treated as an additional
//     JSON Schema URL and validated in turn.
func (v *STAC) Validate(instance any) (Result, error) {
	obj, ok := instance.(map[string]any)
	if !ok {
		return Result{}, errors.New("STAC object must be a JSON object")
	}

	stacVersion, err := stringField(obj, "stac_version")
	if err != nil {
		return Result{}, err
	}

	stacType, err := stringField(obj, "type")
	if err != nil {
		return Result{}, err
	}

	baseURL, err := baseSchemaURL(stacType, stacVersion)
	if err != nil {
		return Result{}, err
	}

	// Collect schema URLs: base schema first, then each extension schema.
	schemaURLs := []string{baseURL}
	if exts, ok := obj["stac_extensions"]; ok {
		extSlice, ok := exts.([]any)
		if !ok {
			return Result{}, errors.New("stac_extensions must be an array")
		}
		for i, ext := range extSlice {
			u, ok := ext.(string)
			if !ok {
				return Result{}, fmt.Errorf("stac_extensions[%d] must be a string URL", i)
			}
			schemaURLs = append(schemaURLs, u)
		}
	}

	res := Result{Valid: true}

	for _, schemaURL := range schemaURLs {
		schema, err := v.cache.Get(schemaURL)
		if err != nil {
			return Result{}, fmt.Errorf("loading schema %q: %w", schemaURL, err)
		}

		if err := schema.Validate(instance); err != nil {
			var valErr *jsonschema.ValidationError
			if errors.As(err, &valErr) {
				res.Valid = false
				res.Errors = append(res.Errors, collectErrors(schemaURL, valErr)...)
			} else {
				return Result{}, fmt.Errorf("unexpected validation error: %w", err)
			}
		}
	}

	return res, nil
}

// collectErrors extracts ONLY the actionable leaf errors from the validation tree.
func collectErrors(schemaURL string, ve *jsonschema.ValidationError) []Error {
	detailed := ve.DetailedOutput()
	if detailed == nil {
		return nil
	}
	return extractLeaves(schemaURL, *detailed)
}

// extractLeaves recursively walks the DetailedOutput tree and returns only
// the nodes that have no children. This strips out the useless "'allOf' failed" 
// wrapper messages and gives you the exact missing fields or type mismatches.
func extractLeaves(schemaURL string, unit jsonschema.OutputUnit) []Error {
	// If this node has no children, it's a root cause (leaf error)
	if len(unit.Errors) == 0 {
		msg := ""
		if unit.Error != nil {
			msg = unit.Error.String()
		}
		
		// Occasionally a leaf error is empty if it's just a structural boolean failure,
		// but we'll capture it anyway just in case.
		if msg == "" {
			msg = "validation failed"
		}

		return []Error{{
			Path:      unit.InstanceLocation,
			Message:   msg,
			SchemaURL: schemaURL,
		}}
	}

	// If it has children, ignore this parent node and dig deeper
	var leaves []Error
	for _, child := range unit.Errors {
		leaves = append(leaves, extractLeaves(schemaURL, child)...)
	}
	return leaves
}

// stringField extracts a non-empty string value from a map, returning a
// descriptive error if the field is absent or the wrong type.
func stringField(obj map[string]any, key string) (string, error) {
	v, ok := obj[key]
	if !ok {
		return "", fmt.Errorf("missing required field %q", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %q must be a string", key)
	}
	if s == "" {
		return "", fmt.Errorf("field %q must not be empty", key)
	}
	return s, nil
}

// Package server provides HTTP handlers for the STAC validator API.
// Each handler decodes an incoming JSON request, resolves the appropriate
// JSON schema from the shared cache, validates the payload, and returns a
// structured JSON response.
package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/StacLabs/gostac-validator/internal/schemas"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Handler holds the dependencies needed by every HTTP handler in this package.
type Handler struct {
	cache *schemas.Cache
}

// NewHandler returns a Handler wired to the supplied schema cache.
func NewHandler(cache *schemas.Cache) *Handler {
	return &Handler{cache: cache}
}

// RegisterRoutes attaches all routes to mux.  Callers may pass http.DefaultServeMux
// or any compatible multiplexer.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /validate", h.Validate)
	mux.HandleFunc("GET /health", h.Health)
}

// validateRequest is the JSON body accepted by the /validate endpoint.
type validateRequest struct {
	// SchemaURL is the canonical URL of the JSON Schema to validate against
	// (e.g. "https://schemas.stacspec.org/v1.0.0/item-spec/json-schema/item.json").
	SchemaURL string `json:"schema_url"`
	// Object is the raw STAC JSON to validate.
	Object json.RawMessage `json:"object"`
}

// validationError represents a single schema validation failure.
type validationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// validateResponse is the JSON body returned by the /validate endpoint.
type validateResponse struct {
	Valid  bool              `json:"valid"`
	Errors []validationError `json:"errors,omitempty"`
}

// healthResponse is the JSON body returned by the /health endpoint.
type healthResponse struct {
	Status string `json:"status"`
}

// Validate handles POST /validate.
// It decodes the request body, fetches (or retrieves from cache) the specified
// JSON Schema, validates the supplied STAC object, and returns a JSON response
// describing whether the object is valid and any validation errors.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	if req.SchemaURL == "" {
		writeError(w, http.StatusBadRequest, "schema_url is required")
		return
	}
	if len(req.Object) == 0 {
		writeError(w, http.StatusBadRequest, "object is required")
		return
	}

	schema, err := h.cache.Get(req.SchemaURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not load schema: "+err.Error())
		return
	}

	// Decode the raw JSON object into a generic value so the schema library
	// can validate it without depending on a concrete Go type.
	var instance any
	if err := json.Unmarshal(req.Object, &instance); err != nil {
		writeError(w, http.StatusBadRequest, "invalid object JSON: "+err.Error())
		return
	}

	resp := validateResponse{Valid: true}

	if err := schema.Validate(instance); err != nil {
		var valErr *jsonschema.ValidationError
		if errors.As(err, &valErr) {
			resp.Valid = false
			resp.Errors = collectErrors(valErr)
		} else {
			writeError(w, http.StatusInternalServerError, "validation error: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// Health handles GET /health and returns a simple liveness check.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

// collectErrors flattens the tree of jsonschema.ValidationError into a slice
// of validationError values suitable for serialising into the API response.
// It uses the library's BasicOutput format which provides a flat list of
// failures with string instance-location paths and human-readable messages.
func collectErrors(ve *jsonschema.ValidationError) []validationError {
	basic := ve.BasicOutput()

	var out []validationError
	for _, unit := range basic.Errors {
		msg := ""
		if unit.Error != nil {
			msg = unit.Error.String()
		}
		out = append(out, validationError{
			Path:    unit.InstanceLocation,
			Message: msg,
		})
	}
	return out
}

// writeJSON serialises v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error body with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

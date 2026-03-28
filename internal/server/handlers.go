// Package server provides HTTP handlers for the STAC validator API.
// Each handler decodes an incoming JSON request body as a raw STAC object,
// auto-detects the applicable schemas from the object's own `type` and
// `stac_extensions` fields, and returns a structured JSON response.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/StacLabs/gostac-validator/internal/validator"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Handler holds the dependencies needed by every HTTP handler in this package.
type Handler struct {
	validator *validator.STAC
}

// NewHandler returns a Handler wired to the supplied STAC validator.
func NewHandler(v *validator.STAC) *Handler {
	return &Handler{validator: v}
}

// RegisterRoutes attaches all routes to mux.  Callers may pass http.DefaultServeMux
// or any compatible multiplexer.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /validate", h.Validate)
	mux.HandleFunc("GET /health", h.Health)
}

// Validate handles POST /validate.
//
// The request body must be a raw STAC Item, Catalog, or Collection JSON object.
// The handler reads the `type` and `stac_version` fields to resolve the correct
// base schema, then validates against each URL in `stac_extensions` as well.
//
// Precision is preserved by using jsonschema.UnmarshalJSON instead of the
// standard library decoder, which would truncate large floating-point
// coordinates to float64.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read request body: "+err.Error())
		return
	}

	// Use jsonschema.UnmarshalJSON to preserve full numeric precision for
	// geographic coordinates that would otherwise be truncated by float64.
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	result, err := h.validator.Validate(instance)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Health handles GET /health and returns a simple liveness check.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readBody reads the entire request body and returns it as bytes, capping at
// 10 MiB to guard against excessively large payloads.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	const maxBytes = 10 << 20 // 10 MiB
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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


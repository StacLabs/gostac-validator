// Package server provides the HTTP transport layer for the STAC validator.
// It handles incoming HTTP requests, parses payloads (including massive
// STAC ItemCollections), routes them to the concurrent validation engine,
// and formats the resulting validation reports into JSON.
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/StacLabs/gostac-validator/internal/validator"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Handler holds the dependencies required by the HTTP endpoints.
// It wraps the core STAC validator so that all requests share the same
// thread-safe, in-memory schema cache.
type Handler struct {
	validator *validator.STAC
}

// NewHandler creates a new HTTP Handler injected with the provided STAC validator.
func NewHandler(v *validator.STAC) *Handler {
	return &Handler{validator: v}
}

// RegisterRoutes attaches the API endpoints to the provided HTTP multiplexer.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /validate", h.Validate)
	mux.HandleFunc("GET /health", h.Health)
}

// BatchResponse represents the JSON payload returned by the /validate endpoint.
// It provides a high-level summary of the batch operation alongside the detailed
// validation results for each individual STAC item processed.
type BatchResponse struct {
	TotalProcessed int                `json:"total_processed"`
	ValidCount     int                `json:"valid_count"`
	InvalidCount   int                `json:"invalid_count"`
	Results        []validator.Result `json:"results"`
}

// Validate is the primary endpoint for STAC validation (POST /validate).
// It intelligently detects the shape of the incoming JSON payload. If the payload
// is a single STAC Item, it wraps it in a slice. If it is an array of items or a
// FeatureCollection/ItemCollection, it extracts the items and validates them all
// concurrently using a Goroutine worker pool.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	body, err := readBody(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read request body: "+err.Error())
		return
	}

	// Parse JSON safely using jsonschema's unmarshaler to prevent Go from 
	// silently truncating highly precise geographic coordinate floats.
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	var itemsToValidate []any

	// Smart Routing: Inspect the generic parsed JSON to determine its structure.
	switch data := instance.(type) {
	case []any:
		// Payload is a raw JSON array of STAC objects.
		itemsToValidate = data
	case map[string]any:
		// Payload is a JSON object. Check if it is a collection of features.
		if typ, ok := data["type"].(string); ok && (typ == "FeatureCollection" || typ == "ItemCollection") {
			if features, ok := data["features"].([]any); ok {
				itemsToValidate = features
			} else {
				writeError(w, http.StatusBadRequest, "FeatureCollection is missing the 'features' array")
				return
			}
		} else {
			// Payload is a single STAC Item, Catalog, or Collection. Wrap it for the batch processor.
			itemsToValidate = []any{data}
		}
	default:
		writeError(w, http.StatusBadRequest, "Unrecognized JSON structure")
		return
	}

	// Process the entire batch concurrently.
	results := h.validateConcurrent(itemsToValidate)

	// Tally the results for the summary payload.
	validCount := 0
	for _, res := range results {
		if res.Valid {
			validCount++
		}
	}

	response := BatchResponse{
		TotalProcessed: len(itemsToValidate),
		ValidCount:     validCount,
		InvalidCount:   len(itemsToValidate) - validCount,
		Results:        results,
	}

	writeJSON(w, http.StatusOK, response)
}

// validateConcurrent validates a slice of STAC objects simultaneously.
// It spawns a Goroutine for every item in the slice, allowing massive batches 
// to be processed in the same time it takes to process a single item. It uses a 
// sync.WaitGroup to block until all items have finished validating against the RAM cache.
func (h *Handler) validateConcurrent(items []any) []validator.Result {
	results := make([]validator.Result, len(items))
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		
		go func(index int, stacItem any) {
			defer wg.Done()
			
			res, err := h.validator.Validate(stacItem)
			if err != nil {
				// Fallback for edge cases where the item itself is fundamentally malformed
				res = validator.Result{
					Valid:  false,
					Errors: []validator.Error{{Message: err.Error()}},
				}
			}
			results[index] = res
		}(i, item)
	}

	wg.Wait()
	
	return results
}

// Health is a simple liveness probe endpoint (GET /health) used by Docker 
// or Kubernetes to ensure the HTTP server is responsive.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readBody safely reads the HTTP request body into a byte slice. 
// It enforces a 50 MiB limit using http.MaxBytesReader to prevent malicious 
// or accidentally massive payloads from causing Out-Of-Memory (OOM) crashes.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	const maxBytes = 150 << 20 // 150 MiB
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeJSON serializes the provided Go data structure into JSON and writes it 
// to the HTTP response with the specified status code and headers.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError is a convenience helper for formatting human-readable error messages 
// into a standardized JSON response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

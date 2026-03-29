// Package server provides the HTTP transport layer for the STAC validator.
// It handles incoming HTTP requests, parses payloads (including massive
// STAC ItemCollections), routes them to the concurrent validation engine,
// and formats the resulting validation reports into JSON.
package server

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/StacLabs/gostac-validator/internal/validator"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Handler holds the dependencies for the HTTP server.
type Handler struct {
	validator *validator.STAC
}

// NewHandler initializes the handler with a shared validator instance.
func NewHandler(v *validator.STAC) *Handler {
	return &Handler{validator: v}
}

// RegisterRoutes defines the API endpoints.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /validate", h.Validate)
	mux.HandleFunc("GET /health", h.Health)
}

// BatchResponse defines the JSON structure returned to the client.
type BatchResponse struct {
	TotalProcessed int                `json:"total_processed"`
	ValidCount     int                `json:"valid_count"`
	InvalidCount   int                `json:"invalid_count"`
	Results        []validator.Result `json:"results"`
}

// Validate handles STAC validation for single items or batches.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	body, err := readBody(w, r)
	if err != nil {
		log.Printf("❌ Error reading body: %v", err)
		writeError(w, http.StatusBadRequest, "could not read request body: "+err.Error())
		return
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		log.Printf("❌ Error parsing JSON: %v", err)
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	var itemsToValidate []any

	// Smart Routing logic
	switch data := instance.(type) {
	case []any:
		itemsToValidate = data
	case map[string]any:
		if typ, ok := data["type"].(string); ok && (typ == "FeatureCollection" || typ == "ItemCollection") {
			if features, ok := data["features"].([]any); ok {
				itemsToValidate = features
			} else {
				writeError(w, http.StatusBadRequest, "Collection is missing the 'features' array")
				return
			}
		} else {
			itemsToValidate = []any{data}
		}
	default:
		writeError(w, http.StatusBadRequest, "Unrecognized JSON structure")
		return
	}

	// Process batch concurrently
	results := h.validateConcurrent(itemsToValidate)

	validCount := 0
	type errorKey struct {
		Message string
		Schema  string
	}
	errorFrequencies := make(map[errorKey]int)

	for _, res := range results {
		if res.Valid {
			validCount++
		} else if len(res.Errors) > 0 {
			// We group errors by Message and the Schema URL that failed
			key := errorKey{
				Message: res.Errors[0].Message,
				Schema:  res.Errors[0].AbsoluteKeywordLocation, // Use AbsoluteKeywordLocation here
			}
			errorFrequencies[key]++
		}
	}
	invalidCount := len(itemsToValidate) - validCount

	// Log metrics with the new intelligent titles and error aggregation
	duration := time.Since(start)
	total := len(itemsToValidate)

	if total == 1 {
		if invalidCount > 0 && len(results[0].Errors) > 0 {
			err := results[0].Errors[0]
			log.Printf("📄 SINGLE ITEM | Valid: 0 | Invalid: 1 | Time: %v | Reason: %s (Schema: %s)",
				duration, err.Message, err.AbsoluteKeywordLocation)
		} else {
			log.Printf("📄 SINGLE ITEM | Valid: 1 | Invalid: 0 | Time: %v", duration)
		}
	} else {
		log.Printf("⚡ BATCH PROCESSED | Total: %d | Valid: %d | Invalid: %d | Time: %v",
			total, validCount, invalidCount, duration)

		if invalidCount > 0 {
			log.Printf("   -> Failure Summary (%d unique error types):", len(errorFrequencies))
			for key, count := range errorFrequencies {
				log.Printf("      - [%d items]: %s", count, key.Message)
				log.Printf("        Context: %s", key.Schema)
			}
		}
	}

	writeJSON(w, http.StatusOK, BatchResponse{
		TotalProcessed: total,
		ValidCount:     validCount,
		InvalidCount:   invalidCount,
		Results:        results,
	})
}

// validateConcurrent runs the validator across multiple goroutines.
func (h *Handler) validateConcurrent(items []any) []validator.Result {
	results := make([]validator.Result, len(items))
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(index int, stacItem any) {
			defer wg.Done()
			res, err := h.validator.Validate(stacItem)
			if err != nil {
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

// Health is a liveness probe.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// getBodyLimit returns the limit in bytes from MAX_BODY_SIZE_MB env var,
// defaulting to 150MB.
func getBodyLimit() int64 {
	const defaultLimitMB = 150
	limitStr := os.Getenv("MAX_BODY_SIZE_MB")
	if limitStr == "" {
		return defaultLimitMB << 20
	}

	limitMB, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil {
		// Log the error or just return default
		return defaultLimitMB << 20
	}

	return limitMB << 20
}

// readBody handles reading the request with a configurable limit.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	// Dynamically fetch the limit
	maxBytes := getBodyLimit()

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeJSON is a helper to return JSON responses.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError is a helper for JSON error messages.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

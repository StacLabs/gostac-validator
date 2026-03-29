package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetBodyLimit(t *testing.T) {
	// Table-driven tests for environment variable parsing
	tests := []struct {
		name     string
		envValue string
		expected int64
	}{
		{
			name:     "Default value when unset",
			envValue: "",
			expected: 150 << 20, // 150 MB
		},
		{
			name:     "Valid custom value",
			envValue: "500",
			expected: 500 << 20, // 500 MB
		},
		{
			name:     "Invalid string falls back to default",
			envValue: "not_a_number",
			expected: 150 << 20, // 150 MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.envValue != "" {
				os.Setenv("MAX_BODY_SIZE_MB", tt.envValue)
			} else {
				os.Unsetenv("MAX_BODY_SIZE_MB")
			}

			// Clean up after test
			t.Cleanup(func() {
				os.Unsetenv("MAX_BODY_SIZE_MB")
			})

			got := getBodyLimit()
			if got != tt.expected {
				t.Errorf("getBodyLimit() = %d bytes, want %d bytes", got, tt.expected)
			}
		})
	}
}

func TestReadBody_LimitExceeded(t *testing.T) {
	// Set the limit artificially low (1 MB)
	os.Setenv("MAX_BODY_SIZE_MB", "1")
	t.Cleanup(func() { os.Unsetenv("MAX_BODY_SIZE_MB") })

	// Create a 2MB dummy payload
	bigPayload := bytes.Repeat([]byte("a"), 2<<20)

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(bigPayload))
	w := httptest.NewRecorder()

	_, err := readBody(w, req)
	if err == nil {
		t.Fatal("Expected an error when reading body larger than limit, got nil")
	}

	// http.MaxBytesReader returns an error containing "request body too large"
	if err.Error() != "http: request body too large" {
		t.Errorf("Expected 'request body too large' error, got: %v", err)
	}
}

func TestValidateHandler_PayloadTooLarge(t *testing.T) {
	// Set the limit artificially low (1 MB)
	os.Setenv("MAX_BODY_SIZE_MB", "1")
	t.Cleanup(func() { os.Unsetenv("MAX_BODY_SIZE_MB") })

	// We can pass nil for the validator because the body limit check
	// happens before the handler attempts to use the validator!
	handler := NewHandler(nil)

	// Create a 2MB dummy payload
	bigPayload := bytes.Repeat([]byte(`{"type":"Feature"}`), 100000)

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(bigPayload))
	w := httptest.NewRecorder()

	handler.Validate(w, req)

	res := w.Result()
	defer res.Body.Close()

	// Ensure the server rejected it as a Bad Request
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
	}

	// Verify the JSON error message
	var errResponse map[string]string
	if err := json.NewDecoder(res.Body).Decode(&errResponse); err != nil {
		t.Fatalf("Failed to decode response JSON: %v", err)
	}

	expectedErrFragment := "could not read request body"
	if errStr, ok := errResponse["error"]; !ok || len(errStr) < len(expectedErrFragment) || errStr[:len(expectedErrFragment)] != expectedErrFragment {
		t.Errorf("Expected error starting with '%s', got '%s'", expectedErrFragment, errStr)
	}
}

func TestHealthHandler(t *testing.T) {
	handler := NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.Health(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", res.StatusCode)
	}

	var response map[string]string
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status, ok := response["status"]; !ok || status != "ok" {
		t.Errorf("Expected status='ok', got %v", response)
	}
}

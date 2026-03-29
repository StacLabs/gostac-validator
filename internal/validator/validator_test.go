package validator

import (
	"github.com/StacLabs/gostac-validator/internal/schemas"
	"testing"
)

func TestValidate(t *testing.T) {
	// Setup a real cache for the test
	cache := schemas.NewCache()
	v := New(cache)

	// Define our test cases
	tests := []struct {
		name    string
		input   map[string]any
		isValid bool
	}{
		{
			name: "Valid Minimal Item",
			input: map[string]any{
				"stac_version": "1.0.0",
				"type":         "Feature",
				"id":           "test-item",
				"geometry":     nil,
				"properties":   map[string]any{"datetime": "2024-01-01T00:00:00Z"},
				"assets":       map[string]any{},
				"links":        []any{},
			},
			isValid: true,
		},
		{
			name: "Invalid - Missing ID",
			input: map[string]any{
				"stac_version": "1.0.0",
				"type":         "Feature",
				"geometry":     nil,
				"properties":   map[string]any{"datetime": "2024-01-01T00:00:00Z"},
				"assets":       map[string]any{},
				"links":        []any{},
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := v.Validate(tt.input)
			if err != nil {
				t.Fatalf("Validate failed with unexpected error: %v", err)
			}
			if res.Valid != tt.isValid {
				t.Errorf("Expected valid=%v, got %v", tt.isValid, res.Valid)
			}
		})
	}
}

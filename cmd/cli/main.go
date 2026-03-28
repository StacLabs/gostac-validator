// Command cli validates STAC JSON files from the command line.
//
// Usage:
//
//	validate-stac <file> [<file>...]   — validate one or more JSON files
//	validate-stac                      — validate JSON read from stdin
//
// For each input the command prints a JSON result object to stdout and exits
// with a non-zero status if any input fails validation.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/StacLabs/gostac-validator/internal/schemas"
	"github.com/StacLabs/gostac-validator/internal/validator"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func main() {
	cache := schemas.NewCache()
	v := validator.New(cache)

	args := os.Args[1:]

	// If no file arguments are given, read from stdin.
	if len(args) == 0 {
		args = []string{"-"}
	}

	allValid := true
	for _, path := range args {
		ok, err := validateFile(v, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			os.Exit(1)
		}
		if !ok {
			allValid = false
		}
	}

	if !allValid {
		os.Exit(1)
	}
}

// fileResult is the per-file JSON output written to stdout.
type fileResult struct {
	File     string           `json:"file"`
	Duration string           `json:"duration"` // Added this!
	Result   validator.Result `json:"result"`
}

func validateFile(v *validator.STAC, path string) (bool, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer f.Close()
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return false, fmt.Errorf("reading: %w", err)
	}

	// Use jsonschema.UnmarshalJSON to preserve full numeric precision for
	// geographic coordinates that would otherwise be truncated by float64.
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("parsing JSON: %w", err)
	}

	start := time.Now()
	result, err := v.Validate(instance)
	duration := time.Since(start) // Stop the timer

	if err != nil {
		return false, err
	}

	label := path
	if path == "-" {
		label = "<stdin>"
	}

	out := fileResult{
		File:     label,
		Duration: duration.String(),
		Result:   result,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return false, fmt.Errorf("encoding output: %w", err)
	}

	return result.Valid, nil
}

// Package schemas provides a thread-safe, in-memory cache for JSON schemas
// used during STAC object validation. Schemas are fetched from the network on
// first use and stored in a sync.Map so that subsequent validations can reuse
// the compiled schema without additional I/O or compilation overhead.
package schemas

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Cache holds compiled JSON schemas keyed by their canonical URL string.
// It is safe to call any method concurrently.
type Cache struct {
	mu       sync.Map
	compiler *jsonschema.Compiler
}

// NewCache returns an initialised Cache whose compiler is pre-configured to
// load remote schemas over HTTP when they are not already cached.
func NewCache() *Cache {
	c := jsonschema.NewCompiler()
	c.UseLoader(jsonschema.SchemeURLLoader{
		"file":  jsonschema.FileLoader{},
		"http":  newHTTPLoader(),
		"https": newHTTPLoader(),
	})

	return &Cache{compiler: c}
}

// Get returns the compiled schema for the given URL, compiling and caching it
// if this is the first request for that URL.
func (c *Cache) Get(schemaURL string) (*jsonschema.Schema, error) {
	if v, ok := c.mu.Load(schemaURL); ok {
		return v.(*jsonschema.Schema), nil
	}

	schema, err := c.compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("compiling schema %q: %w", schemaURL, err)
	}

	// Store only if another goroutine hasn't beaten us to it.
	actual, _ := c.mu.LoadOrStore(schemaURL, schema)
	return actual.(*jsonschema.Schema), nil
}

// httpLoader satisfies jsonschema.URLLoader by fetching schemas over HTTP/S
// using a shared http.Client with a sensible timeout.
type httpLoader http.Client

func newHTTPLoader() *httpLoader {
	return (*httpLoader)(&http.Client{Timeout: 15 * time.Second})
}

func (l *httpLoader) Load(url string) (any, error) {
	client := (*http.Client)(l)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching schema %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching schema %q: unexpected status %d", url, resp.StatusCode)
	}

	return jsonschema.UnmarshalJSON(resp.Body)
}

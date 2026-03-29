# GoSTAC Validator

An enterprise-grade, ultra-fast JSON Schema validator specifically built for the SpatioTemporal Asset Catalog (STAC) ecosystem. 

Written in Go, this tool solves the infamous STAC "Cold Start" `$ref` problem by intelligently downloading, resolving, compiling, and **caching** complex STAC extension schemas in RAM. 

It drops validation times from ~2,500ms (network-bound) down to **~0.24ms** (RAM-bound) per item, making it capable of validating millions of STAC items a day using native Go concurrency.

## Features
* **Dual-Mode:** Run as a local CLI tool or a highly concurrent HTTP Microservice.
* **Thread-Safe Schema Caching:** Downloads remote `$ref`s from GitHub exactly *once* and caches the compiled execution tree in RAM.
* **Smart Batching:** Send a single Item, a raw JSON Array, or a massive `ItemCollection`. The server automatically detects the format and spins up thousands of Goroutines to validate them all concurrently.
* **Auto-Discovery:** Automatically reads `type`, `stac_version`, and `stac_extensions` to apply the correct schemas natively.
* **Lossless Precision:** Bypasses Go's default `float64` truncation to preserve massive STAC geographic coordinate precision safely.
* **PCRE Regex Support:** Uses `regexp2` to natively handle STAC extensions (like `eo`) that require complex negative-lookahead regexes `^(?!eo:)`.

---

## Table of Contents
1. [Installation & Build](#installation--build)
2. [CLI Usage](#cli-usage)
3. [Microservice Usage](#microservice-usage)
4. [Configuration](#configuration)
5. [Benchmarking & Performance](#benchmarking--performance)
6. [Architecture Overview](#architecture-overview)

---

## Installation & Build

**Prerequisites:** [Go 1.26.1+](https://go.dev/dl/)

Clone the repository and download the dependencies:
```bash
git clone https://github.com/StacLabs/gostac-validator.git
cd gostac-validator
go mod tidy
```

Because the project uses standard Go layout, you compile the CLI and the Server into two separate, lightweight binaries:

```bash
# Build the CLI tool
go build -o stac-cli ./cmd/cli

# Build the HTTP Microservice
go build -o stac-server ./cmd/server
```

## CLI Usage

The CLI tool is perfect for local testing, CI/CD pipelines, or ad-hoc validation of STAC files on your machine.

### Run the validator:
```bash
./stac-cli path/to/your/item.json
```

### Expected output:
```json
{
  "file": "path/to/your/item.json",
  "duration": "12.4ms",
  "result": {
    "valid": false,
    "errors": [
      {
        "path": "/assets/SR_B2",
        "message": "additional properties 'eo:bands' not allowed",
        "schema_url": "https://stac-extensions.github.io/eo/v2.0.0/schema.json"
      }
    ]
  }
}
```
**Note:** The CLI starts from a "cold cache" every time it is run. For true performance, use the microservice.

## Microservice Usage

The HTTP server uses a thread-safe `sync.Map` to cache compiled schemas. It is designed to sit behind an ingestor API (like FastAPI) and process thousands of concurrent validation requests safely.

## Configuration

The STAC Validator Microservice can be tuned via environment variables to handle different infrastructure requirements and payload sizes.

| Variable | Description | Default |
| :--- | :--- | :--- |
| `ADDR` | The network address and port for the server to listen on. | `:8080` |
| `MAX_BODY_SIZE_MB` | The maximum allowed size of an incoming POST request in Megabytes. | `150` |

### Start the server:
```bash
# Start with default 150MB limit
./stac-server

# Start with a custom 512MB limit for large batches
MAX_BODY_SIZE_MB=512 ./stac-server
```

### Test via cURL:

You can send a single STAC Item, a JSON array of items, or a full `ItemCollection`. The server will process batches concurrently.

```bash
curl -X POST http://localhost:8080/validate \
     -H "Content-Type: application/json" \
     -d '{
           "type": "Feature", 
           "stac_version": "1.0.0", 
           "id": "test", 
           "geometry": null, 
           "properties": {}, 
           "links": [], 
           "assets": {}
         }'
```

## Benchmarking & Performance

To witness the speed of the schema caching engine and Go's concurrency, boot up the `./stac-server` in one terminal, and run the benchmark scripts in another.

### 1. Sequential HTTP Overhead (benchmark.py)

```plaintext
🔥 Sending First Request (Cold Start)...
First request took: 2425.52 ms

⚡ Sending 1,000 Requests (Warm Cache)...
Total time for 1,000 items: 1.33 seconds
Average time per STAC item: 1.33 ms
```

### 2. Concurrent Batch Processing (benchmark_batch.py)

This test wraps 10,000 STAC items into a single ItemCollection (~104 MB payload) and sends them in one POST request. The Go server spawns 10,000 Goroutines to validate them simultaneously against the RAM cache.

```plaintext
🔥 Sending 1 item to warm up the cache (Cold Start)...
Cache warmed up!

📦 Building an ItemCollection with 10,000 items...
Payload size: 104.23 MB

⚡ Firing massive batch at the Go server...

✅ Batch Processing Complete!
Total Items Processed: 10,000
Valid Items: 0
Invalid Items: 10,000
------------------------------
Total Time Taken: 2.4264 seconds
Average Time per Item: 0.2426 ms
Throughput: 4,121 items / second
```

## Architecture Overview

* **`cmd/`**: Contains the entry points for the applications (`cli` and `server`). These wrap the core business logic and handle the executable binaries.
* **`internal/validator/`**: The core STAC business logic. It automatically detects STAC types, determines the required core and extension schemas, and executes the validation.
* **`internal/schemas/`**: The thread-safe compiler and cache. It resolves remote `$ref` URLs safely and utilizes double-checked locking to prevent race conditions under high concurrency.
* **`internal/server/`**: The HTTP handlers. It enforces JSON geographic coordinate precision decoding, safely unpacks collections for Goroutine batching, and implements max-byte payload limits for security.

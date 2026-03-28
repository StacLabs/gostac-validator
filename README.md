# GoSTAC Validator

An enterprise-grade, ultra-fast JSON Schema validator specifically built for the SpatioTemporal Asset Catalog (STAC) ecosystem. 

Written in Go, this tool solves the infamous STAC "Cold Start" `$ref` problem by intelligently downloading, resolving, compiling, and **caching** complex STAC extension schemas in RAM. 

It drops validation times from ~2,500ms (network-bound) to **~1ms** (RAM-bound) per item, making it capable of validating millions of STAC items a day.

## Features
* **Dual-Mode:** Run as a local CLI tool or a highly concurrent HTTP Microservice.
* **Thread-Safe Schema Caching:** Downloads remote `$ref`s from GitHub exactly *once* and caches the compiled execution tree in RAM.
* **Auto-Discovery:** Automatically reads `type`, `stac_version`, and `stac_extensions` to apply the correct schemas natively.
* **Lossless Precision:** Bypasses Go's default `float64` truncation to preserve massive STAC geographic coordinate precision safely.
* **PCRE Regex Support:** Uses `regexp2` to natively handle STAC extensions (like `eo`) that require complex negative-lookahead regexes `^(?!eo:)`.

---

## Table of Contents
1. [Installation & Build](#installation--build)
2. [CLI Usage](#cli-usage)
3. [Microservice Usage](#microservice-usage)
4. [Benchmarking the Cache](#benchmarking-the-cache)
5. [Architecture overview](#architecture-overview)

---

## Installation & Build

**Prerequisites:** [Go 1.26.1+](https://go.dev/dl/)

Clone the repository and download the dependencies:
```bash
git clone [https://github.com/StacLabs/gostac-validator.git](https://github.com/StacLabs/gostac-validator.git)
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
        "schema_url": "[https://stac-extensions.github.io/eo/v2.0.0/schema.json](https://stac-extensions.github.io/eo/v2.0.0/schema.json)"
      }
    ]
  }
}
```
**Note:** The CLI starts from a "cold cache" every time it is run. For true performance, use the microservice.

## Microservice Usage

The HTTP server uses a thread-safe `sync.Map` to cache compiled schemas. It is designed to sit behind an ingestor API (like FastAPI) and process thousands of concurrent validation requests safely.

### Start the server:
```bash
./stac-server
# 🚀 STAC Validation Server running on http://localhost:8080/validate
```

### Test via cURL:
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

## Benchmarking the Cache

To witness the speed of the schema caching engine, boot up the `./stac-server` in one terminal, and run this Python script in another.

### benchmark.py
```python
import time
import requests
import json

# Load a local STAC item
with open("sample_stac/test_item.json", "r") as f:
    stac_item = json.load(f)

url = "http://localhost:8080/validate"

print("🔥 Sending First Request (Cold Start)...")
start = time.time()
requests.post(url, json=stac_item)
print(f"First request took: {(time.time() - start) * 1000:.2f} ms\n")

print("⚡ Sending 1,000 Requests (Warm Cache)...")
start_bulk = time.time()
for _ in range(1000):
    requests.post(url, json=stac_item)
total_time = time.time() - start_bulk

print(f"Total time for 1,000 items: {total_time:.2f} seconds")
print(f"Average time per STAC item: {(total_time / 1000) * 1000:.2f} ms")
```

### Results on standard hardware:

```plaintext
🔥 Sending First Request (Cold Start)...
First request took: 2425.52 ms

⚡ Sending 1,000 Requests (Warm Cache)...
Total time for 1,000 items: 1.33 seconds
Average time per STAC item: 1.33 ms
```

## Architecture Overview

* **`cmd/`**: Contains the entry points for the applications (`cli` and `server`). These wrap the core business logic and handle the executable binaries.
* **`internal/validator/`**: The core STAC business logic. It automatically detects STAC types, determines the required core and extension schemas, and executes the validation.
* **`internal/schemas/`**: The thread-safe compiler and cache. It resolves remote `$ref` URLs safely and utilizes double-checked locking to prevent race conditions under high concurrency.
* **`internal/server/`**: The HTTP handlers. It enforces JSON geographic coordinate precision decoding and implements max-byte payload limits for security.


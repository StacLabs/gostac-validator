# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.2] - 2026-03-30

### Added
- **Configurable Payload Limits**: Introduced the `MAX_BODY_SIZE_MB` environment variable, allowing operators to tune the maximum allowed size for incoming STAC batches (defaults to 150MB).

## [0.1.1] - 2026-03-29

### Added
- **Intelligent Batch Logging**: Enhanced the `/validate` endpoint with high-visibility logging for both single-item and bulk `ItemCollection` requests.
- **Error Aggregation**: Implemented a frequency-based error summarizer for large batches. Instead of flooding logs, the service now identifies and counts unique failure reasons (e.g., "Top failure reason (99/100): 'datetime' is required").
- **Schema Contextualization**: Validation failures now include the specific `AbsoluteKeywordLocation`, allowing developers to click directly to the failing STAC Extension schema.
- **Performance Metrics**: Real-time execution timing added to all validation logs to monitor throughput and latency.
- **Project Tooling**: Added a `Makefile` and `go test` suite to standardize the contributor workflow and ensure build stability. [#3](https://github.com/StacLabs/gostac-validator/pull/3)

## [0.1.0] - 2026-03-29

### Added
- Core STAC validation engine with PCRE regex (`^(?!eo:)`) support via `regexp2`.
- Lossless float decoding for STAC geographic coordinates.
- Thread-safe, in-memory JSON Schema `$ref` cache to eliminate cold starts.
- Concurrent batch processing `/validate` endpoint for `ItemCollection` payloads.
- High-performance CLI tool for local STAC validation.
- Dockerfile for microservice deployment.

[Unreleased]: https://github.com/StacLabs/gostac-validator/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/StacLabs/gostac-validator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/StacLabs/gostac-validator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/StacLabs/gostac-validator/releases/tag/v0.1.0
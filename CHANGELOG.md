# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-29
### Added
- Core STAC validation engine with PCRE regex (`^(?!eo:)`) support via `regexp2`.
- Lossless float decoding for STAC geographic coordinates.
- Thread-safe, in-memory JSON Schema `$ref` cache to eliminate cold starts.
- Concurrent batch processing `/validate` endpoint for `ItemCollection` payloads.
- High-performance CLI tool for local STAC validation.
- Dockerfile for microservice deployment.

[Unreleased]: https://github.com/StacLabs/gostac-validator/compare/v0.1.0...main
[v0.1.0]: https://github.com/StacLabs/gostac-validator/compare/v0.1.0...v0.0.0
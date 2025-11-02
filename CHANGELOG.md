# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-11-02

### Added
- 🎉 Initial release of GoForge
- ✨ CLI tool for generating Go HTTP servers
- 🏗️ Clean Architecture implementation
- 🔌 Health check endpoint generation
- 🛑 Graceful shutdown support
- 📝 Optional structured logging
- 🐳 Optional Dockerfile generation
- 📖 Optional README generation
- ⚡ Embedded templates (single binary distribution)
- 🧪 Comprehensive test suite (80%+ coverage)
- 📚 Full documentation

### Features
- Interactive mode with prompts
- Non-interactive mode for CI/CD (`--yes` flag)
- Configurable port and application name
- Custom output directory support
- Production-ready server code generation
- Middleware support (logging, panic recovery)
- HTTP timeouts configuration
- Signal handling for graceful shutdown

### Architecture
- Domain-Driven Design
- Dependency Injection
- Interface-based design
- Template-based code generation
- Zero external runtime dependencies

### Documentation
- Comprehensive README with examples
- Contributing guidelines
- MIT License
- Code comments and package documentation
- Architecture diagrams

---

## [Unreleased]

### Planned for 1.1.0
- Database integration (PostgreSQL, MySQL)
- Authentication middleware (JWT)
- API documentation generation (Swagger/OpenAPI)
- Configuration file support (YAML/TOML)
- More template customization options

### Planned for 2.0.0
- gRPC server support
- GraphQL server support
- Kubernetes manifests generation
- Monitoring/metrics (Prometheus)
- Distributed tracing (OpenTelemetry)
- Multi-endpoint API generation

---

## Release Notes

### Version 1.0.0 - "The Beginning"

This is the first stable release of GoForge. The project demonstrates:

1. **Clean Architecture**: Proper separation of concerns with Domain, Usecase, and Infrastructure layers
2. **Dependency Injection**: Manual DI implementation for clarity and control
3. **Template Engine**: Go's text/template for code generation
4. **Production Ready**: Generated servers include health checks, graceful shutdown, and proper error handling

**What Makes This Release Special:**
- Single binary distribution (templates embedded)
- Works from any directory
- Zero configuration required
- 40+ passing tests
- Complete documentation

**Usage Example:**
```bash
goforge generate --name=my-api --port=8080
cd my-api
go run main.go
```

**Generated Server Features:**
- Health endpoint at `/health`
- JSON responses
- Graceful shutdown with 30s timeout
- Request/response logging (optional)
- Docker support (optional)
- Production-ready structure

---

## How to Upgrade

### From Source
```bash
go install github.com/Frank5ive/goforge@latest
```

### Binary
Download the latest release from [releases page](https://github.com/Frank5ive/goforge/releases).

---

## Breaking Changes

None yet - this is the first release!

---

## Deprecations

None yet.

---

## Security

No known security vulnerabilities.

To report security vulnerabilities, please email: [security@example.com](mailto:security@example.com)

---

[Unreleased]: https://github.com/Frank5ive/goforge/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/Frank5ive/goforge/releases/tag/v1.0.0

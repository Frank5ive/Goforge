<div align="center">

# ⚒️ GoForge

**Forge production-ready Go HTTP servers in seconds**

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Clean Architecture](https://img.shields.io/badge/Architecture-Clean-brightgreen.svg)](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[Features](#-features) •
[Installation](#-installation) •
[Quick Start](#-quick-start) •
[Documentation](#-documentation) •
[Architecture](#-architecture) •
[Contributing](#-contributing)

</div>

---

## 🎯 What is GoForge?

GoForge is a CLI tool that generates production-ready Go HTTP servers using only the standard library: structured logging with `log/slog`, Go 1.22+ routing, graceful shutdown, tests, linting, CI, and a hardened Dockerfile. Stop copying boilerplate code and start forging quality servers instantly.

### Why GoForge?

✅ **Production Ready** - Graceful shutdown, request IDs, panic recovery, and timeouts  
✅ **Zero Dependencies** - Generated servers use only the Go standard library  
✅ **Tested and Linted** - Generated projects ship with tests and pass their own golangci-lint config  
✅ **Verified Output** - Every option combination is generated, linted, tested, built, and booted by GoForge's test suite  
✅ **Ready for CI/CD** - Optional GitHub Actions workflow and non-root Docker image  

---

## 🚀 Features

### Core Features
- 🔌 **Health Endpoint** - `GET /health` with a JSON response
- 🛑 **Graceful Shutdown** - Handles SIGINT/SIGTERM and drains in-flight requests
- 🧭 **Modern Routing** - Go 1.22+ `http.ServeMux` patterns like `GET /users/{id}`, with automatic 405s
- 🪪 **Request IDs** - Every response carries an `X-Request-ID`; incoming IDs are reused for tracing
- 🧯 **Panic Recovery** - Panics become 500s and are logged with a stack trace
- ⏱️ **Timeouts** - Read, write, idle, and read-header timeouts
- ⚙️ **12-Factor Config** - `PORT` (and `LOG_LEVEL`/`LOG_FORMAT` with logging) from the environment
- 🧪 **Tests and Tooling** - Handler and middleware tests, a Makefile, and a golangci-lint config

### Optional Features
- 📝 **Structured Logging** - `log/slog` in text or JSON, with one log line per request
- 🐳 **Docker** - Multi-stage build, non-root user, health check, and `.dockerignore`
- 🤖 **GitHub Actions CI** - Vet, race-enabled tests, lint, and Docker build
- 📖 **README** - Documentation for the generated project

---

## 📦 Installation

### Option 1: From Source

```bash
go install github.com/Frank5ive/goforge@latest
```

### Option 2: Download Binary

Download the latest release from the [releases page](https://github.com/Frank5ive/goforge/releases).

```bash
# Linux/macOS
chmod +x goforge
sudo mv goforge /usr/local/bin/

# Windows
# Add goforge.exe to your PATH
```

### Option 3: Build from Source

```bash
git clone https://github.com/Frank5ive/goforge.git
cd goforge
go build -o goforge .
```

---

## ⚡ Quick Start

### Generate Your First Server

```bash
# Interactive mode: prompts for the module path and features
goforge generate --name=my-api

# Non-interactive mode (scripts and CI)
goforge generate --name=my-api --module=github.com/you/my-api --yes

# Preview the files without writing anything
goforge generate --name=my-api --dry-run --yes
```

### Run Your Server

```bash
cd my-api
go run .
```

Your server is now running at `http://localhost:8080` 🎉

### Test the Health Endpoint

```bash
curl http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-11-02T16:43:58Z",
  "service": "my-api"
}
```

---

## 📚 Documentation

### Command Line Options

```bash
goforge generate [flags]
```

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--name` | `-n` | `myapp` | Name of the application (also the project directory) |
| `--module` | `-m` | app name | Go module path, e.g. `github.com/acme/my-api` |
| `--port` | `-p` | `8080` | Default port (the `PORT` env var overrides it at runtime) |
| `--output` | `-o` | `.` | Directory to create the project in |
| `--logger` | | `true` | Include request logging |
| `--docker` | | `true` | Include a Dockerfile |
| `--readme` | | `true` | Include a README.md |
| `--yes` | `-y` | `false` | Skip prompts and use flag values and defaults |
| `--dry-run` | | `false` | List the files that would be created without writing anything |
| `--git` | | `false` | Run `git init` in the new project |

#### Interactive Prompts

When running in a terminal without `--yes`, GoForge asks for anything not already set by flag:

1. **Go module path** - Leave empty to use the app name
2. **Features** - Toggle request logging, Dockerfile, and README

Prompts need a terminal. In scripts and CI, pass `--yes` and set options with flags.
If generation fails partway, GoForge removes the partially created project directory.

### Examples

#### Basic Server

```bash
goforge generate --name=hello-api --port=3000
```

#### Minimal Server Without Docker or Logging

```bash
goforge generate --name=worker --logger=false --docker=false --yes
```

#### CI/CD Ready

```bash
goforge generate --name=ci-api --module=github.com/acme/ci-api --output=./services --yes --git
```

---

## 🏗️ Generated Project Structure

```
my-api/
├── main.go                    # Entry point: config, startup, graceful shutdown
├── server/
│   ├── server.go              # HTTP server and middleware chain
│   ├── routes.go              # Route registration (add endpoints here)
│   ├── middleware.go          # Request ID, panic recovery, request logging
│   └── server_test.go
├── handlers/
│   ├── health.go              # GET /health
│   ├── respond.go             # JSON response helper
│   └── health_test.go
├── logger/                    # slog setup from LOG_LEVEL / LOG_FORMAT (optional)
├── Makefile                   # run, build, test, cover, lint, docker-build, ...
├── .golangci.yml              # golangci-lint v2 config
├── .github/workflows/ci.yml   # GitHub Actions CI (optional)
├── Dockerfile                 # Multi-stage, non-root (optional)
├── .dockerignore              # (optional)
├── README.md                  # (optional)
├── .gitignore
└── go.mod
```

---

## 🎨 GoForge Architecture

GoForge itself is built following **Clean Architecture** principles:

```
┌─────────────────────────────────────────────┐
│            External Interfaces              │
│  (HTTP Handlers, CLI, Database Adapters)    │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────┴──────────────────────────┐
│         Application Business Rules          │
│              (Use Cases)                    │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────┴──────────────────────────┐
│        Enterprise Business Rules            │
│          (Entities, Interfaces)             │
└─────────────────────────────────────────────┘
```

### Key Principles

✅ **Dependency Inversion** - Core logic doesn't depend on frameworks  
✅ **Separation of Concerns** - Each layer has a single responsibility  
✅ **Testability** - Easy to mock and test components  
✅ **Flexibility** - Swap implementations without affecting business logic  

---

## 🔧 Advanced Usage

### Custom Port Configuration

```bash
goforge generate --name=custom-api --port=3000
```

### Generate Multiple Services

```bash
# User service
goforge generate --name=user-service --port=8001 --output=./services

# Order service
goforge generate --name=order-service --port=8002 --output=./services

# Payment service
goforge generate --name=payment-service --port=8003 --output=./services
```

### Docker Deployment

```bash
# Generate with Dockerfile
goforge generate --name=my-api --port=8080
cd my-api

# Build Docker image
docker build -t my-api:latest .

# Run container
docker run -p 8080:8080 my-api:latest

# Test
curl http://localhost:8080/health
```

---

## 🧪 Testing

GoForge's test suite generates a project for every combination of options using the
real embedded templates, then checks that each one is gofmt-clean, passes `go vet`,
compiles, serves `/health`, and shuts down gracefully on `SIGTERM`.

```bash
# Full suite, including the end-to-end generation test
go test ./...

# Skip the end-to-end test for a quick run
go test -short ./...
```

---

## 🛠️ Development

### Project Stack

- **Language**: Go 1.24+
- **CLI Framework**: Cobra
- **Interactive Prompts**: Huh
- **Architecture**: Clean Architecture
- **Testing**: Go testing package
- **Template Engine**: Go text/template

### Building GoForge

```bash
# Clone repository
git clone https://github.com/Frank5ive/goforge.git
cd goforge

# Install dependencies
go mod download

# Build
go build -o goforge .

# Run tests
go test ./...

# Run with coverage
go test -cover ./...
```

### Project Structure

```
goforge/
├── cmd/                    # CLI commands (Cobra)
├── internal/
│   ├── domain/            # Business entities & interfaces
│   ├── usecase/           # Application logic
│   ├── infrastructure/    # Template & file operations
│   └── di/                # Dependency injection
├── templates/             # Go server templates
└── main.go               # Entry point
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for how generation works and how to add templates or options.

---

## 🤝 Contributing

We welcome contributions! Here's how you can help:

### Ways to Contribute

- 🐛 **Report Bugs** - Open an issue with reproduction steps
- ✨ **Suggest Features** - Share your ideas for improvements
- 📖 **Improve Documentation** - Fix typos, add examples
- 🔧 **Submit PRs** - Fix bugs or implement features

### Contribution Process

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Clean Architecture principles
- Write tests for new features
- Update documentation
- Keep commits atomic and descriptive
- Run `go fmt` and `go vet` before committing

---

## 📝 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

### What This Means

✅ Commercial use allowed  
✅ Modification allowed  
✅ Distribution allowed  
✅ Private use allowed  

---

## 🙏 Acknowledgments

- Inspired by [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html) by Uncle Bob
- Built with [Cobra](https://github.com/spf13/cobra) CLI framework
- Uses [Huh](https://github.com/charmbracelet/huh) for interactive prompts
- The Go gopher was designed by [Renée French](https://reneefrench.blogspot.com/) and is licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). The pixel-art gopher in GoForge's terminal banner is adapted from it.

---

## 📮 Support & Contact

- 🐛 **Issues**: [GitHub Issues](https://github.com/Frank5ive/goforge/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/Frank5ive/goforge/discussions)

---

## ⭐ Star History

If you find GoForge useful, please consider giving it a star! ⭐

It helps others discover the project and motivates continued development.

---

## 🗺️ Roadmap

### v0.1 (Current)
- ✅ Basic server generation
- ✅ Health endpoints
- ✅ Docker support
- ✅ Logging

### Next
- [ ] Database integration (PostgreSQL, MySQL)
- [ ] Authentication middleware (JWT)
- [ ] API documentation generation (Swagger/OpenAPI)
- [ ] Configuration file support (YAML/TOML)

### Future
- [ ] gRPC server support
- [ ] GraphQL server support
- [ ] Kubernetes manifests
- [ ] Monitoring/metrics (Prometheus)
- [ ] Distributed tracing (OpenTelemetry)

---

<div align="center">

**Built with ❤️ by Frank Gondwe**

[⬆ Back to Top](#-goforge)

</div>

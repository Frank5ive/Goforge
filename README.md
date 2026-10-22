<div align="center">

<img src="docs/logo.svg" alt="GoForge: a pixel-art Go gopher next to the GOFORGE wordmark, with the tagline 'Forge production-ready Go servers in seconds'" width="100%">

[![CI](https://github.com/Frank5ive/goforge/actions/workflows/ci.yml/badge.svg)](https://github.com/Frank5ive/goforge/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Frank5ive/goforge)](https://github.com/Frank5ive/goforge/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/Frank5ive/goforge.svg)](https://pkg.go.dev/github.com/Frank5ive/goforge)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[Features](#-features) •
[Installation](#-installation) •
[Quick Start](#-quick-start) •
[Adding Resources](#-adding-resources) •
[Adding a Database](#-adding-a-database) •
[Documentation](#-documentation) •
[Architecture](#-goforge-architecture) •
[Contributing](#-contributing)

</div>

---

## 🎯 What is GoForge?

GoForge is a CLI tool that generates production-ready Go HTTP servers using only the standard library: structured logging with `log/slog`, Go 1.22+ routing, graceful shutdown, tests, linting, CI, and a hardened Dockerfile. Then it keeps helping: `goforge add resource` adds complete, tested REST resources to your project. Stop copying boilerplate code and start forging quality servers instantly.

### Why GoForge?

✅ **Production Ready** - Graceful shutdown, request IDs, panic recovery, and timeouts  
✅ **Grows With You** - `goforge add resource` generates CRUD endpoints, storage, business logic, and tests  
✅ **Zero Dependencies** - Generated servers use only the Go standard library  
✅ **Tested and Linted** - Generated projects ship with tests and pass their own golangci-lint config  
✅ **Verified Output** - Every option combination is generated, linted, tested, built, and booted by GoForge's test suite  
✅ **Ready for CI/CD** - Optional GitHub Actions workflow and non-root Docker image  

---

## 🚀 Features

### Core Features
- 🔌 **Health and Readiness** - `GET /health` for liveness and `GET /readyz` for readiness (checks dependencies such as the database)
- 🛑 **Graceful Shutdown** - Handles SIGINT/SIGTERM and drains in-flight requests
- 🧭 **Modern Routing** - Go 1.22+ `http.ServeMux` patterns like `GET /users/{id}`, with automatic 405s
- 🪪 **Request IDs** - Every response carries an `X-Request-ID`; incoming IDs are reused for tracing
- 🧯 **Panic Recovery** - Panics become 500s and are logged with a stack trace
- ⏱️ **Timeouts** - Read, write, idle, and read-header timeouts
- ⚙️ **12-Factor Config** - A `config` package loads `PORT`, `CORS_ALLOWED_ORIGINS`, `MAX_BODY_BYTES`, and `SHUTDOWN_TIMEOUT` from the environment
- 🌐 **CORS** - Opt-in with `CORS_ALLOWED_ORIGINS`, including preflight handling
- 📏 **Body Limits** - Request bodies are capped (1 MiB by default) and oversized ones get `413`
- 🧰 **JSON Helpers** - `WriteJSON`, `WriteError`, and a strict `DecodeJSON` that rejects unknown fields
- 🧪 **Tests and Tooling** - Handler and middleware tests, a Makefile, and a golangci-lint config

### Optional Features
- 📝 **Structured Logging** - `log/slog` in text or JSON, with one log line per request
- 🐳 **Docker** - Multi-stage build, non-root user, health check, and `.dockerignore`
- 🤖 **GitHub Actions CI** - Vet, race-enabled tests, lint, and Docker build
- 📖 **README** - Documentation for the generated project

---

## 📦 Installation

### Option 1: Go Install

Requires Go 1.26 or newer to install GoForge. Generated projects need Go 1.24 or newer.

```bash
go install github.com/Frank5ive/goforge@latest
```

### Option 2: Download a Binary

Prebuilt binaries for Linux, macOS, and Windows (amd64 and arm64) are on the
[releases page](https://github.com/Frank5ive/goforge/releases), with a `checksums.txt` to verify them.

```bash
# Linux/macOS: extract and move onto your PATH
tar -xzf goforge_*_linux_amd64.tar.gz
sudo mv goforge /usr/local/bin/

# Windows: extract the .zip and add goforge.exe to your PATH
```

### Option 3: Build from Source

```bash
git clone https://github.com/Frank5ive/goforge.git
cd goforge
go build -o goforge .
```

### Shell Completion

Tab-complete commands and flags in bash, zsh, fish, or PowerShell:

```bash
# bash (current session; add to ~/.bashrc to keep it)
source <(goforge completion bash)

# zsh
goforge completion zsh > "${fpath[1]}/_goforge"

# fish
goforge completion fish > ~/.config/fish/completions/goforge.fish
```

Run `goforge completion <shell> --help` for details.

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

## 🧩 Adding Resources

Inside a project created by `goforge generate`, add a complete REST resource with one command:

```bash
goforge add resource product --fields "name:string,price:float,in_stock:bool,image_url:string?"
```

```
✅ Added the product resource. Created 7 files:
   products/product.go         # Product model, Input, Patch, validation rules, Repository interface
   products/memory.go          # Thread-safe in-memory Repository
   products/service.go         # Business logic: list, get, create, update, patch, delete
   products/handler.go         # HTTP handlers and route registration
   products/service_test.go
   products/handler_test.go
   products/requests.http      # Example requests to run from your editor

📝 Updated:
   server/routes.go            # Routes registered automatically

🌐 Endpoints:
   GET    /products?limit=20&offset=0
   POST   /products
   GET    /products/{id}
   PUT    /products/{id}
   PATCH  /products/{id}
   DELETE /products/{id}
```

**Ready to go:** the routes are registered in `server/routes.go` for you, so the endpoints
work as soon as you start (or restart) the server with `go run .`. No wiring needed. Open
`products/requests.http` in VS Code (REST Client extension) or a JetBrains IDE to try every
endpoint with one click; it already points at your project's port.

Each resource uses Clean Architecture–style layering in a single package: handlers call
the service, the service holds the business rules, and storage sits behind a `Repository`
interface, so resources move to PostgreSQL (see [Adding a Database](#-adding-a-database))
without touching the service or handlers. Keeping a resource in one package makes it easy to
find and follow.

**Fields** are `name:type` pairs. Types: `string`, `int`, `float`, `bool`, and `time`
(RFC 3339 in JSON). String fields are required unless marked optional with `?`. `id`,
`created_at`, and `updated_at` are added automatically.

**Listing** is paginated: `GET /products?limit=20&offset=40` returns
`{"items": [...], "total": 57, "limit": 20, "offset": 40}`, oldest first. `limit` defaults to
20 and is capped at 100.

**Updating:** `PUT` replaces every editable field, while `PATCH` changes only the fields you
send (`{"price": 24.5}`). Both run the same validation rules.

**Responses:** `201` with a `Location` header on create, `204` on delete, `404` for unknown
IDs, `422` with every failed rule listed (`{"error":"invalid product: name is required"}`),
and `400` for malformed JSON, unknown fields, or invalid `limit`/`offset`.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--fields` | `-f` | `name:string` | Comma-separated `name:type` fields |
| `--plural` | | derived | Plural for irregular words, e.g. `--plural people` for `person` |
| `--dir` | `-d` | `.` | Project directory (the one containing `go.mod`) |
| `--dry-run` | | `false` | Show the files that would be created or changed |

---

## 🐘 Adding a Database

Store your resources in PostgreSQL with one command, either when generating
(`goforge generate --db postgres`) or later, inside an existing project:

```bash
goforge add db postgres
make db-up     # start PostgreSQL in Docker
go run .       # connects, applies migrations, and serves your resources from PostgreSQL
```

This adds:

- **A `db` package:** a [pgx](https://github.com/jackc/pgx) connection pool and a small
  migration runner. SQL files in `db/migrations` are embedded in the binary and applied in
  order at startup, each once, in its own transaction. A PostgreSQL advisory lock stops two
  instances from migrating at the same time.
- **Startup wiring:** `main.go` connects before serving, `DATABASE_URL` joins the `config`
  package (it defaults to the local Docker database), and `/readyz` returns `503` when the
  database is unreachable.
- **Every existing resource moved to PostgreSQL:** each gets a `PostgresRepository`, a
  migration that creates its table, and updated wiring in `server/routes.go`. **Resources you
  add afterwards use PostgreSQL automatically.**
- **Tests that prove both storage layers behave the same:** each resource's
  `repository_test.go` runs one set of tests against the in-memory and the PostgreSQL
  repositories. The PostgreSQL run is skipped unless `TEST_DATABASE_URL` is set;
  `make test-db` runs everything against PostgreSQL in Docker.
- **Local tooling:** `docker-compose.yml` (PostgreSQL, plus the app when the project has a
  Dockerfile) and `make db-up`, `db-down`, `db-reset`, and `test-db`.
- **CI:** the generated GitHub Actions workflow gets a PostgreSQL service, so the database
  tests run on every push.
- **The pgx dependency** in `go.mod`. The project's minimum Go version becomes 1.25, which pgx
  requires.

Edits are made only at marker comments and are all planned before anything is written. If
any step fails, including downloading pgx, the project is left exactly as it was. Use
`--dry-run` to preview the changes.

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
| `--db` | | none | Store data in a database: `postgres` (same as `goforge add db postgres`) |

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
├── main.go                    # Entry point: startup and graceful shutdown
├── config/                    # Settings from environment variables
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

- **Language**: Go 1.26+
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
- ✅ Server generation with health checks, graceful shutdown, and middleware
- ✅ Structured logging, tests, linting, CI, and Docker
- ✅ Environment-based configuration, CORS, and request body limits
- ✅ `goforge add resource` for tested CRUD resources
- ✅ PostgreSQL with migrations (`goforge add db postgres`)

### Next
- [ ] Authentication middleware (JWT)
- [ ] OpenAPI documentation generated from resources
- [ ] Homebrew tap

### Future
- [ ] gRPC server support
- [ ] GraphQL server support
- [ ] Kubernetes manifests
- [ ] Monitoring/metrics (Prometheus)
- [ ] Distributed tracing (OpenTelemetry)

---

<div align="center">

**Built with ❤️ by Frank Gondwe**

[⬆ Back to Top](#top)

</div>

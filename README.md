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

GoForge is a CLI tool that generates production-ready Go HTTP servers with health checks, graceful shutdown, middleware, and Docker support. Stop copying boilerplate code and start forging quality servers instantly.

### Why GoForge?

✅ **Production Ready** - Graceful shutdown, health checks, panic recovery, timeouts  
✅ **Zero Configuration** - Works immediately after installation  
✅ **Verified Output** - Every option combination is generated, compiled, vetted, and booted by the test suite  
✅ **Docker Support** - Optional Dockerfile generation  
✅ **Request Logging** - Optional logger with per-request method, path, status, and duration  

---

## 🚀 Features

### Core Features
- 🧱 **Clear Structure** - Separate `handlers`, `server`, and `logger` packages
- 🔌 **Health Endpoint** - Built-in `/health` endpoint with JSON response
- 🛑 **Graceful Shutdown** - Handles SIGINT/SIGTERM and drains in-flight requests
- ⚙️ **12-Factor Config** - Port can be overridden with the `PORT` environment variable
- ⚡ **Fast Generation** - Creates projects in under 1 second
- 📦 **Single Binary** - No external dependencies, templates embedded

### Optional Features
- 📝 **Request Logging** - Logs method, path, status, and duration for every request
- 🐳 **Docker Support** - Multi-stage Dockerfile for production
- 📖 **Documentation** - Auto-generated README with usage examples
- 🔧 **Middleware** - Panic recovery and request logging

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
# Interactive mode (recommended for first time)
goforge generate --name=my-api --port=8080

# Non-interactive mode (perfect for CI/CD)
goforge generate --name=my-api --port=8080 --yes
```

### Run Your Server

```bash
cd my-api
go mod download
go run main.go
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
| `--name` | `-n` | `myapp` | Name of the application |
| `--port` | `-p` | `8080` | Port number for the server |
| `--output` | `-o` | `.` | Output directory |
| `--yes` | | `false` | Skip interactive prompts |

#### Interactive Prompts

When running without `--yes`, GoForge will ask:

1. **Include logger?** - Adds a leveled logger and request logging middleware
2. **Generate Dockerfile?** - Creates production-ready Docker image
3. **Generate README.md?** - Auto-generates documentation

### Examples

#### Basic Server

```bash
goforge generate --name=hello-api --port=3000
```

#### Production Server with All Features

```bash
goforge generate --name=production-api --port=8080
# Answer "Yes" to all prompts
```

#### CI/CD Ready

```bash
goforge generate --name=ci-api --port=8080 --yes --output=./services
```

---

## 🏗️ Generated Project Structure

```
my-api/
├── main.go                 # Application entry point
├── go.mod                  # Go module file
├── .gitignore             # Git ignore patterns
├── Dockerfile             # Docker configuration (optional)
├── README.md              # Project documentation (optional)
├── handlers/              # HTTP request handlers
│   └── health.go         # Health check endpoint
├── server/                # Server configuration
│   ├── server.go         # HTTP server with graceful shutdown
│   └── middleware.go     # HTTP middleware
└── logger/                # Logging utilities (optional)
    └── logger.go         # Structured logger
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
- **Interactive Prompts**: Survey
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
- Uses [Survey](https://github.com/AlecAivazis/survey) for interactive prompts
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

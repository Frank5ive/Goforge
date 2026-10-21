# GoForge Quick Reference

## 🚀 Installation

```bash
# From source
go install github.com/Frank5ive/goforge@latest

# Or build locally
git clone https://github.com/Frank5ive/goforge.git
cd goforge
go build -o goforge .
```

## ⚡ Quick Commands

```bash
# Generate server (interactive)
goforge generate --name=my-api --port=8080

# Generate server (non-interactive - for CI/CD)
goforge generate --name=my-api --port=8080 --yes

# Custom output directory
goforge generate -n api -p 3000 -o ./services

# Show help
goforge --help
goforge generate --help

# Show version
goforge --version
```

## 📁 Generated Project Structure

```
my-api/
├── main.go              # Entry point
├── go.mod               # Dependencies
├── handlers/
│   └── health.go       # Health endpoint
├── server/
│   ├── server.go       # HTTP server
│   └── middleware.go   # Middleware
├── logger/             # Optional
│   └── logger.go
├── Dockerfile          # Optional
└── README.md           # Optional
```

## 🧪 Using Generated Server

```bash
cd my-api
go mod download
go run main.go

# In another terminal
curl http://localhost:8080/health
```

## 🔧 Development

```bash
# Run tests
go test ./...

# With coverage
go test -cover ./...

# Build
go build -o goforge .

# Format code
go fmt ./...

# Vet code
go vet ./...
```

## 📚 Documentation

- Full docs: [README.md](README.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Issues: https://github.com/Frank5ive/goforge/issues

## 🎯 Features Generated

- ✅ Health endpoint (`/health`)
- ✅ Graceful shutdown
- ✅ `PORT` environment variable override
- ✅ Request logging (optional)
- ✅ Panic recovery middleware
- ✅ Configurable timeouts
- ✅ Docker support (optional)
- ✅ README with usage (optional)

## 🏗️ GoForge Architecture Layers

1. **Domain** - Business entities & interfaces
2. **Usecase** - Application logic
3. **Infrastructure** - External adapters
4. **DI** - Dependency wiring

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📝 License

MIT License - See [LICENSE](LICENSE)

## 🙏 Support

- 🐛 Report bugs: [GitHub Issues](https://github.com/Frank5ive/goforge/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/Frank5ive/goforge/discussions)
- ⭐ Star the project if you find it useful!

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

# Add a REST resource (run inside the generated project)
goforge add resource product --fields "name:string,price:float"

# Show help
goforge --help
goforge generate --help

# Show version
goforge --version
```

## 📁 Generated Project Structure

```
my-api/
├── main.go             # Entry point
├── config/             # Settings from environment variables
├── go.mod
├── Makefile            # make run / test / lint / build
├── server/             # Server, routes, middleware, tests
├── handlers/           # Handlers, JSON helper, tests
├── logger/             # Optional: slog setup
├── .github/workflows/  # Optional: CI
├── Dockerfile          # Optional
└── README.md           # Optional
```

## 🧪 Using Generated Server

```bash
cd my-api
make run            # or: go run .

# In another terminal
curl http://localhost:8080/health

make test           # run the generated tests
make help           # list all commands
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

- ✅ Health endpoint (`GET /health`)
- ✅ Graceful shutdown
- ✅ Go 1.22+ routing patterns with automatic 405s
- ✅ Request IDs and panic recovery middleware
- ✅ `PORT` environment variable override
- ✅ Tests, Makefile, and golangci-lint config
- ✅ Structured `log/slog` logging (optional)
- ✅ Non-root Docker image (optional)
- ✅ GitHub Actions CI (optional)
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

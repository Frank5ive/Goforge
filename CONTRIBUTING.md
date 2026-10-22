# Contributing to GoForge

First off, thank you for considering contributing to GoForge! It's people like you that make GoForge such a great tool.

## Code of Conduct

This project and everyone participating in it is governed by our Code of Conduct. By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the issue list as you might find out that you don't need to create one. When you are creating a bug report, please include as many details as possible:

* **Use a clear and descriptive title**
* **Describe the exact steps which reproduce the problem**
* **Provide specific examples to demonstrate the steps**
* **Describe the behavior you observed after following the steps**
* **Explain which behavior you expected to see instead and why**
* **Include screenshots and animated GIFs** if possible
* **Include your environment details** (OS, Go version, etc.)

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, please include:

* **Use a clear and descriptive title**
* **Provide a step-by-step description of the suggested enhancement**
* **Provide specific examples to demonstrate the steps**
* **Describe the current behavior** and **explain which behavior you expected to see instead**
* **Explain why this enhancement would be useful**

### Pull Requests

* Fill in the required template
* Do not include issue numbers in the PR title
* Follow the Go coding style
* Include thoughtfully-worded, well-structured tests
* Document new code
* End all files with a newline

## Development Process

### Setup Development Environment

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/goforge.git
cd goforge

# Add upstream remote
git remote add upstream https://github.com/Frank5ive/goforge.git

# Install dependencies
go mod download
```

### Making Changes

```bash
# Create a new branch
git checkout -b feature/my-new-feature

# Make your changes
# ... edit files ...

# Run tests (the end-to-end tests generate and build every option combination,
# run against SQLite, and on Linux with Docker also against PostgreSQL, MySQL, and
# MongoDB; add -short to skip them)
go test ./...

# Format and lint (https://golangci-lint.run, config in .golangci.yml)
go fmt ./...
go vet ./...
golangci-lint run

# Commit your changes
git commit -am 'Add some feature'

# Push to your fork
git push origin feature/my-new-feature
```

### Coding Standards

* Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
* Use meaningful variable and function names
* Write comments for exported functions
* Keep functions small and focused
* Write tests for new functionality
* Maintain test coverage above 80%

### Commit Messages

* Use the present tense ("Add feature" not "Added feature")
* Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
* Limit the first line to 72 characters or less
* Reference issues and pull requests liberally after the first line

Example:
```
Add database connection pooling

- Implement connection pool with configurable size
- Add retry logic for failed connections
- Update documentation

Closes #123
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage, measured as CI does (the badge in the README):
# across every package, including code the end-to-end tests exercise
go test -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1

# Run specific package tests
go test ./internal/domain/...

# Run tests verbosely
go test -v ./...
```

### Documentation

* Update README.md if you change functionality
* Add code comments for complex logic
* Update CHANGELOG.md for significant changes
* Include examples in documentation

## Project Structure

```
goforge/
├── cmd/                 # CLI commands
├── internal/
│   ├── domain/         # Business logic (no external deps)
│   ├── usecase/        # Application logic (depends on domain)
│   ├── infrastructure/ # External adapters (implements domain interfaces)
│   └── di/             # Dependency injection
├── templates/          # Code generation templates
├── tools/release/      # Release rules for the Release workflow (not part of the CLI)
└── main.go            # Entry point
```

### Layer Dependencies

* **Domain** → No dependencies
* **Usecase** → Domain only
* **Infrastructure** → Domain only
* **DI** → All layers
* **CLI/Main** → All layers

See [ARCHITECTURE.md](ARCHITECTURE.md) for how generation works, and for step-by-step
guides to adding a template or a new option.

## Releasing

Releases are automated: pushing a branch named `release/<version>` publishes that
version once every check passes.

1. Branch from an up-to-date `master`, named after the version, e.g. `release/1.2.0`
   (or `release/1.2.0-rc.1` for a prerelease).
2. In `CHANGELOG.md`, move the `[Unreleased]` entries under a new
   `## [1.2.0] - YYYY-MM-DD` heading and commit. That section becomes the release notes.
3. Push the branch: `git push origin release/1.2.0`.

The Release workflow (`.github/workflows/release.yml`) then:

- **checks the rules** (`tools/release`): the version must be valid
  [semver](https://semver.org), higher than every existing release, never released
  before, and have a non-empty `CHANGELOG.md` section. `go.mod` must be tidy and the
  GoReleaser config valid;
- **runs the full CI workflow** on the release commit (Linux, macOS, Windows, the
  minimum Go version, lint, and vulnerability checks);
- **publishes**: a complete GoReleaser dry run, then it tags the commit `v1.2.0`,
  uploads reproducible binaries for Linux, macOS, and Windows with checksums to a
  draft release, and publishes the draft only once everything is uploaded.
  Prereleases are marked as such and never become "latest".

If a check fails, fix it on the branch and push again. If publishing fails partway,
re-run the workflow: it resumes from the same commit, because a tag is never moved
and a published version is never replaced. Once a version is published, the next
release needs a higher version.

Don't push `v*` tags by hand; let the workflow create them.

To check the rules or the build locally:

```bash
go run ./tools/release check release/1.2.0 "$(git rev-parse HEAD)"
goreleaser release --snapshot --clean   # output goes to dist/
```

## Recording the demo

`docs/demo.tape` is a [VHS](https://github.com/charmbracelet/vhs) script that records
`generate` → `add resource` → `go test` to `docs/demo.gif`. VHS needs `ttyd` and
`ffmpeg`, and records whichever `goforge` is on your PATH:

```bash
go install . && vhs docs/demo.tape
```

Then show it near the top of `README.md`:
`<img src="docs/demo.gif" alt="GoForge generating a server and adding a resource" width="100%">`

## Questions?

Feel free to open an issue with your question.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

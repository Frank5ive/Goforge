# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

The first public release will be tagged `v0.1.0`.

### Added
- `goforge generate` command with interactive prompts (module path and a feature
  multi-select) and a non-interactive `--yes` mode
- `--name`, `--port`, and `--output` flags; app names may contain hyphens (e.g. `my-api`)
- `--module` to set the Go module path (defaults to the app name)
- `--logger`, `--docker`, and `--readme` flags; options set by flag are not prompted for
- `--dry-run` to list the files that would be created without writing anything
- `--git` to run `git init` in the new project
- Generated files are listed after generation
- A partially generated project is removed if generation fails
- Errors go to stderr with exit code 1; prompts fail fast with guidance when stdin is
  not a terminal
- Generated server with:
  - `/health` endpoint returning JSON
  - Graceful shutdown on SIGINT/SIGTERM with a 30s drain timeout
  - Panic recovery middleware and HTTP read/write/idle timeouts
  - `PORT` environment variable override
  - Optional logger with request logging middleware
  - Optional multi-stage Dockerfile with a health check
  - Optional README
  - `.gitignore`
- Generated Go code is always gofmt-formatted
- Templates embedded in the binary (single-file distribution)
- Terminal banner with a pixel-art Go gopher and wordmark; adapts to terminal width,
  respects `NO_COLOR`, and is skipped when output is piped
- End-to-end test that generates, vets, builds, runs, and shuts down every option combination

## Security

To report a vulnerability, please open a private security advisory on
[GitHub](https://github.com/Frank5ive/goforge/security/advisories/new) rather than a public issue.

[Unreleased]: https://github.com/Frank5ive/goforge/commits/master

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
- Generated server, using only the Go standard library, with:
  - `GET /health` endpoint returning JSON
  - Graceful shutdown on SIGINT/SIGTERM with a 30s drain timeout
  - Go 1.22+ routing patterns in `server/routes.go`
  - Request ID middleware (`X-Request-ID`) and panic recovery with stack traces
  - HTTP read, write, idle, and read-header timeouts
  - A `config` package that loads `PORT`, `CORS_ALLOWED_ORIGINS`, `MAX_BODY_BYTES`,
    and `SHUTDOWN_TIMEOUT` from the environment and reports every invalid value at once
  - Opt-in CORS middleware with preflight handling
  - Request body size limit (1 MiB by default) with `413` responses
  - Exported JSON helpers: `WriteJSON`, `WriteError`, and a strict `DecodeJSON`
  - Handler and middleware tests, a Makefile, and a golangci-lint v2 config
  - Optional structured logging with `log/slog` (`LOG_LEVEL`, `LOG_FORMAT`) and request logs
  - Optional multi-stage Dockerfile running as a non-root user, with a health check
  - Optional GitHub Actions CI workflow (`--ci`)
  - Optional README
  - `.gitignore`
- `goforge add resource <name> --fields "..."` adds a REST resource to a generated
  project: a package with the model and validation rules, a `Repository` interface
  with a thread-safe in-memory implementation, a service, CRUD handlers, and tests.
  Routes are registered in `server/routes.go` automatically, so the endpoints work as
  soon as the server starts. Supports `string`, `int`, `float`, `bool`, and `time`
  fields, optional strings (`name:string?`), `--plural` for irregular words, and
  `--dry-run`; a failed run leaves the project unchanged.
  - Paginated lists: `?limit=&offset=` returning `{items, total, limit, offset}`
    (default 20, max 100)
  - `PATCH /{resource}/{id}` for partial updates, validated like full updates
  - A `requests.http` file of example requests, pointed at the project's port
- `goforge completion bash|zsh|fish|powershell` for shell tab completion
- Generated projects have a `GET /readyz` readiness endpoint backed by `server.Deps`,
  which holds shared services such as a database pool
- Generated projects record their options and resources in `.goforge.json`, so
  `goforge add` commands can extend them safely
- `goforge add db <engine>` (or `goforge generate --db <engine>`) stores data in a
  database: PostgreSQL (`postgres`), MySQL/MariaDB (`mysql`), SQLite (`sqlite`), or
  MongoDB (`mongo`):
  - a `db` package that connects at startup and, for SQL databases, applies embedded
    migrations once each behind a lock (advisory lock, `GET_LOCK`, or SQLite's
    `BEGIN IMMEDIATE`)
  - `DATABASE_URL` in `config`, and `/readyz` checking the database
  - every existing resource converted to a database repository (with a migration for
    SQL databases); resources added later use the database automatically
  - shared repository tests that run against the in-memory and database
    repositories (SQLite always; others when `TEST_DATABASE_URL` is set)
  - `docker-compose.yml`, `make db-up`/`db-down`/`db-reset`/`test-db`, and a
    database service in the generated CI workflow (none needed for SQLite)
  - for MongoDB, the generated CI test job runs on `ubuntu-24.04`: MongoDB refuses
    to start on Linux 6.19 through 7.0.13, which newer `ubuntu-latest` images run
  - planned in full before writing, and rolled back completely (including `go.mod`)
    if any step fails
- Generated Go code is always gofmt-formatted
- Templates embedded in the binary (single-file distribution)
- Terminal banner with a pixel-art Go gopher and wordmark; adapts to terminal width,
  respects `NO_COLOR`, and is skipped when output is piped
- README logo (`docs/logo.svg`) generated from the same pixel art as the banner
- End-to-end test that generates every option combination and checks each one is
  gofmt-clean, passes `go vet` and its own tests, builds, serves `/health`, and shuts
  down gracefully
- CI on Linux, macOS, and Windows, plus the minimum Go version, golangci-lint, and
  govulncheck
- Release automation with GoReleaser: tagging `v*` publishes binaries for Linux,
  macOS, and Windows (amd64/arm64) with checksums
- `goforge --version` reports the release version, or the module version for
  `go install` builds
- Dependabot updates for Go modules and GitHub Actions, and a security policy

## Security

See [SECURITY.md](SECURITY.md) for how to report a vulnerability.

[Unreleased]: https://github.com/Frank5ive/goforge/commits/master

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Field rules in `--fields`: an `email` type, and `(min..max)` ranges for numbers and string
  lengths (`name:string(1..100)`, `price:float(0..)`). Each rule becomes a check in the generated
  `Validate` and gets its own generated test; sample values in the generated tests satisfy them
- `ref(<resource>)` fields link resources (`category_id:ref(category)`): an unknown ID is a
  `422`, and an item can't be deleted while others refer to it (`409`). The services are wired in
  `server/routes.go` and the checks run in the service, so they work on every database; every
  repository can count references, and the shared repository tests check it
- Each resource has a `rules.go`, the place for its own business rules, which `Validate` runs
  after the field rules
- `goforge generate --resource name:fields` (repeatable) creates resources in the same command;
  if adding the database or a resource fails, nothing is created
- `goforge --help` describes the whole workflow on one screen: every command, a one-command
  example, and the field syntax

### Changed
- When output is piped (scripts and AI agents), `generate` and `add` print a compact summary
  instead of every file: about 95% shorter. `--json` still has the full details
- `inspect` shows fields in the `--fields` syntax
- If adding the database fails during `generate --db`, the new project is removed instead of
  left half-made

## [0.2.0] - 2026-10-22

### Fixed
- Generated projects no longer hide the cause of unexpected errors: a 500 from a resource
  (for example a failed database query) is now logged with its cause, method, path, and request
  ID. Previously the error was discarded. A failed readiness check now logs its cause too.

### Added
- Error responses from the generated handlers include a `request_id` matching the `X-Request-ID`
  header and the server's log lines, so an error can be traced to its logs
- `APP_ENV` (`production` by default, or `development`): in development, 500 responses include
  their cause in `detail`; `make run` uses development
- Every log line written during a request carries its request ID; `handlers.Logger(ctx)` gives
  application code the same logger
- Panics return a JSON error with the request ID instead of a plain-text 500
- `--json` on `generate`, `add resource`, and `add db`: the result (files created and updated,
  endpoints, notes, and next steps) as JSON on stdout, for scripts and AI agents
- `goforge inspect` describes a project (module, Go version, features, database, resources and
  fields, routes, environment variables, and marker comments) as text or JSON, and reports
  anything that would stop `goforge add` from working, such as a deleted marker or file
- `goforge check` runs gofmt, `go vet`, the tests, and golangci-lint (when installed) and reports
  each problem with its file, line, and message (a failing test at the line that failed, a panic
  at the line that panicked), as text or JSON; tests and lint wait until the code compiles
- Generated projects include an `AGENTS.md` telling AI coding agents to use `goforge add`,
  `goforge inspect`, and `goforge check`, and the project's rules (where business logic goes,
  how errors and logging work, and which marker comments to keep)
- Failed commands report a stable error code (such as `not_a_goforge_project` or
  `marker_missing`) and a hint on how to fix the problem, in text and in JSON

### Changed
- `generate` no longer fails without a terminal: when input isn't a terminal (or with `--json`),
  it uses the flag values and defaults, as with `--yes`
- Error messages no longer start with "generation error at step [...]"
- `goforge --help` describes every command

## [0.1.0] - 2026-10-22

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
  - MongoDB containers set `GLIBC_TUNABLES=glibc.pthread.rseq=1`, so they start on
    Linux 6.19 through 7.0.13, which MongoDB otherwise refuses (SERVER-121912)
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

[Unreleased]: https://github.com/Frank5ive/goforge/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Frank5ive/goforge/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Frank5ive/goforge/releases/tag/v0.1.0

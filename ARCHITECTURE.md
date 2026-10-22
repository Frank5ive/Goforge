# GoForge Architecture

GoForge is a small CLI organized in Clean Architecture layers. Dependencies point
inward: `cmd` → `di` → `usecase` → `domain`, and `infrastructure` implements
interfaces defined in `domain`.

```
main.go                     Embeds templates/ and hands them to cmd
cmd/                        Cobra commands, prompts (huh), terminal banner
internal/
  di/                       Wires infrastructure into the usecase
  usecase/                  ServerGeneratorUsecase: validate → render → format → write
  domain/                   ServerConfig, validation, interfaces, error types
  infrastructure/
    template/               GoTemplateRenderer: parses every *.tmpl from an fs.FS
    filesystem/             FileSystemWriter (os) and RecordingWriter (records / dry run)
templates/                  text/template sources for generated projects
```

## How generation works

1. `cmd/generate.go` collects options from flags and, in a terminal, prompts for
   anything not set by flag. It builds a `domain.ServerConfig`.
2. `di.BuildContainer` parses all templates and wraps the filesystem writer in a
   `RecordingWriter` (which skips writes in `--dry-run` mode).
3. `ServerGeneratorUsecase.Generate` validates the config, refuses to touch an
   existing directory, then renders each entry of `projectFiles` whose `include`
   function matches the config. Go output is run through `go/format`, so invalid
   Go fails generation instead of being written. If any file fails, the partially
   generated project directory is removed.
4. The CLI prints the recorded file list and, with `--git`, runs `git init`.

## How `goforge add resource` works

`cmd/add.go` parses the name and `--fields` into a `domain.ResourceConfig`, which
derives the package, type, and route names (`blog_post` → package `blogposts`, type
`BlogPost`, path `/blog-posts`). `ResourceGeneratorUsecase.Add` then:

1. Validates the config and reads the module path from the project's `go.mod`.
2. Computes the new `server/routes.go` (import plus registration above the
   `// goforge:routes` marker) before writing anything, so a missing marker fails
   without side effects.
3. Renders the `templates/resource/*.tmpl` files into the new package, then writes
   the updated routes. If any step fails, the new package directory is removed.

`TestAddResource` in `main_test.go` adds resources covering every field type to a
generated project and checks that it vets, passes its tests, builds, and serves CRUD
requests.

## Extension points in generated projects

`goforge add` commands edit generated code only at marker comments, and record what
they did in the project's `.goforge.json` manifest:

| Marker | File | Used for |
|--------|------|----------|
| `// goforge:routes` | `server/routes.go` | Registering resource routes |
| `// goforge:deps-fields`, `// goforge:deps-ready` | `server/deps.go` | Shared services and their readiness checks |
| `// goforge:deps-connect` | `main.go` | Connecting shared services at startup |
| `// goforge:config-fields`, `// goforge:config-load` | `config/config.go` | New settings from the environment |

New code is inserted just above a marker, so repeated additions stack in order. If a
marker is missing (for example, the user deleted it), the command fails before writing
anything and explains what to add.

## Adding a file to generated projects

1. Add `templates/<name>.tmpl`. Template data is `*domain.ServerConfig`, so you can
   use fields like `{{.AppName}}`, `{{.Port}}`, `{{.IncludeLogger}}` and methods like
   `{{.GetModuleName}}`.
2. Add a line to `projectFiles` in `internal/usecase/server_generator.go` with the
   template name, output path, and when to include it.
3. Run `go test ./...`. `TestGeneratedProjects` in `main_test.go` generates every
   option combination from the embedded templates and checks that each one is
   gofmt-clean, passes `go vet`, builds, serves `/health`, and shuts down cleanly.

## Adding an option

1. Add a field to `domain.ServerConfig` (and validation if needed).
2. Add a flag in `newGenerateCmd`. For an on/off feature, add an entry to `features`
   in `cmd/generate.go` so it is also offered in the interactive prompt.
3. Use the field in templates or in a `projectFiles` `include` function, and extend
   the option matrix in `TestGeneratedProjects` if it changes generated Go code.

## How `goforge add db <engine>` works

Everything that differs between databases is described by an `engine` in
`internal/usecase/engines.go`: the driver module, the `db` package templates, the
per-resource repository templates, the SQL dialect (identifier quoting, placeholders,
column types), the `server.Deps` field and readiness check, default and test URLs, the
docker compose template, CI service, Makefile targets, and `.gitignore` entries.
Templates live in `templates/db/<engine>/`, with `templates/db/sql/` shared by the
`database/sql` engines (MySQL and SQLite).

`DatabaseGeneratorUsecase.Add` reads the project's `.goforge.json` to learn its options
and resources, then plans every change in a `changeset` before touching the disk:

1. Marker edits to `config/config.go` (`DatabaseURL`), `server/deps.go` (the connection
   and its readiness check), `main.go` (connecting at startup), and `server/routes.go`
   (switching each resource from `NewMemoryRepository()` to the engine's repository),
   plus Makefile targets, a CI service, and `.gitignore` entries where the engine has them.
2. New files: the engine's `db` package, `docker-compose.yml`, and, for each resource,
   its repository, tests, and (for SQL databases) a migration.
3. The manifest, now recording the database.

`changeset.apply` writes everything and then runs `go get` and `go mod tidy` (through the
`domain.ModuleManager` interface). If any step fails, it removes the files and directories
it created and restores every edited file, `go.mod`, and `go.sum`.

`add resource` uses the same changeset and, when the manifest records a database, also
plans the resource's repository files and migration. `generate --db <engine>` runs
`generate` and then the same `add db` usecase.

SQL statements are built by `addEngineData` from the dialect, with parameters numbered in
the order they appear, since `?` placeholders bind by position.

`TestAddDatabase` in `main_test.go` runs the whole flow for every engine: vet, the
project's tests against the real database, and data surviving a server restart. SQLite
runs everywhere; the others start a throwaway container and need Docker on Linux.

### Adding an engine

1. Add a template directory under `templates/db/` with the `db` package, repository, and
   repository test (reuse `templates/db/sql/` for a `database/sql` driver).
2. Describe it in `engines.go` and register it in `engines`.
3. Add its name (and any aliases) to `domain.DatabaseEngines`.
4. Add it to `databaseEngines` in `main_test.go` with a function that starts it.

## The banner and README logo

The terminal banner (`cmd/banner.go`) draws the gopher from the `gopherPixels` grid
using half-block characters, next to the GOFORGE wordmark. The README logo
(`docs/logo.svg`) is generated from the same grid and wordmark by
`renderLogoSVG` in `cmd/logo_test.go`, so the two always match.

After changing the gopher or wordmark, regenerate the logo:

```bash
go test ./cmd -run TestLogoSVGUpToDate -update-logo
```

`TestLogoSVGUpToDate` fails if the committed SVG is out of date.

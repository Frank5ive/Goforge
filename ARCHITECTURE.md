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

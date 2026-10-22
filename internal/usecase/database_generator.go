package usecase

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Frank5ive/goforge/internal/domain"
)

// pgx is the PostgreSQL driver generated projects use.
const (
	pgxModule  = "github.com/jackc/pgx/v5"
	pgxVersion = "v5.11.0"
)

// Markers in generated projects where `goforge add db` inserts code.
const (
	configFieldsMarker = "// goforge:config-fields"
	configLoadMarker   = "// goforge:config-load"
	depsFieldsMarker   = "// goforge:deps-fields"
	depsReadyMarker    = "// goforge:deps-ready"
	depsConnectMarker  = "// goforge:deps-connect"
	ciServicesMarker   = "# goforge:ci-test-services"
)

// databaseFiles are the files of the db package, relative to the project.
var databaseFiles = []struct {
	template string
	path     string
}{
	{"db/db.tmpl", "db/db.go"},
	{"db/migrate.tmpl", "db/migrate.go"},
	{"db/migrate_test.tmpl", "db/migrate_test.go"},
	{"db/migrations_readme.tmpl", "db/migrations/README.md"},
	{"db/dbtest.tmpl", "db/dbtest/dbtest.go"},
}

// DatabaseGeneratorUsecase adds a PostgreSQL database to existing generated
// projects. It implements the domain.DatabaseGenerator interface.
type DatabaseGeneratorUsecase struct {
	templateRenderer domain.TemplateRenderer
	fileWriter       domain.FileWriter
	modules          domain.ModuleManager
	now              func() time.Time
}

// NewDatabaseGeneratorUsecase creates a new instance of DatabaseGeneratorUsecase.
func NewDatabaseGeneratorUsecase(
	renderer domain.TemplateRenderer,
	writer domain.FileWriter,
	modules domain.ModuleManager,
) *DatabaseGeneratorUsecase {
	return &DatabaseGeneratorUsecase{
		templateRenderer: renderer,
		fileWriter:       writer,
		modules:          modules,
		now:              time.Now,
	}
}

// databaseData is the template data for database templates.
type databaseData struct {
	Module  string
	AppName string
	DBName  string // database name, e.g. "my_api"
	Port    int
	Docker  bool // the project has a Dockerfile, so docker-compose also runs the app
}

// Add implements domain.DatabaseGenerator. Every edit is computed before anything
// is written, and all files plus go.mod and go.sum are restored if any step
// fails, so a failed run leaves the project unchanged.
func (u *DatabaseGeneratorUsecase) Add(config *domain.DatabaseConfig) (domain.Changes, error) {
	if err := config.Validate(); err != nil {
		return domain.Changes{}, domain.NewGenerationError("validation", "database validation failed", err)
	}
	dir := config.ProjectDir

	modulePath, err := readModulePath(u.fileWriter, dir)
	if err != nil {
		return domain.Changes{}, domain.NewGenerationError("project_check",
			fmt.Sprintf("%s is not a Go module; run this inside a project created by goforge generate", dir), err)
	}

	manifest, err := u.readOrStartManifest(dir, modulePath)
	if err != nil {
		return domain.Changes{}, err
	}
	if manifest.Database != "" {
		return domain.Changes{}, domain.NewGenerationError("project_check",
			fmt.Sprintf("this project already uses a %s database", manifest.Database), nil)
	}

	data := databaseData{
		Module:  modulePath,
		AppName: manifest.AppName,
		DBName:  databaseName(manifest.AppName, modulePath),
		Port:    manifest.Port,
		Docker:  manifest.Features.Docker || u.fileWriter.FileExists(filepath.Join(dir, "Dockerfile")),
	}
	if data.Port == 0 {
		data.Port = fallbackPort
	}

	cs := newChangeset(u.fileWriter)
	if err := u.planEdits(cs, dir, data, manifest); err != nil {
		return domain.Changes{}, domain.NewGenerationError("project_check", "cannot add the database", err)
	}
	if err := u.planFiles(cs, dir, data, manifest); err != nil {
		return domain.Changes{}, domain.NewGenerationError("generate_files", "cannot add the database", err)
	}

	manifest.Database = domain.DatabasePostgres
	if err := planManifest(cs, dir, manifest); err != nil {
		return domain.Changes{}, domain.NewGenerationError("manifest", "cannot update "+domain.ManifestFile, err)
	}

	goFiles := []string{filepath.Join(dir, "go.mod"), filepath.Join(dir, "go.sum")}
	err = cs.apply(goFiles, func() error {
		if err := u.modules.Require(dir, pgxModule, pgxVersion); err != nil {
			return fmt.Errorf("adding %s: %w", pgxModule, err)
		}
		return nil
	})
	if err != nil {
		return domain.Changes{}, domain.NewGenerationError("apply", "the project was left unchanged", err)
	}

	return domain.Changes{Created: cs.createdIn(dir), Updated: cs.editedIn(dir), Notes: cs.notes}, nil
}

// readOrStartManifest returns the project's manifest, or a new one for older
// projects that have none, as long as they have no resources to convert.
func (u *DatabaseGeneratorUsecase) readOrStartManifest(dir, modulePath string) (domain.Manifest, error) {
	manifest, ok, err := readManifest(u.fileWriter, dir)
	if err != nil {
		return domain.Manifest{}, domain.NewGenerationError("project_check", "cannot read "+domain.ManifestFile, err)
	}
	if ok {
		return manifest, nil
	}

	routes, err := u.fileWriter.ReadFile(filepath.Join(dir, "server", "routes.go"))
	if err != nil {
		return domain.Manifest{}, domain.NewGenerationError("project_check",
			"server/routes.go not found; run this inside a project created by goforge generate", err)
	}
	if strings.Contains(routes, ".NewMemoryRepository()") {
		return domain.Manifest{}, domain.NewGenerationError("project_check",
			"this project has resources but no "+domain.ManifestFile+" (it was made by an older GoForge), so they can't be converted automatically", nil)
	}
	return domain.Manifest{Module: modulePath}, nil
}

// planEdits queues the changes to existing files: config, deps, main, routes,
// and (when present) the Makefile and CI workflow.
func (u *DatabaseGeneratorUsecase) planEdits(cs *changeset, dir string, data databaseData, manifest domain.Manifest) error {
	defaultURL := fmt.Sprintf("postgres://app:app@localhost:5432/%s?sslmode=disable", data.DBName)

	err := u.editGo(cs, dir, "config/config.go", func(src string) (string, error) {
		src, err := insertAboveMarker(src, configFieldsMarker, `// DatabaseURL is the PostgreSQL connection string (DATABASE_URL). The
// default matches the database started by `+"`make db-up`"+`.
DatabaseURL string
`)
		if err != nil {
			return "", err
		}
		return insertAboveMarker(src, configLoadMarker, fmt.Sprintf(`cfg.DatabaseURL = getenv("DATABASE_URL")
if cfg.DatabaseURL == "" {
	cfg.DatabaseURL = %q
}
`, defaultURL))
	})
	if err != nil {
		return err
	}

	err = u.editGo(cs, dir, "server/deps.go", func(src string) (string, error) {
		src, err := addImport(src, "github.com/jackc/pgx/v5/pgxpool")
		if err != nil {
			return "", err
		}
		if src, err = insertAboveMarker(src, depsFieldsMarker, "// DB is the PostgreSQL connection pool.\nDB *pgxpool.Pool\n"); err != nil {
			return "", err
		}
		if src, err = addImport(src, "errors"); err != nil {
			return "", err
		}
		return insertAboveMarker(src, depsReadyMarker, `if d.DB == nil {
	return errors.New("the database is not connected")
}
if err := d.DB.Ping(ctx); err != nil {
	return err
}
`)
	})
	if err != nil {
		return err
	}

	err = u.editGo(cs, dir, "main.go", func(src string) (string, error) {
		src, err := addImport(src, data.Module+"/db")
		if err != nil {
			return "", err
		}
		return insertAboveMarker(src, depsConnectMarker, `pool, err := db.Connect(ctx, cfg.DatabaseURL, log)
if err != nil {
	return err
}
defer pool.Close()
deps.DB = pool
`)
	})
	if err != nil {
		return err
	}

	err = u.editGo(cs, dir, "server/routes.go", func(src string) (string, error) {
		for _, r := range manifest.Resources {
			pkg := r.ResourceConfig(dir).PackageName()
			memory := pkg + ".NewMemoryRepository()"
			if !strings.Contains(src, memory) {
				return "", fmt.Errorf("server/routes.go no longer creates %s; register the resource with %s.NewPostgresRepository(deps.DB) yourself", memory, pkg)
			}
			src = strings.Replace(src, memory, pkg+".NewPostgresRepository(deps.DB)", 1)
		}
		return src, nil
	})
	if err != nil {
		return err
	}

	if err := u.editMakefile(cs, dir, data); err != nil {
		return err
	}
	return u.editCI(cs, dir, data)
}

// editGo queues an edit to a Go file in the project, formatting the result.
func (u *DatabaseGeneratorUsecase) editGo(cs *changeset, dir, rel string, change func(string) (string, error)) error {
	path := filepath.Join(dir, filepath.FromSlash(rel))
	src, err := u.fileWriter.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s not found; run this inside a project created by goforge generate", rel)
	}

	src, err = change(src)
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	if src, err = formatGo(rel, src); err != nil {
		return err
	}
	cs.edit(path, src)
	return nil
}

// editMakefile appends database targets to the project's Makefile, if it has one.
func (u *DatabaseGeneratorUsecase) editMakefile(cs *changeset, dir string, data databaseData) error {
	path := filepath.Join(dir, "Makefile")
	if !u.fileWriter.FileExists(path) {
		return nil
	}
	makefile, err := u.fileWriter.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(makefile, "db-up:") {
		return nil
	}

	testURL := fmt.Sprintf("postgres://app:app@localhost:5432/%s_test?sslmode=disable", data.DBName)
	targets := strings.Join([]string{
		"",
		".PHONY: db-up db-down db-reset test-db",
		"",
		"db-up: ## Start PostgreSQL in Docker",
		"\tdocker compose up -d --wait db",
		"",
		"db-down: ## Stop PostgreSQL (data is kept)",
		"\tdocker compose down",
		"",
		"db-reset: ## Delete all data and start a fresh PostgreSQL",
		"\tdocker compose down -v",
		"\tdocker compose up -d --wait db",
		"",
		"test-db: db-up ## Run all tests, including the PostgreSQL ones",
		fmt.Sprintf("\t-docker compose exec -T db createdb -U app %s_test 2>/dev/null", data.DBName),
		fmt.Sprintf("\tTEST_DATABASE_URL=%q go test ./...", testURL),
		"",
	}, "\n")

	cs.edit(path, strings.TrimRight(makefile, "\n")+"\n"+targets)
	return nil
}

// editCI adds a PostgreSQL service and TEST_DATABASE_URL to the test job of the
// project's CI workflow, if it has one with the marker.
func (u *DatabaseGeneratorUsecase) editCI(cs *changeset, dir string, data databaseData) error {
	path := filepath.Join(dir, ".github", "workflows", "ci.yml")
	if !u.fileWriter.FileExists(path) {
		return nil
	}
	workflow, err := u.fileWriter.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(workflow, ciServicesMarker) {
		cs.note("Your CI workflow was customized, so it was left alone. To run the PostgreSQL tests in CI, add a postgres service and set TEST_DATABASE_URL in the test job.")
		return nil
	}

	testDB := data.DBName + "_test"
	updated, err := insertAboveMarker(workflow, ciServicesMarker, fmt.Sprintf(`services:
  postgres:
    image: postgres:18-alpine
    env:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: app
      POSTGRES_DB: %[1]s
    ports:
      - 5432:5432
    options: >-
      --health-cmd "pg_isready -U app -d %[1]s"
      --health-interval 2s
      --health-timeout 3s
      --health-retries 15
env:
  TEST_DATABASE_URL: postgres://app:app@localhost:5432/%[1]s?sslmode=disable
`, testDB))
	if err != nil {
		return err
	}
	cs.edit(path, updated)
	return nil
}

// planFiles queues the db package, docker-compose.yml, and the PostgreSQL files
// for every existing resource.
func (u *DatabaseGeneratorUsecase) planFiles(cs *changeset, dir string, data databaseData, manifest domain.Manifest) error {
	for _, f := range databaseFiles {
		if err := planTemplate(u.templateRenderer, cs, filepath.Join(dir, filepath.FromSlash(f.path)), f.template, data); err != nil {
			return err
		}
	}

	composePath := filepath.Join(dir, "docker-compose.yml")
	if u.fileWriter.FileExists(composePath) {
		cs.note("docker-compose.yml already exists, so it was left alone. Add a PostgreSQL service named db for `make db-up` to work.")
	} else if err := planTemplate(u.templateRenderer, cs, composePath, "compose.tmpl", data); err != nil {
		return err
	}

	version := u.now().UTC()
	for _, r := range manifest.Resources {
		resource := r.ResourceConfig(dir)
		resourceData := newResourceData(resource, data.Module)
		resourceData.Port = data.Port

		// Resources from before Phase 3 lack the shared repository tests
		pkgDir := filepath.Join(dir, resource.PackageName())
		if !u.fileWriter.FileExists(filepath.Join(pkgDir, "repository_test.go")) {
			if err := planTemplate(u.templateRenderer, cs, filepath.Join(pkgDir, "repository_test.go"), "resource/repository_test.tmpl", resourceData); err != nil {
				return err
			}
		}

		if err := planPostgresResource(u.templateRenderer, cs, dir, resource, resourceData, version); err != nil {
			return err
		}
		// Keep migration versions unique and in resource order
		version = version.Add(time.Second)
	}
	return nil
}

// planPostgresResource queues a resource's PostgreSQL repository, its tests, and
// the migration that creates its table.
func planPostgresResource(renderer domain.TemplateRenderer, cs *changeset, dir string, resource *domain.ResourceConfig, data resourceData, version time.Time) error {
	pkgDir := filepath.Join(dir, resource.PackageName())
	for _, f := range postgresFiles {
		if err := planTemplate(renderer, cs, filepath.Join(pkgDir, f.file), f.template, data); err != nil {
			return err
		}
	}

	migration := fmt.Sprintf("%s_create_%s.sql", version.Format("20060102150405"), resource.PluralName())
	return planTemplate(renderer, cs, filepath.Join(dir, "db", "migrations", migration), "resource/migration.tmpl", data)
}

// planTemplate renders a template and queues it as a new file, formatting Go.
func planTemplate(renderer domain.TemplateRenderer, cs *changeset, path, template string, data any) error {
	content, err := renderer.Render(template, data)
	if err != nil {
		return err
	}
	if filepath.Ext(path) == ".go" {
		if content, err = formatGo(filepath.Base(path), content); err != nil {
			return err
		}
	}
	return cs.create(path, content)
}

// planManifest queues the updated manifest.
func planManifest(cs *changeset, dir string, manifest domain.Manifest) error {
	content, err := marshalManifest(manifest)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, domain.ManifestFile)
	if cs.writer.FileExists(path) {
		cs.edit(path, content)
		return nil
	}
	return cs.create(path, content)
}

// databaseName derives a PostgreSQL database name from the app name (or the
// last element of the module path): lowercase, with hyphens as underscores.
func databaseName(appName, modulePath string) string {
	name := appName
	if name == "" {
		name = modulePath[strings.LastIndex(modulePath, "/")+1:]
	}
	name = strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	if name == "" {
		return "app"
	}
	return name
}

// readModulePath returns the module path declared in the project's go.mod.
func readModulePath(w domain.FileWriter, projectDir string) (string, error) {
	goMod, err := w.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(goMod, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`), nil
		}
	}
	return "", errors.New("go.mod has no module directive")
}

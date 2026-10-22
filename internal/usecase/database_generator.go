package usecase

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Frank5ive/goforge/internal/domain"
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

// DatabaseGeneratorUsecase adds a database to existing generated projects. It
// implements the domain.DatabaseGenerator interface.
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
	Label   string
}

// Add implements domain.DatabaseGenerator. Every edit is computed before anything
// is written, and all files plus go.mod and go.sum are restored if any step
// fails, so a failed run leaves the project unchanged.
func (u *DatabaseGeneratorUsecase) Add(config *domain.DatabaseConfig) (domain.Changes, error) {
	if err := config.Validate(); err != nil {
		return domain.Changes{}, domain.NewGenerationError("validation", "database validation failed", err)
	}
	eng, err := engineFor(config.Engine)
	if err != nil {
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
		Label:   eng.label,
	}
	if data.Port == 0 {
		data.Port = fallbackPort
	}

	cs := newChangeset(u.fileWriter)
	if err := u.planEdits(cs, dir, eng, data, manifest); err != nil {
		return domain.Changes{}, domain.NewGenerationError("project_check", "cannot add the database", err)
	}
	if err := u.planFiles(cs, dir, eng, data, manifest); err != nil {
		return domain.Changes{}, domain.NewGenerationError("generate_files", "cannot add the database", err)
	}

	manifest.Database = eng.name
	if err := planManifest(cs, dir, manifest); err != nil {
		return domain.Changes{}, domain.NewGenerationError("manifest", "cannot update "+domain.ManifestFile, err)
	}

	goFiles := []string{filepath.Join(dir, "go.mod"), filepath.Join(dir, "go.sum")}
	err = cs.apply(goFiles, func() error {
		if err := u.modules.Require(dir, eng.module, eng.version); err != nil {
			return fmt.Errorf("adding %s: %w", eng.module, err)
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
// and (when present) the Makefile, CI workflow, and .gitignore.
func (u *DatabaseGeneratorUsecase) planEdits(cs *changeset, dir string, eng *engine, data databaseData, manifest domain.Manifest) error {
	err := u.editGo(cs, dir, "config/config.go", func(src string) (string, error) {
		src, err := insertAboveMarker(src, configFieldsMarker, fmt.Sprintf(`// DatabaseURL is the %s connection string (DATABASE_URL). The
// default matches the local development database.
DatabaseURL string
`, eng.label))
		if err != nil {
			return "", err
		}
		return insertAboveMarker(src, configLoadMarker, fmt.Sprintf(`cfg.DatabaseURL = getenv("DATABASE_URL")
if cfg.DatabaseURL == "" {
	cfg.DatabaseURL = %q
}
`, eng.defaultURL(data.DBName)))
	})
	if err != nil {
		return err
	}

	err = u.editGo(cs, dir, "server/deps.go", func(src string) (string, error) {
		src, err := addImport(src, eng.depsImport)
		if err != nil {
			return "", err
		}
		if src, err = addImport(src, "errors"); err != nil {
			return "", err
		}
		if src, err = insertAboveMarker(src, depsFieldsMarker, fmt.Sprintf("// DB is the %s database.\nDB %s\n", eng.label, eng.depsType)); err != nil {
			return "", err
		}
		return insertAboveMarker(src, depsReadyMarker, fmt.Sprintf(`if d.DB == nil {
	return errors.New("the database is not connected")
}
if err := %s; err != nil {
	return err
}
`, eng.readyCheck))
	})
	if err != nil {
		return err
	}

	err = u.editGo(cs, dir, "main.go", func(src string) (string, error) {
		src, err := addImport(src, data.Module+"/db")
		if err != nil {
			return "", err
		}
		return insertAboveMarker(src, depsConnectMarker, connectCode)
	})
	if err != nil {
		return err
	}

	err = u.editGo(cs, dir, "server/routes.go", func(src string) (string, error) {
		for _, r := range manifest.Resources {
			pkg := r.ResourceConfig(dir).PackageName()
			memory := pkg + ".NewMemoryRepository()"
			repo := fmt.Sprintf("%s.New%s(deps.DB)", pkg, eng.repoType)
			if !strings.Contains(src, memory) {
				return "", fmt.Errorf("server/routes.go no longer creates %s; register the resource with %s yourself", memory, repo)
			}
			src = strings.Replace(src, memory, repo, 1)
		}
		return src, nil
	})
	if err != nil {
		return err
	}

	if err := u.editMakefile(cs, dir, eng, data); err != nil {
		return err
	}
	if err := u.editCI(cs, dir, eng, data); err != nil {
		return err
	}
	return u.editGitignore(cs, dir, eng)
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

// editMakefile appends the engine's database targets to the project's Makefile,
// if it has one.
func (u *DatabaseGeneratorUsecase) editMakefile(cs *changeset, dir string, eng *engine, data databaseData) error {
	path := filepath.Join(dir, "Makefile")
	if !u.fileWriter.FileExists(path) {
		return nil
	}
	makefile, err := u.fileWriter.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(makefile, "db-reset:") {
		return nil
	}

	testURL := ""
	if eng.testURL != nil {
		testURL = eng.testURL(data.DBName)
	}
	targets := eng.makeTargets(data.DBName, testURL)
	block := append([]string{"", ".PHONY: " + strings.Join(phonyTargets(targets), " "), ""}, targets...)

	cs.edit(path, strings.TrimRight(makefile, "\n")+"\n"+strings.Join(block, "\n")+"\n")
	return nil
}

// editCI adds the engine's database service and TEST_DATABASE_URL to the test
// job of the project's CI workflow, if it has one with the marker.
func (u *DatabaseGeneratorUsecase) editCI(cs *changeset, dir string, eng *engine, data databaseData) error {
	path := filepath.Join(dir, ".github", "workflows", "ci.yml")
	if eng.ciService == nil || !u.fileWriter.FileExists(path) {
		return nil
	}
	workflow, err := u.fileWriter.ReadFile(path)
	if err != nil {
		return err
	}
	if !strings.Contains(workflow, ciServicesMarker) {
		cs.note("Your CI workflow was customized, so it was left alone. To run the %s tests in CI, add a %s service and set TEST_DATABASE_URL in the test job.", eng.label, eng.label)
		return nil
	}

	updated, err := insertAboveMarker(workflow, ciServicesMarker, eng.ciService(data.DBName+"_test", eng.testURL(data.DBName)))
	if err != nil {
		return err
	}
	cs.edit(path, updated)
	return nil
}

// editGitignore adds the engine's ignore patterns to the project's .gitignore.
func (u *DatabaseGeneratorUsecase) editGitignore(cs *changeset, dir string, eng *engine) error {
	path := filepath.Join(dir, ".gitignore")
	if len(eng.gitignore) == 0 || !u.fileWriter.FileExists(path) {
		return nil
	}
	gitignore, err := u.fileWriter.ReadFile(path)
	if err != nil {
		return err
	}

	block := "\n# " + eng.label + " database files\n" + strings.Join(eng.gitignore, "\n") + "\n"
	cs.edit(path, strings.TrimRight(gitignore, "\n")+"\n"+block)
	return nil
}

// planFiles queues the db package, docker-compose.yml, and the database files
// for every existing resource.
func (u *DatabaseGeneratorUsecase) planFiles(cs *changeset, dir string, eng *engine, data databaseData, manifest domain.Manifest) error {
	for _, f := range eng.dbFiles {
		if err := planTemplate(u.templateRenderer, cs, filepath.Join(dir, filepath.FromSlash(f.path)), f.template, data); err != nil {
			return err
		}
	}

	if eng.composeTemplate != "" {
		composePath := filepath.Join(dir, "docker-compose.yml")
		if u.fileWriter.FileExists(composePath) {
			cs.note("docker-compose.yml already exists, so it was left alone. Add a %s service named db for `make db-up` to work.", eng.label)
		} else if err := planTemplate(u.templateRenderer, cs, composePath, eng.composeTemplate, data); err != nil {
			return err
		}
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

		if err := planDatabaseResource(u.templateRenderer, cs, dir, eng, resource, resourceData, version); err != nil {
			return err
		}
		// Keep migration versions unique and in resource order
		version = version.Add(time.Second)
	}
	return nil
}

// planDatabaseResource queues a resource's repository for eng, its tests, and,
// for SQL databases, the migration that creates its table.
func planDatabaseResource(renderer domain.TemplateRenderer, cs *changeset, dir string, eng *engine, resource *domain.ResourceConfig, data resourceData, version time.Time) error {
	addEngineData(&data, eng, resource.PluralName())

	pkgDir := filepath.Join(dir, resource.PackageName())
	for _, f := range eng.repositoryFiles {
		if err := planTemplate(renderer, cs, filepath.Join(pkgDir, f.path), f.template, data); err != nil {
			return err
		}
	}

	if eng.dialect == nil {
		return nil
	}
	migration := fmt.Sprintf("%s_create_%s.sql", version.Format("20060102150405"), resource.PluralName())
	return planTemplate(renderer, cs, filepath.Join(dir, "db", "migrations", migration), "db/migration.tmpl", data)
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

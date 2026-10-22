package usecase

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Frank5ive/goforge/internal/domain"
)

// ProjectInspectorUsecase implements domain.ProjectInspector from a project's
// .goforge.json, go.mod, and the files `goforge add` edits.
type ProjectInspectorUsecase struct {
	fileWriter domain.FileWriter
}

// NewProjectInspectorUsecase returns a ProjectInspectorUsecase that reads files
// with fileWriter.
func NewProjectInspectorUsecase(fileWriter domain.FileWriter) *ProjectInspectorUsecase {
	return &ProjectInspectorUsecase{fileWriter: fileWriter}
}

// marker is a comment `goforge add` inserts code above.
type marker struct {
	file, marker, usedBy string
	// needed reports whether a missing marker stops the command from working,
	// given the project's manifest
	needed func(domain.Manifest) bool
}

var markers = []marker{
	{"server/routes.go", routesMarker, "add resource", func(domain.Manifest) bool { return true }},
	{"config/config.go", configFieldsMarker, "add db", noDatabase},
	{"config/config.go", configLoadMarker, "add db", noDatabase},
	{"server/deps.go", depsFieldsMarker, "add db", noDatabase},
	{"server/deps.go", depsReadyMarker, "add db", noDatabase},
	{"main.go", depsConnectMarker, "add db", noDatabase},
	// Without it, add db still works and only notes that CI needs a database
	{".github/workflows/ci.yml", ciServicesMarker, "add db", func(domain.Manifest) bool { return false }},
}

func noDatabase(m domain.Manifest) bool { return m.Database == "" }

// Inspect implements domain.ProjectInspector.
func (u *ProjectInspectorUsecase) Inspect(dir string) (domain.ProjectInfo, error) {
	goMod, err := u.fileWriter.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return domain.ProjectInfo{}, domain.WithCode(domain.CodeNotAProject,
			fmt.Errorf("%s is not a Go module; run this inside a project created by goforge generate: %w", dir, err))
	}
	manifest, ok, err := readManifest(u.fileWriter, dir)
	if err != nil {
		return domain.ProjectInfo{}, err
	}
	if !ok {
		return domain.ProjectInfo{}, domain.WithCode(domain.CodeNotAProject,
			fmt.Errorf("%s has no %s, so it was not created by goforge generate", dir, domain.ManifestFile))
	}

	info := domain.ProjectInfo{
		AppName:   manifest.AppName,
		Module:    manifest.Module,
		GoVersion: goDirective(goMod),
		Port:      manifest.Port,
		Features:  manifest.Features,
		Database:  manifest.Database,
		Resources: []domain.ResourceInfo{},
		Routes:    []domain.Route{{Method: "GET", Path: "/health"}, {Method: "GET", Path: "/readyz"}},
		Env:       projectEnv(manifest),
		Problems:  []string{},
	}

	for _, r := range manifest.Resources {
		config := r.ResourceConfig(dir)
		res := domain.ResourceInfo{
			Name:    r.Name,
			Plural:  r.Plural,
			Package: config.PackageName(),
			Path:    config.RoutePath(),
			Fields:  r.Fields,
		}
		info.Resources = append(info.Resources, res)
		for _, route := range []struct{ method, path string }{
			{"GET", res.Path}, {"POST", res.Path}, {"GET", res.Path + "/{id}"},
			{"PUT", res.Path + "/{id}"}, {"PATCH", res.Path + "/{id}"}, {"DELETE", res.Path + "/{id}"},
		} {
			info.Routes = append(info.Routes, domain.Route{Method: route.method, Path: route.path, Resource: r.Name})
		}
		if !u.fileWriter.FileExists(filepath.Join(dir, res.Package)) {
			info.Problems = append(info.Problems, fmt.Sprintf(
				"%s lists the %s resource, but its %s/ package is missing", domain.ManifestFile, r.Name, res.Package))
		}
	}

	for _, m := range markers {
		src, err := u.fileWriter.ReadFile(filepath.Join(dir, filepath.FromSlash(m.file)))
		if err != nil {
			// An optional file the project was generated without, such as the CI workflow
			continue
		}
		present := strings.Contains(src, m.marker)
		info.Markers = append(info.Markers, domain.MarkerInfo{File: m.file, Marker: m.marker, Present: present, UsedBy: m.usedBy})
		if !present && m.needed(manifest) {
			info.Problems = append(info.Problems, fmt.Sprintf(
				"%s lost its %q marker, so `goforge %s` can't update it; restore the comment", m.file, m.marker, m.usedBy))
		}
	}

	return info, nil
}

// goDirective returns the version in go.mod's go directive, e.g. "1.24.0".
func goDirective(goMod string) string {
	for line := range strings.Lines(goMod) {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	return ""
}

// projectEnv returns the environment variables a generated server reads.
func projectEnv(m domain.Manifest) []string {
	env := []string{"APP_ENV", "PORT", "CORS_ALLOWED_ORIGINS", "MAX_BODY_BYTES", "SHUTDOWN_TIMEOUT"}
	if m.Features.Logger {
		env = append(env, "LOG_LEVEL", "LOG_FORMAT")
	}
	if m.Database != "" {
		env = append(env, "DATABASE_URL")
	}
	return env
}

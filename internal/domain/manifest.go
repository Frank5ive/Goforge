package domain

// ManifestFile is the name of the project manifest in a generated project's root.
const ManifestFile = ".goforge.json"

// Manifest records how a project was generated and what has been added to it,
// so later `goforge add` commands know its options and resources without
// parsing its code.
type Manifest struct {
	AppName   string             `json:"app_name,omitempty"`
	Module    string             `json:"module"`
	Port      int                `json:"port,omitempty"`
	Features  ManifestFeatures   `json:"features"`
	Database  string             `json:"database,omitempty"`
	Resources []ManifestResource `json:"resources,omitempty"`
}

// ManifestFeatures records the optional features chosen at generation time.
type ManifestFeatures struct {
	Logger bool `json:"logger"`
	Docker bool `json:"docker"`
	Readme bool `json:"readme"`
	CI     bool `json:"ci"`
}

// ManifestResource records a resource added with `goforge add resource`.
type ManifestResource struct {
	Name   string  `json:"name"`
	Plural string  `json:"plural"`
	Fields []Field `json:"fields"`
}

// NewManifest returns the manifest for a newly generated project.
func NewManifest(config *ServerConfig) Manifest {
	return Manifest{
		AppName: config.AppName,
		Module:  config.GetModuleName(),
		Port:    config.Port,
		Features: ManifestFeatures{
			Logger: config.IncludeLogger,
			Docker: config.IncludeDocker,
			Readme: config.IncludeReadme,
			CI:     config.IncludeCI,
		},
	}
}

// AddResource records a resource in the manifest.
func (m *Manifest) AddResource(config *ResourceConfig) {
	m.Resources = append(m.Resources, ManifestResource{
		Name:   config.Name,
		Plural: config.PluralName(),
		Fields: config.Fields,
	})
}

// ResourceConfig returns the configuration to regenerate a recorded resource
// in the project at projectDir.
func (r ManifestResource) ResourceConfig(projectDir string) *ResourceConfig {
	return &ResourceConfig{
		ProjectDir: projectDir,
		Name:       r.Name,
		Plural:     r.Plural,
		Fields:     r.Fields,
	}
}

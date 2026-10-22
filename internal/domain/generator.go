package domain

// ServerGenerator defines the contract for generating server projects.
// This is the main use case interface that orchestrates the generation process.
type ServerGenerator interface {
	// Generate creates a new server project based on the provided configuration.
	// It returns an error if generation fails at any step.
	Generate(config *ServerConfig) error
}

// ResourceGenerator defines the contract for adding a REST resource to an
// existing generated project.
type ResourceGenerator interface {
	// Add generates the resource package and registers its routes.
	Add(config *ResourceConfig) error
}

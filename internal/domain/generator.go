package domain

// ServerGenerator defines the contract for generating server projects.
// This is the main use case interface that orchestrates the generation process.
type ServerGenerator interface {
	// Generate creates a new server project based on the provided configuration.
	// It returns an error if generation fails at any step.
	Generate(config *ServerConfig) error
}

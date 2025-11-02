package domain

// TemplateRenderer defines the contract for rendering Go templates.
// This abstracts the template engine implementation from the domain layer.
type TemplateRenderer interface {
	// Render processes a template with the given data and returns the result.
	// templateName: name/path of the template file
	// data: the data structure to pass to the template
	Render(templateName string, data interface{}) (string, error)
}

// FileWriter defines the contract for filesystem operations.
// This abstracts file and directory creation from the domain layer.
type FileWriter interface {
	// WriteFile writes content to a file at the specified path.
	// It creates parent directories if they don't exist.
	WriteFile(path string, content string) error
	
	// CreateDirectory creates a directory at the specified path.
	// It creates parent directories if they don't exist.
	CreateDirectory(path string) error
	
	// FileExists checks if a file or directory exists at the path.
	FileExists(path string) bool
}

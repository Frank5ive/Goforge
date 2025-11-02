package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// ServerConfig represents the configuration for a generated server project.
// It encapsulates all user preferences and settings for server generation.
type ServerConfig struct {
	AppName       string
	Port          int
	OutputDir     string
	IncludeLogger bool
	IncludeDocker bool
	IncludeReadme bool
}

// Validate performs business rule validation on the ServerConfig.
// It ensures the configuration meets domain requirements before generation.
func (c *ServerConfig) Validate() error {
	if err := c.validateAppName(); err != nil {
		return err
	}
	
	if err := c.validatePort(); err != nil {
		return err
	}
	
	if err := c.validateOutputDir(); err != nil {
		return err
	}
	
	return nil
}

// validateAppName ensures the application name follows Go package naming conventions.
func (c *ServerConfig) validateAppName() error {
	if c.AppName == "" {
		return NewValidationError("app name cannot be empty")
	}
	
	if len(c.AppName) > 50 {
		return NewValidationError("app name cannot exceed 50 characters")
	}
	
	// Go package naming convention: lowercase letters, numbers, and underscores
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9_]*$`, c.AppName)
	if !matched {
		return NewValidationError("app name must start with a lowercase letter and contain only lowercase letters, numbers, and underscores")
	}
	
	// Reserved Go keywords
	reservedKeywords := []string{
		"break", "case", "chan", "const", "continue",
		"default", "defer", "else", "fallthrough", "for",
		"func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return",
		"select", "struct", "switch", "type", "var",
	}
	
	for _, keyword := range reservedKeywords {
		if c.AppName == keyword {
			return NewValidationError(fmt.Sprintf("app name cannot be a Go reserved keyword: %s", keyword))
		}
	}
	
	return nil
}

// validatePort ensures the port number is within valid range.
func (c *ServerConfig) validatePort() error {
	if c.Port < 1024 {
		return NewValidationError("port must be >= 1024 (ports below 1024 require root privileges)")
	}
	
	if c.Port > 65535 {
		return NewValidationError("port must be <= 65535")
	}
	
	return nil
}

// validateOutputDir ensures the output directory is specified.
func (c *ServerConfig) validateOutputDir() error {
	if strings.TrimSpace(c.OutputDir) == "" {
		return NewValidationError("output directory cannot be empty")
	}
	
	return nil
}

// GetModuleName returns the Go module name based on app name.
// This follows Go module naming conventions.
func (c *ServerConfig) GetModuleName() string {
	return fmt.Sprintf("github.com/yourorg/%s", c.AppName)
}

// GetProjectPath returns the full path where the project will be generated.
func (c *ServerConfig) GetProjectPath() string {
	return fmt.Sprintf("%s/%s", c.OutputDir, c.AppName)
}

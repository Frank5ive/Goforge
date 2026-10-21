package domain

import (
	"testing"
)

func TestServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ServerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: ServerConfig{
				AppName:       "myapp",
				Port:          8080,
				OutputDir:     "/tmp",
				IncludeLogger: true,
				IncludeDocker: true,
				IncludeReadme: true,
			},
			wantErr: false,
		},
		{
			name: "empty app name",
			config: ServerConfig{
				AppName:   "",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "app name cannot be empty",
		},
		{
			name: "app name too long",
			config: ServerConfig{
				AppName:   "this_is_a_very_long_application_name_that_exceeds_fifty_characters",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "app name cannot exceed 50 characters",
		},
		{
			name: "app name with uppercase",
			config: ServerConfig{
				AppName:   "MyApp",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "must start with a lowercase letter",
		},
		{
			name: "app name with special characters",
			config: ServerConfig{
				AppName:   "my.app",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "must start with a lowercase letter",
		},
		{
			name: "app name with spaces",
			config: ServerConfig{
				AppName:   "my app",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "must start with a lowercase letter",
		},
		{
			name: "app name ending in hyphen",
			config: ServerConfig{
				AppName:   "my-app-",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "end with a letter or number",
		},
		{
			name: "valid app name with hyphens",
			config: ServerConfig{
				AppName:   "user-service",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: false,
		},
		{
			name: "valid single letter app name",
			config: ServerConfig{
				AppName:   "a",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: false,
		},
		{
			name: "app name is reserved keyword",
			config: ServerConfig{
				AppName:   "package",
				Port:      8080,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "cannot be a Go reserved keyword",
		},
		{
			name: "port too low",
			config: ServerConfig{
				AppName:   "myapp",
				Port:      80,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "port must be >= 1024",
		},
		{
			name: "port too high",
			config: ServerConfig{
				AppName:   "myapp",
				Port:      99999,
				OutputDir: "/tmp",
			},
			wantErr: true,
			errMsg:  "port must be <= 65535",
		},
		{
			name: "empty output directory",
			config: ServerConfig{
				AppName:   "myapp",
				Port:      8080,
				OutputDir: "",
			},
			wantErr: true,
			errMsg:  "output directory cannot be empty",
		},
		{
			name: "valid app name with underscores and numbers",
			config: ServerConfig{
				AppName:   "my_app_123",
				Port:      3000,
				OutputDir: "./output",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ServerConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ServerConfig.Validate() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestServerConfig_GetModuleName(t *testing.T) {
	config := ServerConfig{
		AppName: "testapp",
	}

	expected := "github.com/yourorg/testapp"
	got := config.GetModuleName()

	if got != expected {
		t.Errorf("GetModuleName() = %v, want %v", got, expected)
	}
}

func TestServerConfig_GetProjectPath(t *testing.T) {
	config := ServerConfig{
		AppName:   "testapp",
		OutputDir: "/home/user/projects",
	}

	expected := "/home/user/projects/testapp"
	got := config.GetProjectPath()

	if got != expected {
		t.Errorf("GetProjectPath() = %v, want %v", got, expected)
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		expected string
	}{
		{
			name:     "error without field",
			err:      NewValidationError("invalid value"),
			expected: "validation error: invalid value",
		},
		{
			name:     "error with field",
			err:      NewValidationErrorWithField("port", "must be greater than 1024"),
			expected: "validation error [port]: must be greater than 1024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenerationError_Error(t *testing.T) {
	baseErr := NewValidationError("invalid config")
	err := NewGenerationError("template_rendering", "failed to render template", baseErr)

	expected := "generation error at step [template_rendering]: failed to render template"
	got := err.Error()

	if !containsString(got, expected) {
		t.Errorf("Error() = %v, expected to contain %v", got, expected)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

package domain

import (
	"path/filepath"
	"strings"
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
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ServerConfig.Validate() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestServerConfig_GetModuleName(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		want       string
	}{
		{"defaults to app name", "", "testapp"},
		{"uses explicit module path", "github.com/acme/testapp", "github.com/acme/testapp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ServerConfig{AppName: "testapp", ModulePath: tt.modulePath}

			if got := config.GetModuleName(); got != tt.want {
				t.Errorf("GetModuleName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServerConfig_ValidateModulePath(t *testing.T) {
	tests := []struct {
		modulePath string
		wantErr    bool
	}{
		{"github.com/acme/my-api", false},
		{"example.com/team/service_v2", false},
		{"my-api", false},
		{"github.com/acme/my api", true},
		{"/github.com/acme", true},
		{"github.com//acme", true},
		{"github.com/acme/", true},
		{"github.com/.hidden", true},
		{"github.com/acme/api!", true},
	}

	for _, tt := range tests {
		t.Run(tt.modulePath, func(t *testing.T) {
			config := ServerConfig{AppName: "myapp", ModulePath: tt.modulePath, Port: 8080, OutputDir: "."}

			err := config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestServerConfig_GetProjectPath(t *testing.T) {
	config := ServerConfig{
		AppName:   "testapp",
		OutputDir: "/home/user/projects",
	}

	expected := filepath.Join("/home/user/projects", "testapp")
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

	if !strings.Contains(got, expected) {
		t.Errorf("Error() = %v, expected to contain %v", got, expected)
	}
}

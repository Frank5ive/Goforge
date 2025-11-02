# Domain Layer

The **Domain Layer** is the core of the application, containing business logic, entities, and contracts (interfaces). It is completely independent of external frameworks, databases, or infrastructure concerns.

## Structure

```
internal/domain/
├── entity.go          # Core business entities (ServerConfig)
├── generator.go       # Generator interface contract
├── repository.go      # Infrastructure interfaces (TemplateRenderer, FileWriter)
├── errors.go          # Domain-specific error types
└── entity_test.go     # Unit tests for domain logic
```

## Core Components

### 1. Entities (`entity.go`)

#### `ServerConfig`
Represents the configuration for a generated server project.

**Fields:**
- `AppName` - Name of the application (validated against Go naming conventions)
- `Port` - Server port (must be between 1024-65535)
- `OutputDir` - Directory where project will be generated
- `IncludeLogger` - Whether to include structured logging
- `IncludeDocker` - Whether to generate Dockerfile
- `IncludeReadme` - Whether to generate README.md

**Methods:**
- `Validate()` - Validates all business rules
- `GetModuleName()` - Returns the Go module name
- `GetProjectPath()` - Returns the full project path

**Validation Rules:**
- App name must be lowercase, start with a letter, and contain only letters, numbers, and underscores
- App name cannot exceed 50 characters
- App name cannot be a Go reserved keyword
- Port must be >= 1024 and <= 65535
- Output directory cannot be empty

### 2. Interfaces

#### `ServerGenerator` (`generator.go`)
Main contract for generating server projects.

```go
type ServerGenerator interface {
    Generate(config *ServerConfig) error
}
```

#### `TemplateRenderer` (`repository.go`)
Abstracts template rendering operations.

```go
type TemplateRenderer interface {
    Render(templateName string, data interface{}) (string, error)
}
```

#### `FileWriter` (`repository.go`)
Abstracts filesystem operations.

```go
type FileWriter interface {
    WriteFile(path string, content string) error
    CreateDirectory(path string) error
    FileExists(path string) bool
}
```

### 3. Error Types (`errors.go`)

Domain-specific errors for better error handling and debugging:

- **`ValidationError`** - Validation failures
- **`GenerationError`** - Errors during server generation
- **`TemplateError`** - Template rendering errors
- **`FileSystemError`** - Filesystem operation errors

All errors implement Go's `error` interface and support error wrapping with `Unwrap()`.

## Design Principles

### Dependency Rule
The domain layer **does not depend** on any other layer. It defines contracts (interfaces) that other layers must implement.

### Clean Architecture
```
┌─────────────────────────────────────┐
│         Domain Layer (Core)          │
│  • Entities (ServerConfig)           │
│  • Interfaces (ServerGenerator)      │
│  • Business Rules (Validation)       │
│  • No external dependencies          │
└─────────────────────────────────────┘
         ↑                    ↑
         │                    │
    ┌────┴────┐         ┌────┴────┐
    │ Usecase │         │  Infra  │
    │  Layer  │         │  Layer  │
    └─────────┘         └─────────┘
```

### Testability
All domain logic is pure and easily testable without mocks or external dependencies.

## Testing

Run tests:
```bash
go test ./internal/domain/... -v
```

Run with coverage:
```bash
go test ./internal/domain/... -cover
```

## Usage Example

```go
// Create a configuration
config := &domain.ServerConfig{
    AppName:       "myapp",
    Port:          8080,
    OutputDir:     "./output",
    IncludeLogger: true,
    IncludeDocker: true,
    IncludeReadme: true,
}

// Validate before use
if err := config.Validate(); err != nil {
    // Handle validation error
    log.Fatalf("Invalid configuration: %v", err)
}

// Get computed values
moduleName := config.GetModuleName()  // "github.com/yourorg/myapp"
projectPath := config.GetProjectPath() // "./output/myapp"
```

## Next Steps

After the domain layer, implement:
1. **Usecase Layer** - Orchestrates domain logic
2. **Infrastructure Layer** - Implements domain interfaces
3. **DI Container** - Wires everything together

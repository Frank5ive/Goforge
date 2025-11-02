# Usecase Layer

The **Usecase Layer** orchestrates business operations by coordinating domain entities and infrastructure interfaces. It implements the core application logic without depending on specific frameworks or external libraries.

## Structure

```
internal/usecase/
├── server_generator.go       # Main orchestration logic
└── server_generator_test.go  # Comprehensive unit tests with mocks
```

## Core Components

### `ServerGeneratorUsecase`

The main usecase that implements the `domain.ServerGenerator` interface. It orchestrates the entire server generation process through a series of well-defined steps.

#### Dependencies (Constructor Injection)

```go
type ServerGeneratorUsecase struct {
    templateRenderer domain.TemplateRenderer
    fileWriter       domain.FileWriter
}
```

- **`templateRenderer`** - Handles template rendering (abstraction from domain)
- **`fileWriter`** - Handles filesystem operations (abstraction from domain)

#### Constructor

```go
func NewServerGeneratorUsecase(
    renderer domain.TemplateRenderer,
    writer domain.FileWriter,
) *ServerGeneratorUsecase
```

Uses dependency injection to receive implementations from the infrastructure layer.

## Generation Process

The `Generate` method orchestrates 12 steps:

1. **Validate Configuration** - Uses domain validation rules
2. **Check Project Existence** - Prevents overwriting existing projects
3. **Create Directory Structure** - Sets up project folders
4. **Generate main.go** - Application entry point
5. **Generate server.go** - HTTP server setup with graceful shutdown
6. **Generate Health Handler** - `/health` endpoint
7. **Generate go.mod** - Go module file
8. **Generate Logger** (optional) - Structured logging package
9. **Generate Middleware** - HTTP middleware (logging, recovery)
10. **Generate Dockerfile** (optional) - Multi-stage container build
11. **Generate README** (optional) - Documentation
12. **Generate .gitignore** - Git ignore patterns

### Generated Project Structure

```
<appname>/
├── main.go              # Application entry point
├── go.mod               # Go module file
├── .gitignore           # Git ignore patterns
├── Dockerfile           # (optional) Container build
├── README.md            # (optional) Documentation
├── handlers/
│   └── health.go        # Health check handler
├── server/
│   ├── server.go        # HTTP server with graceful shutdown
│   └── middleware.go    # HTTP middleware
└── logger/              # (optional)
    └── logger.go        # Structured logger
```

## Error Handling

All errors are wrapped with domain-specific error types for better debugging:

- **Validation Errors** - Configuration validation failures
- **Generation Errors** - Step-specific failures with context
- Each error includes the step name and wrapped original error

Example:
```go
return domain.NewGenerationError(
    "generate_main",
    "failed to generate main.go",
    err,
)
```

## Design Principles

### 1. Dependency Inversion
The usecase depends on domain interfaces, not concrete implementations:

```
┌─────────────────────────┐
│   Usecase Layer         │
│  (Orchestration)        │
└───────┬─────────────────┘
        │ depends on
        ↓
┌─────────────────────────┐
│   Domain Interfaces     │
│  (Contracts)            │
└─────────────────────────┘
        ↑ implemented by
        │
┌─────────────────────────┐
│  Infrastructure Layer   │
│  (Implementations)      │
└─────────────────────────┘
```

### 2. Single Responsibility
Each method has a single, well-defined purpose:
- `Generate()` - Orchestrates the overall process
- `generateMainFile()` - Generates only main.go
- `generateServerFile()` - Generates only server.go
- etc.

### 3. Open/Closed Principle
New file types can be added without modifying existing code:
```go
// Easy to add new generation steps
if config.IncludeSwagger {
    if err := u.generateSwagger(config); err != nil {
        return domain.NewGenerationError("generate_swagger", "failed", err)
    }
}
```

### 4. Testability
All dependencies are injected, making the usecase fully testable with mocks:

```go
renderer := NewMockTemplateRenderer()
writer := NewMockFileWriter()
usecase := NewServerGeneratorUsecase(renderer, writer)
```

## Testing

### Test Coverage: 80.7%

The test suite includes:
- ✅ Successful generation with all features
- ✅ Generation without optional features
- ✅ Invalid configuration handling
- ✅ Project already exists error
- ✅ Template rendering errors
- ✅ File write errors
- ✅ Directory creation errors
- ✅ Project structure validation

### Running Tests

```bash
# Run all tests
go test ./internal/usecase/... -v

# Run with coverage
go test ./internal/usecase/... -cover

# Generate coverage report
go test ./internal/usecase/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Mock Implementations

The test suite provides mock implementations for testing:

```go
// MockTemplateRenderer - tracks template rendering calls
type MockTemplateRenderer struct {
    RenderFunc func(templateName string, data interface{}) (string, error)
    CallCount  map[string]int
}

// MockFileWriter - tracks file and directory operations
type MockFileWriter struct {
    WriteFileFunc      func(path string, content string) error
    CreateDirectoryFunc func(path string) error
    FileExistsFunc     func(path string) bool
    FilesWritten       map[string]string
    DirsCreated        []string
}
```

## Usage Example

```go
// Create dependencies (from infrastructure layer)
renderer := infrastructure.NewGoTemplateRenderer("./templates")
writer := infrastructure.NewFileSystemWriter()

// Create usecase with dependency injection
usecase := usecase.NewServerGeneratorUsecase(renderer, writer)

// Create configuration
config := &domain.ServerConfig{
    AppName:       "myapp",
    Port:          8080,
    OutputDir:     "./output",
    IncludeLogger: true,
    IncludeDocker: true,
    IncludeReadme: true,
}

// Generate the server
if err := usecase.Generate(config); err != nil {
    log.Fatalf("Generation failed: %v", err)
}
```

## Integration with Other Layers

### From CLI Layer
```go
// cmd/generate.go calls the usecase
generator := di.GetServerGenerator() // From DI container
err := generator.Generate(config)
```

### To Infrastructure Layer
```go
// Usecase calls infrastructure through interfaces
content := u.templateRenderer.Render("main.tmpl", config)
u.fileWriter.WriteFile(path, content)
```

## Extensibility

### Adding New File Types

1. Add template to `templates/` directory
2. Create generation method in usecase:
```go
func (u *ServerGeneratorUsecase) generateNewFile(config *domain.ServerConfig) error {
    content, err := u.templateRenderer.Render("newfile.tmpl", config)
    if err != nil {
        return err
    }
    path := filepath.Join(config.GetProjectPath(), "newfile.go")
    return u.fileWriter.WriteFile(path, content)
}
```
3. Call it from `Generate()` method
4. Add test case

### Adding Configuration Options

1. Add field to `domain.ServerConfig`
2. Update validation if needed
3. Use in generation logic:
```go
if config.NewFeature {
    if err := u.generateNewFeature(config); err != nil {
        return domain.NewGenerationError("generate_feature", "failed", err)
    }
}
```

## Next Steps

After the usecase layer, implement:
1. **Infrastructure Layer** - Template rendering and file I/O
2. **Templates** - Go template files for code generation
3. **DI Container** - Wire everything together
4. **Integration** - Connect CLI to usecase

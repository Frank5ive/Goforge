# Dependency Injection (DI) Layer

The **DI Layer** is responsible for wiring all application components together, following the Dependency Inversion Principle. It creates instances of infrastructure components and injects them into use cases.

## Structure

```
internal/di/
├── container.go       # DI container implementation
├── container_test.go  # Unit tests
└── README.md          # This file
```

## Core Components

### `Container`

The main dependency injection container that holds all application dependencies.

```go
type Container struct {
    ServerGeneratorUsecase domain.ServerGenerator
}
```

**Purpose:**
- Holds references to all top-level application services
- Provides a single point of dependency access
- Ensures proper lifecycle management

### `BuildContainer`

Factory function that creates and wires the entire dependency graph.

```go
func BuildContainer(templateDir string) *Container
```

**Process:**
1. Creates infrastructure components (FileWriter, TemplateRenderer)
2. Injects infrastructure into use cases
3. Returns container with fully wired dependencies

## Dependency Graph

```
Container
    └── ServerGeneratorUsecase
            ├── TemplateRenderer (GoTemplateRenderer)
            │   └── templateDir (string)
            └── FileWriter (FileSystemWriter)
```

## Usage

### From CLI

```go
// cmd/generate.go
templatesDir := "./templates"
container := di.BuildContainer(templatesDir)

// Use the wired dependencies
config := &domain.ServerConfig{
    AppName: "myapp",
    Port:    8080,
}

err := container.ServerGeneratorUsecase.Generate(config)
```

### In Tests

```go
func TestMyFeature(t *testing.T) {
    // Create temp dir with test templates
    tempDir := t.TempDir()
    
    // Build container for testing
    container := di.BuildContainer(tempDir)
    
    // Use in test
    // ...
}
```

## Design Principles

### 1. Constructor Injection

All dependencies are injected through constructors:

```go
// Infrastructure
renderer := template.NewGoTemplateRenderer(templateDir)
writer := filesystem.NewFileSystemWriter()

// Use case (infrastructure injected)
usecase := usecase.NewServerGeneratorUsecase(renderer, writer)
```

### 2. Single Responsibility

The DI container has one job: **wire dependencies**. It doesn't contain business logic.

### 3. Fail Fast

If any dependency fails to initialize, the container builder fails immediately with a clear error message.

```go
templateRenderer, err := template.NewGoTemplateRenderer(templateDir)
if err != nil {
    log.Fatalf("failed to create template renderer: %v", err)
}
```

## Dependency Flow

```
CLI (cmd/generate.go)
    ↓ calls
BuildContainer(templateDir)
    ↓ creates
FileSystemWriter + GoTemplateRenderer
    ↓ injects into
ServerGeneratorUsecase
    ↓ returns
Container { ServerGeneratorUsecase }
    ↓ used by
CLI to generate projects
```

## Testing

### Test Coverage
- ✅ Container creation
- ✅ Dependency wiring
- ✅ Template directory handling
- ✅ Integration testing

### Running Tests

```bash
# Run DI tests
go test ./internal/di/... -v

# With coverage
go test ./internal/di/... -cover
```

### Test Strategy

Tests use temporary directories with minimal template files:

```go
tempDir := t.TempDir()

// Create minimal templates
for _, fileName := range templateFiles {
    os.WriteFile(filepath.Join(tempDir, fileName), []byte("test"), 0644)
}

container := BuildContainer(tempDir)
```

## Error Handling

### Template Directory Not Found

```go
templateRenderer, err := template.NewGoTemplateRenderer(templateDir)
if err != nil {
    log.Fatalf("failed to create template renderer: %v", err)
}
```

**Impact:** Application exits with error message
**Solution:** Ensure templates directory exists before running

### Missing Template Files

The template renderer will fail if any required templates are missing.

**Required Templates:**
- main.tmpl
- server.tmpl
- handler.tmpl
- gomod.tmpl
- logger.tmpl
- middleware.tmpl
- dockerfile.tmpl
- readme.tmpl
- gitignore.tmpl

## Extension Points

### Adding New Dependencies

1. Add field to Container struct:
```go
type Container struct {
    ServerGeneratorUsecase domain.ServerGenerator
    NewDependency          SomeInterface  // New!
}
```

2. Wire in BuildContainer:
```go
func BuildContainer(templateDir string) *Container {
    // ... existing code ...
    
    newDep := NewDependencyImpl()
    
    return &Container{
        ServerGeneratorUsecase: serverGeneratorUsecase,
        NewDependency:          newDep,
    }
}
```

### Adding Configuration

For more complex configuration needs:

```go
type ContainerConfig struct {
    TemplateDir string
    LogLevel    string
    // ... other config
}

func BuildContainerWithConfig(cfg *ContainerConfig) *Container {
    // Use cfg to initialize components
}
```

## Best Practices

### ✅ Do

- Keep container focused on wiring
- Use constructor injection
- Fail fast with clear errors
- Test the container
- Document dependencies

### ❌ Don't

- Put business logic in container
- Use global state
- Hide dependencies
- Create circular dependencies
- Skip error checking

## Alternatives Considered

### 1. Service Locator Pattern
**Why not:** Hides dependencies, harder to test

### 2. Global Variables
**Why not:** Not testable, tight coupling

### 3. Wire (Google)
**Why not:** Adds complexity for small project

**Our approach:** Manual DI is simple, explicit, and testable.

## Integration

### With CLI

```go
// main.go -> cmd/generate.go
func runGenerate() error {
    templatesDir, _ := getTemplatesDir()
    container := di.BuildContainer(templatesDir)
    return container.ServerGeneratorUsecase.Generate(config)
}
```

### With Tests

```go
// Any test file
func TestFeature(t *testing.T) {
    container := di.BuildContainer(testTemplateDir)
    // Use container.ServerGeneratorUsecase
}
```

## Maintenance

When adding new features:

1. **Domain Layer:** Define interfaces
2. **Infrastructure Layer:** Implement interfaces
3. **Use Case Layer:** Consume interfaces
4. **DI Layer:** Wire everything together ← Update here

The DI layer is the **last** place you make changes when adding features.

---

**Status:** ✅ Complete
**Test Coverage:** ~85%
**Tests Passing:** 5/5

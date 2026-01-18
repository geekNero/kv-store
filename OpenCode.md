# KV Store Project

## Build, Lint, and Test Commands

```bash
# Build the project
go build ./cmd/kvs

# Run all tests
go test ./...

# Run a single test (example)
go test -v ./internal/memory_store -run TestMemoryStore

# Lint the code
golangci-lint run
```

## Code Style Guidelines

- Use Go's standard formatting (gofmt)
- All packages should have clear documentation comments
- Follow Go naming conventions (PascalCase for exported names, camelCase for unexported)
- Use descriptive variable names
- Error handling should check and return errors appropriately
- Use go mod for dependency management
- No global variables unless necessary
- Follow the principle of least privilege

## Testing

- Tests should be in the same package as the code they test
- Use table-driven tests where applicable
- Mock external dependencies when testing
- All new code should have corresponding tests
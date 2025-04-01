# Ramble Tests

This directory contains tests for the Ramble application.

## Directory Structure

- `unit/` - Contains unit tests for individual components
- `integration/` - Contains integration tests for the complete application

## Running Tests

### Running all tests

```bash
go test ./tests/...
```

### Running unit tests only

```bash
go test ./tests/unit/...
```

### Running integration tests only

```bash
go test ./tests/integration/...
```

### Running with verbose output

```bash
go test -v ./tests/...
```

### Running with coverage reporting

```bash
go test -cover ./tests/...
```

## Writing Tests

### Unit Tests

Unit tests should focus on testing single components in isolation. They should be fast and not rely on external resources.

Example:
```go
func TestComponent(t *testing.T) {
    // Test setup
    component := NewComponent()

    // Test execution
    result := component.DoSomething()

    // Test verification
    if result != expectedResult {
        t.Errorf("Expected %v, got %v", expectedResult, result)
    }
}
```

### Integration Tests

Integration tests should test the interaction between multiple components or the complete application. They may be slower and depend on external resources.

Example:
```go
func TestFeature(t *testing.T) {
    // Test setup
    app := NewApp()

    // Test execution
    app.Start()
    result := app.ProcessInput(testInput)
    app.Stop()

    // Test verification
    if result != expectedResult {
        t.Errorf("Expected %v, got %v", expectedResult, result)
    }
}
```

## Test Data

Test data files (like audio samples) should be placed in a `testdata/` directory within each test directory.
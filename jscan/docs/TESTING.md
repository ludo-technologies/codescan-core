# Testing Guide

The jscan Go code is the `internal/js` package of the `polyscan` module, so every command below runs from `polyscan/polyscan`, not from `jscan/`.

## Running Tests

```bash
cd polyscan

# Run all tests with the race detector
make test

# Run tests directly with go test
go test ./...

# Run the JavaScript/TypeScript tests only
go test ./internal/js/...

# Run tests for a specific package
go test ./internal/js/analyzer/...
go test ./internal/js/service/...
go test ./internal/js/app/...

# Skip long-running tests
go test -short ./internal/js/...
```

## Running Benchmarks

```bash
# Run all benchmarks with memory allocation stats
go test -bench=. -benchmem -run='^$' ./internal/js/...

# Run benchmarks for a specific package
go test -bench=. -benchmem -run='^$' ./internal/js/analyzer/...
```

## Code Coverage

```bash
# Generate an HTML coverage report
go test -coverprofile=coverage.out ./internal/js/...
go tool cover -html=coverage.out -o coverage.html

# This produces:
#   coverage.out  - raw coverage profile
#   coverage.html - HTML report (open in browser)
```

## Test Data

Test fixtures live under the `polyscan/testdata/` directory:

```
testdata/
└── javascript/
    └── simple/     # Simple JavaScript files for basic test scenarios
```

Test files use these fixtures to parse real JavaScript source code through tree-sitter, ensuring analysis results are validated against actual language constructs rather than synthetic inputs.

## Writing New Tests

Follow standard Go testing conventions:

- Test files are named `*_test.go` and placed alongside the code they test
- Use `testing.T` for unit tests and `testing.B` for benchmarks
- Use `testdata/` for fixture files -- Go tooling automatically excludes this directory from builds
- Use `t.Helper()` in test helper functions for accurate error line reporting
- Use `t.Skip()` or `testing.Short()` to conditionally skip long-running tests

### Example Test Structure

```go
func TestComplexity_SimpleFunction(t *testing.T) {
    // Arrange: load fixture
    src, err := os.ReadFile("testdata/javascript/simple/example.js")
    if err != nil {
        t.Fatal(err)
    }

    // Act: run analysis
    result, err := analyzer.CalculateComplexity(src)
    if err != nil {
        t.Fatal(err)
    }

    // Assert: verify results
    if result.Cyclomatic != 3 {
        t.Errorf("expected cyclomatic complexity 3, got %d", result.Cyclomatic)
    }
}
```

### Test Utilities

Shared test helpers are available in `internal/js/testutil/` for common setup and assertion patterns.

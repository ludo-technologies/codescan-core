# Development Guide

## Prerequisites

- **Go 1.24.6+** - [Download](https://go.dev/dl/)
- **A C compiler** - required because tree-sitter is reached through cgo
- **golangci-lint** - Required for linting (`make lint`)

## Getting Started

```bash
# Clone the monorepo and enter the jscan directory
git clone https://github.com/ludo-technologies/polyscan.git
cd polyscan/jscan

# Build the binary (the Go code lives in the polyscan module; make delegates there)
make build
```

## Makefile Targets

| Target | Description |
|---|---|
| `make build` | Build the `jscan` binary |
| `make test` | Run all tests with verbose output |
| `make test-short` | Run short tests only (skip long-running tests) |
| `make bench` | Run benchmarks with memory allocation stats |
| `make coverage` | Generate HTML coverage report (`coverage.html`) |
| `make lint` | Run `golangci-lint` |
| `make fmt` | Format all Go source files |
| `make clean` | Remove build artifacts, coverage files, and `dist/` |
| `make install` | Build and install the binary via `go install` |
| `make run` | Build and run against `testdata/javascript/simple/` |
| `make version` | Print version, commit, and build date |
| `make build-all` | Attempt builds for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64. See the note below |
| `make deps` | Download and verify module dependencies |
| `make tidy` | Tidy `go.mod` and `go.sum` |

## Project Structure

```
jscan/
├── cmd/jscan/          # CLI entry point (main.go, analyze.go, check.go, deps.go, init.go)
├── domain/             # Domain models (complexity, dead code, clone, coupling, output)
├── app/                # Application use cases
├── service/            # Service layer
├── internal/
│   ├── parser/         # tree-sitter JavaScript/TypeScript parser
│   ├── analyzer/       # Analysis engines (CFG, complexity, dead code, clones, deps)
│   ├── config/         # Configuration management
│   ├── constants/      # Shared constants
│   ├── testutil/       # Test utilities
│   └── version/        # Version information
├── npm/                # npm package wrapper
└── testdata/           # Test fixtures (javascript/)
```

## Build Details

The build injects version metadata via linker flags (`-ldflags`):

- `Version` - from `git describe --tags --always --dirty`
- `Commit` - from `git rev-parse --short HEAD`
- `Date` - build date
- `BuiltBy` - set to `make` when built via Makefile, or `release` when built via the release workflow

The release workflow injects all four fields (`Version`, `Commit`, `Date`, and `BuiltBy=release`). Binaries built with a bare `go build` retain the placeholder values (`dev`, `unknown`, `unknown`, `source`).

## Cross-compilation does not work

tree-sitter is a C library reached through cgo, so setting `GOOS` and `GOARCH`
only succeeds for the platform you are already on. `make build-all` will fail
for the other targets.

This is why `.github/workflows/jscan-release.yml` builds each target on its own
runner rather than cross-compiling from one. Note that there is no Intel macOS
target: the release matrix covers linux/amd64, linux/arm64, darwin/arm64, and
windows/amd64.

## Documentation site

The user-facing documentation at [polyscan.codescan.dev](https://polyscan.codescan.dev/)
is built with MkDocs Material from `website/` at the repository root.

```bash
cd ../website
pip install -r requirements.txt
mkdocs serve
```

CI builds it with `mkdocs build --strict`, so a broken internal link fails the
build.

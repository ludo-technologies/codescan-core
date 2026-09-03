# Development Guide

## Prerequisites

- **Go 1.24.6+** - [Download](https://go.dev/dl/)
- **A C compiler** - required because tree-sitter is reached through cgo

## Getting Started

```bash
# Clone the monorepo and enter the polyscan module
git clone https://github.com/ludo-technologies/polyscan.git
cd polyscan/polyscan

# Build the polyscan binary, which runs the jscan code for JavaScript and TypeScript
make build
./polyscan analyze testdata/javascript/simple/
```

There is no separate `jscan` binary anymore. The `jscan` npm package is a wrapper that runs `polyscan`, and the Go code that was the jscan CLI is the `internal/js` package of the `polyscan` module.

## Makefile Targets

The Makefile lives in `polyscan/polyscan`:

| Target | Description |
|---|---|
| `make build` | Build the `polyscan` binary |
| `make test` | Run all tests with the race detector |
| `make lint` | Run `go vet` and check `gofmt` |
| `make fmt` | Format all Go source files |
| `make clean` | Remove build artifacts, coverage files, and `dist/` |
| `make install` | Build and install the binary via `go install` |
| `make run` | Build and run against `testdata/go/` |

## Project Structure

```
polyscan/
├── cmd/polyscan/           # CLI entry point
├── internal/js/            # The JavaScript/TypeScript backend, formerly jscan
│   ├── js.go               # Loads configuration, collects files, and runs the analyses
│   ├── domain/             # Domain models (complexity, dead code, clone, coupling, output)
│   ├── app/                # Application use cases
│   ├── service/            # Service layer
│   ├── parser/             # tree-sitter JavaScript/TypeScript parser
│   ├── analyzer/           # Analysis engines (CFG, complexity, dead code, clones, deps)
│   ├── config/             # Configuration management
│   ├── constants/          # Shared constants
│   ├── testutil/           # Test utilities
│   └── version/            # Version information
└── testdata/javascript/    # Test fixtures

jscan/
├── npm/                    # The jscan npm package, a wrapper around polyscan
└── docs/                   # This documentation
```

## Build Details

The build injects version metadata via linker flags (`-ldflags`):

- `Version` - from `git describe --tags --always --dirty`
- `Commit` - from `git rev-parse --short HEAD`
- `Date` - build date
- `BuiltBy` - set to `make` when built via Makefile, or `release` when built via the release workflow

Both `internal/version` and `internal/js/version` are stamped, because the JavaScript section of the report carries jscan's version fields. The release workflow injects all four fields (`Version`, `Commit`, `Date`, and `BuiltBy=release`). Binaries built with a bare `go build` retain the placeholder values (`dev`, `unknown`, `unknown`, `source`).

## Cross-compilation does not work

tree-sitter is a C library reached through cgo, so setting `GOOS` and `GOARCH`
only succeeds for the platform you are already on.

This is why `.github/workflows/polyscan-release.yml` builds each target on its
own runner rather than cross-compiling from one. The jscan release workflow only
publishes the npm wrapper and builds nothing.

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

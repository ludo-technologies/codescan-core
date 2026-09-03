# Contributing to jscan

Thank you for your interest in contributing to jscan! This document provides guidelines and instructions for contributing.

## Development Environment Setup

### Prerequisites

- **Go 1.24.6+** - [Download Go](https://go.dev/dl/)
- **A C compiler** - required because jscan parses with tree-sitter through cgo
- **Make** - Available by default on macOS and Linux

### Getting Started

jscan lives in the [polyscan monorepo](https://github.com/ludo-technologies/polyscan). Its Go code is the `internal/js` package of the `polyscan` module, which the `polyscan` CLI runs for JavaScript and TypeScript files. Only the npm wrapper package and the documentation stay under `jscan/`. The standalone `jscan` repository and the Go `jscan` CLI were retired.

1. Fork the repository on GitHub
2. Clone your fork and enter the `polyscan` module directory:
   ```bash
   git clone https://github.com/<your-username>/polyscan.git
   cd polyscan/polyscan
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```

## Build and Test

The `polyscan` module has a Makefile for common development tasks. Run it from `polyscan/polyscan`:

| Command | Description |
|---|---|
| `make build` | Build the `polyscan` binary |
| `make test` | Run the test suite with the race detector |
| `make lint` | Run `go vet` and check `gofmt` |
| `make fmt` | Format code with go fmt |

Before submitting a pull request, ensure all checks pass:

```bash
make lint
make test
```

## Pull Request Guidelines

1. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feat/my-feature
   ```
2. **Write tests** for any new functionality or bug fixes.
3. **Run lint and tests** before pushing:
   ```bash
   make lint
   make test
   ```
4. **Submit a pull request** against the `main` branch with a clear description of the changes.

## Commit Message Convention

This project follows [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>: <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | Description |
|---|---|
| `feat` | A new feature |
| `fix` | A bug fix |
| `docs` | Documentation only changes |
| `chore` | Maintenance tasks (deps, CI, etc.) |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `test` | Adding or updating tests |

### Examples

```
feat: add support for Vue.js single-file components
fix: resolve false positive in dead code detection for re-exports
docs: update README with new CLI options
chore: upgrade tree-sitter dependency to v0.25
```

## Code Style

- Run `go fmt` (or `make fmt`) to format your code before committing.
- Run `make lint` to catch common issues.
- Follow standard Go conventions and idioms.

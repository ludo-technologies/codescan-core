# Contributing

jscan lives in the [polyscan monorepo](https://github.com/ludo-technologies/polyscan) under `jscan/`. The standalone `jscan` repository was retired and its history moved here.

## Set up

You need Go 1.24.6 or later, a C compiler, and `golangci-lint`. The C compiler is required because jscan parses with tree-sitter, which is a C library reached through cgo.

```bash
git clone https://github.com/ludo-technologies/polyscan.git
cd polyscan/jscan
make build
```

## Everyday commands

Run these from the `jscan/` directory.

| Command | What it does |
| --- | --- |
| `make build` | Build the `jscan` binary |
| `make test` | Run the full test suite |
| `make test-short` | Skip the long-running tests |
| `make lint` | Run `golangci-lint` |
| `make fmt` | Format the Go sources |
| `make coverage` | Write an HTML coverage report |
| `make bench` | Run benchmarks with allocation statistics |
| `make run` | Build and run against the bundled fixtures |
| `make tidy` | Tidy `go.mod` and `go.sum` |

Run `make lint` and `make test` before opening a pull request. Continuous integration runs both.

!!! note "`make build-all` does not work for every target"

    The target sets `GOOS` and `GOARCH` for five platforms, but tree-sitter needs cgo, so cross-compiling only succeeds for the platform you are already on. The release pipeline builds each target on its own runner for this reason. Use it to check that a build works locally, not to produce release artifacts.

## Where the code lives

```text
jscan/
├── cmd/jscan/     CLI entry point, one file per command
├── app/           Application use cases
├── service/       Orchestration, formatting, output
├── domain/        Models and service interfaces, depends on nothing
├── internal/
│   ├── parser/    tree-sitter integration
│   ├── analyzer/  The analysis engines
│   ├── config/    Configuration loading and validation
│   ├── reporter/  Report formatting
│   └── version/   Version information
├── npm/           The npm package wrapper
└── testdata/      Test fixtures
```

The layering follows Clean Architecture, with every layer depending on `domain` and `domain` depending on nothing. Command handlers may call either a use case in `app/` or a service directly, whichever keeps the concurrency simpler. [`jscan/docs/ARCHITECTURE.md`](https://github.com/ludo-technologies/polyscan/blob/main/jscan/docs/ARCHITECTURE.md) describes each layer and the reasoning behind the main design decisions.

## The shared core

Algorithms that are not specific to a language live in the `core/` module at the repository root, which both jscan and pyscn depend on as a published Go module. That covers APTED tree edit distance, control flow graph structures and their analyses, clone grouping, graph algorithms including Tarjan's, MinHash and locality-sensitive hashing, and the health score calculation.

This matters when you plan a change. A fix to a shared algorithm belongs in `core/` and affects both analyzers, so it needs a `core/vX.Y.Z` tag before the consuming change can reference it. A fix to JavaScript-specific behavior, such as the control flow graph builder or the module resolver, belongs in `jscan/`.

If you are unsure which side a change belongs on, say so in the issue before writing it.

## Commits

The project uses [Conventional Commits](https://www.conventionalcommits.org/):

```text
feat: add support for Vue single-file components
fix: resolve false positive in dead code detection for re-exports
docs: document the exclude pattern matching rules
chore: upgrade tree-sitter to v0.25
```

The accepted types are `feat`, `fix`, `docs`, `chore`, `refactor`, and `test`.

## Pull requests

Branch from `main`, write tests for the behavior you are changing, and open the pull request against `main` with a description of what changed and why.

Changes to the analyzers need test coverage in particular, since a regression there is silent. It produces a different number rather than an error, and nobody notices until the score moves.

## Contributing to these docs

This site is built with MkDocs Material from `website/` in the same repository.

```bash
cd website
pip install -r requirements.txt
mkdocs serve
```

That serves the site at `http://127.0.0.1:8000` with live reload. Continuous integration builds with `mkdocs build --strict`, which turns a broken internal link into a build failure, so check your links before pushing.

Every page has an edit link in its top right corner that takes you to the source file on GitHub.

One request specific to this site. Much of what is documented here was verified by running the binary rather than by reading the code, because the two disagreed in several places. If you document a behavior, run it first, and paste the output you actually got.

## Reporting bugs

Open an issue in the [polyscan repository](https://github.com/ludo-technologies/polyscan/issues). Include the output of `jscan version`, the exact command, and the smallest input that reproduces the problem.

## Code of Conduct

Participation is governed by the [Code of Conduct](https://github.com/ludo-technologies/polyscan/blob/main/CODE_OF_CONDUCT.md).

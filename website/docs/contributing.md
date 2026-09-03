# Contributing

polyscan lives in the [polyscan monorepo](https://github.com/ludo-technologies/polyscan), in the `polyscan/` module. jscan, the JavaScript/TypeScript analyzer it grew out of, merged into that module and survives as its `internal/js` backend.

## Set up

You need Go 1.24.6 or later and a C compiler. The C compiler is required because polyscan parses with tree-sitter, which is a C library reached through cgo.

```bash
git clone https://github.com/ludo-technologies/polyscan.git
cd polyscan/polyscan
make build
```

## Everyday commands

Run these from the `polyscan/` directory.

| Command | What it does |
| --- | --- |
| `make build` | Build the `polyscan` binary |
| `make test` | Run the test suite with the race detector |
| `make lint` | Run `go vet` and check `gofmt` |
| `make fmt` | Format the Go sources |

Run `make lint` and `make test` before opening a pull request. Continuous integration runs both.

!!! note "Cross-compilation does not work"

    tree-sitter needs cgo, so cross-compiling by setting `GOOS` only succeeds for the platform you are already on. The release pipeline builds each target on its own runner for this reason.

## Where the code lives

```text
polyscan/
├── cmd/polyscan/  CLI entry point
├── cmd/jscan/     The retired jscan CLI, kept buildable from the moved code
├── internal/
│   ├── analysis/  File collection and dispatch for the generic engine
│   ├── engine/    The declarative language engine (grammar + queries)
│   ├── lang/      Language definitions: golang, rust, cpp
│   ├── clone/     Clone detection over the generic engine
│   ├── js/        The JavaScript/TypeScript backend, formerly jscan
│   ├── report/    Combining every language into one report
│   └── version/   Version information
└── npm/           The npm package wrapper
```

Inside `internal/js` the layering follows Clean Architecture, with every layer depending on `domain` and `domain` depending on nothing. [`jscan/docs/ARCHITECTURE.md`](https://github.com/ludo-technologies/polyscan/blob/main/jscan/docs/ARCHITECTURE.md) describes each layer and the reasoning behind the main design decisions, from before the move.

Adding a language to the generic engine is declarative: a tree-sitter grammar and two queries. `internal/lang/golang/golang.go` is the reference implementation, and the polyscan [README](https://github.com/ludo-technologies/polyscan/blob/main/polyscan/README.md#adding-a-language) documents the query contract.

## The shared core

Algorithms that are not specific to a language live in the `core/` module at the repository root, which both polyscan and pyscn depend on as a published Go module. That covers APTED tree edit distance, control flow graph structures and their analyses, clone grouping, graph algorithms including Tarjan's, MinHash and locality-sensitive hashing, and the health score calculation.

This matters when you plan a change. A fix to a shared algorithm belongs in `core/` and affects both analyzers, so it needs a `core/vX.Y.Z` tag before the consuming change can reference it — the `polyscan` module depends on the published tag rather than a `replace` directive, so that `go install` works. A fix to language-specific behavior, such as the JavaScript module resolver or a tree-sitter query, belongs in `polyscan/`.

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

Open an issue in the [polyscan repository](https://github.com/ludo-technologies/polyscan/issues). Include the output of `polyscan version`, the exact command, and the smallest input that reproduces the problem.

## Code of Conduct

Participation is governed by the [Code of Conduct](https://github.com/ludo-technologies/polyscan/blob/main/CODE_OF_CONDUCT.md).

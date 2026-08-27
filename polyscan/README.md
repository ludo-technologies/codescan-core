# polyscan

A multi-language code quality analyzer. It detects the language of each file by its extension and currently measures cyclomatic complexity for Go.

## Installation

```bash
npx polyscan analyze .
```

Or install from source, which needs Go and a C compiler because the tree-sitter grammars are compiled through cgo:

```bash
go install github.com/ludo-technologies/polyscan/polyscan/cmd/polyscan@latest
```

## Usage

```bash
# Text report to stdout
polyscan analyze .

# JSON report to stdout
polyscan analyze --format json src/

# List only functions with complexity 10 or higher; the summary still covers every function
polyscan analyze --min-complexity 10 .
```

Files that fail to parse are skipped, counted in `files_skipped`, and listed under `Errors`.

## Complexity

Cyclomatic complexity is one plus the number of decision points in a function, counted the way `core/cfg` counts them on a control flow graph:

| Go construct | Decision points |
| --- | --- |
| `if`, `else if` | 1 each |
| `for` (any form) | 1 |
| `switch`, type switch, `select` | 1 per `case`, `default` excluded |
| `&&`, `\|\|` | 1 each |

Function literals are not reported on their own. Their decision points count toward the enclosing function, so the numbers match gocyclo.

Risk levels use the thresholds shared by every polyscan analyzer: low up to 9, medium up to 19, high from 20.

## Adding a language

A language is declarative: a tree-sitter grammar and two queries. See `internal/lang/golang/golang.go`.

- The definitions query matches each function once. `@definition.<kind>` spans the function, `@name` its name, and an optional `@receiver` is prefixed to the name. The bundled `queries/tags.scm` of a grammar is the starting point.
- In the decisions query every capture is one decision point, attributed to the innermost function that contains it and reported under the capture's name.

## Development

```bash
make test    # go test -race ./...
make lint    # go vet and gofmt
make build   # ./polyscan
```

The module depends on the published `core` tag rather than a `replace` directive, so `go install` works. A change to `core` has to be tagged before polyscan can use it.

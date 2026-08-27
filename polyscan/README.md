# polyscan

A multi-language code quality analyzer. It detects the language of each file by its extension and currently measures cyclomatic complexity and detects code clones for Go.

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

# Clone detection only
polyscan analyze --select clone .

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

## Clone detection

Every function of at least 10 lines of code (blank lines and comments excluded) and 20 syntax nodes is a fragment. Fragments are compared with the APTED tree edit distance over a tree of named syntax nodes, with comments dropped and identifiers, literals and operators carried in the node labels. Pairs are classified the way pyscn and jscan classify them:

| Type | Meaning | Reported when |
| --- | --- | --- |
| Type-1 | Exact copy apart from whitespace and comments | Similarity ≥ 0.85 and identical text |
| Type-2 | Same structure with renamed identifiers or changed literals | Similarity ≥ 0.75 and matching normalized trees |
| Type-3 | Near copy with statements added, removed or changed | Similarity ≥ 0.70 |

Pairs below 0.70 are not reported. Test files (`*_test.go`) are analyzed for complexity but excluded from clone detection: test functions share a skeleton by convention, and on this repository they made up 92% of the pairs.

Pairs are merged into groups by connected components, and the groups are deduplicated by the shared `core/clone` passes. When there are more than 10,000 candidate pairs, only pairs that share a MinHash band are compared, and within a band each function is compared with at most 1,024 of the functions that follow it. Neighbours in a band are always compared, so a large set of near-identical functions still ends up in one group.

## Adding a language

A language is declarative: a tree-sitter grammar and two queries. See `internal/lang/golang/golang.go`.

- The definitions query matches each function once. `@definition.<kind>` spans the function, `@name` its name, and an optional `@receiver` is prefixed to the name. The bundled `queries/tags.scm` of a grammar is the starting point.
- In the decisions query every capture is one decision point, attributed to the innermost function that contains it and reported under the capture's name.
- The clone spec lists the node types of identifiers, literals and structural patterns, the cost tiers of the tree edit distance, and pairs of related node types. `TestFiles` names the test file globs to exclude from clone detection.

## Development

```bash
make test    # go test -race ./...
make lint    # go vet and gofmt
make build   # ./polyscan
```

The module depends on the published `core` tag rather than a `replace` directive, so `go install` works. A change to `core` has to be tagged before polyscan can use it.

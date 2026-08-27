# polyscan

A multi-language code quality analyzer. It detects the language of each file by its extension and measures cyclomatic complexity and detects code clones for Go and Rust.

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
# HTML report, written to polyscan-report.html and opened in the browser
polyscan analyze .

# HTML report to a chosen path, without opening the browser
polyscan analyze --no-open -o report.html .

# JSON or text report to stdout
polyscan analyze --format json src/
polyscan analyze --format text src/

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

| Rust construct | Decision points |
| --- | --- |
| `if`, `else if`, `if let` | 1 each |
| `for`, `while`, `while let`, `loop` | 1 each |
| `match` | 1 per arm except the last, which is the path the other arms branch away from, plus 1 per arm guard |
| `let ... else` | 1 |
| `&&`, `\|\|`, including `&&` in a let chain | 1 each |
| `?` | 1, the early return that `core/cfg` counts as an exception edge |

Function literals and closures are not reported on their own. Their decision points count toward the enclosing function, so the Go numbers match gocyclo.

Risk levels use the thresholds shared by every polyscan analyzer: low up to 9, medium up to 19, high from 20.

## Clone detection

Every function of at least 10 lines of code (blank lines and comments excluded) and 20 syntax nodes is a fragment. Fragments are compared with the APTED tree edit distance over a tree of named syntax nodes, with comments dropped and identifiers, literals and operators carried in the node labels. Pairs are classified the way pyscn and jscan classify them:

| Type | Meaning | Reported when |
| --- | --- | --- |
| Type-1 | Exact copy apart from whitespace and comments | Similarity ≥ 0.85 and identical text |
| Type-2 | Same structure with renamed identifiers or changed literals | Similarity ≥ 0.75 and matching normalized trees |
| Type-3 | Near copy with statements added, removed or changed | Similarity ≥ 0.70 |

Pairs below 0.70 are not reported. Test code is analyzed for complexity but excluded from clone detection: test functions share a skeleton by convention, and on this repository they made up 92% of the pairs. For Go that is `*_test.go`; for Rust it is `#[test]` functions, `#[cfg(test)]` modules, `tests.rs` and `*_tests.rs` files and any `tests` directory, the conventional homes of a test module split into its own file and of Cargo's integration tests.

Rust macro invocations parse as token trees, so the code inside a macro call contributes tokens but no structure. Clone detection recall is lower on macro-heavy code. The bundled tree-sitter-rust grammar predates Rust 2024 edition syntax such as `unsafe extern` blocks; a file that uses it is reported as a syntax error and skipped, which affected 0.75% of the files in a sample of 369 crates.

Pairs are merged into groups by connected components, and the groups are deduplicated by the shared `core/clone` passes. When there are more than 10,000 candidate pairs, only pairs that share a MinHash band are compared, and within a band each function is compared with at most 1,024 of the functions that follow it. Neighbours in a band are always compared, so a large set of near-identical functions still ends up in one group.

## Adding a language

A language is declarative: a tree-sitter grammar and two queries. See `internal/lang/golang/golang.go`.

- The definitions query matches each function once. `@definition.<kind>` spans the function, `@name` its name, and an optional `@receiver` is prefixed to the name. The bundled `queries/tags.scm` of a grammar is the starting point.
- In the decisions query every capture is one decision point, attributed to the innermost function that contains it and reported under the capture's name.
- The clone spec lists the node types of identifiers, literals and structural patterns, the cost tiers of the tree edit distance, and pairs of related node types. `TestFiles` names test files by file name glob or, with a trailing slash, by directory, and `TestCode` is a query capturing test code inside a file; both are excluded from clone detection.

## Development

```bash
make test    # go test -race ./...
make lint    # go vet and gofmt
make build   # ./polyscan
```

The module depends on the published `core` tag rather than a `replace` directive, so `go install` works. A change to `core` has to be tagged before polyscan can use it.

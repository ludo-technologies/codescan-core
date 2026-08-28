# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-08-28

The first release of the multi-language analyzer. One `polyscan` binary detects the language of each file by its extension and runs the same analyses on Go, Rust and C++.

### Added

- `polyscan analyze` with an HTML report, opened in the browser by default, and `--format json` and `--format text` for stdout. `--select complexity,clone` chooses the analyses, `--min-complexity` filters the listed functions, `--output` names the HTML file and `--no-open` skips the browser
- Cyclomatic complexity per function, following the `core/cfg` conventions: one point per two-way branch or loop, one per switch or match case with the default excluded, one per short-circuit operator, and one per exception edge (`?` in Rust, `catch` in C++). Closures and lambdas count toward the enclosing function. Go numbers match gocyclo
- Clone detection over functions of at least 10 lines of code and 20 syntax nodes, through `core/clone`: the APTED tree edit distance with a per-language cost model, Type-1 to Type-3 classification, connected grouping with the shared dedupe passes, and MinHash banding above 10,000 candidate pairs. Test code is analyzed for complexity but excluded from clone detection: `*_test.go`; Rust `#[test]` functions, `#[cfg(test)]` items, `tests.rs`, `*_tests.rs` and `tests/`; C++ `*_test.*`, `*_tests.*`, `test_*.*`, `*Test.*`, `test/` and `tests/`
- A generic tree-sitter engine. A language is a grammar plus declarative queries: function definitions, enclosing scopes, decision points, test code, and a clone spec naming identifiers, literals, structural patterns and cost tiers
- Go, Rust and C++ definitions. Headers, `.h` included, are analyzed as C++
- A file with a syntax error is analyzed without the functions that contain the error and reported as partial with a warning; a file that cannot be read is skipped and reported as an error
- Distribution through npm (`npx polyscan`), with native builds per platform, and `go install github.com/ludo-technologies/polyscan/polyscan/cmd/polyscan@latest`

### Known limitations

- C++ is parsed without the preprocessor: every `#if` branch is analyzed, macros are not expanded, and code whose syntax needs expansion is left out of the file's partial result
- Rust macro invocations are opaque token trees, and the bundled grammar predates Rust 2024 syntax such as `unsafe extern` blocks
- Type-4 (semantic) clones are not reported; polyscan runs no control-flow or data-flow analysis

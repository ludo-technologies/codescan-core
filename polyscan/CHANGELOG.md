# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2026-09-05

### Fixed

- Nesting depth is computed for Go, Rust and C++. The generic engine had no notion of nesting, so the directory rows for those languages always printed an average of 0.00 and a maximum of 0. Each language now declares a nesting query and the engine measures a function's depth the way the JavaScript analyzer does: the body is depth 0, an else-if continues its chain, and code in a separately extracted function is that function's own (#94)
- Parse errors are charged to the health score whichever analyses ran. Skipped files were counted only by the complexity analysis, so a run that left complexity out of `--select` scored unparsable files as clean. The dead-code rate is also divided by the files dead code analysis covered, so Go, Rust and C++ files in a mixed tree no longer dilute the JavaScript-only rate (#92)
- The report header shows the real project directory. A relative target such as `polyscan/` printed the project name twice because the common directory was the relative path itself; the analyzed files are resolved to absolute paths first
- The complexity penalty saturated once the weighted ratio of medium and high risk functions reached 5 percent, so a mostly clean codebase with a few complex functions scored 0/100 on complexity. It now saturates at 30 percent, matching the documented intent, and any function above the medium threshold costs at least one point so the score only reads 100 when no function is at risk (#96)

## [0.2.0] - 2026-09-03

### Added

- JavaScript and TypeScript analysis, moved in from jscan. `polyscan analyze` runs complexity, dead code, clone detection, class coupling (CBO) and module dependency analysis on `.js`, `.jsx`, `.ts` and `.tsx` files, reads an existing `jscan.config.json` (or any of the other accepted names) when present, and the `jscan` npm package became a deprecated wrapper around this CLI

### Changed

- One report, one health score across languages. `polyscan analyze` renders every language into the jscan-style report (health score, verdict, per-dimension cards, hotspot files) as HTML, JSON and text; the separate JavaScript report (`polyscan-report.js.html`, the `javascript` JSON key) is gone
- The health score is computed over the dimensions that ran: each enabled dimension is charged against its own maximum, and a dimension a language does not have (dead code, coupling and dependencies outside JavaScript/TypeScript) is left out of the score rather than scored as clean
- `--select` now covers every analysis (`complexity`, `deadcode`, `clone`, `cbo`, `deps`, default all) and applies across languages
- One JSON shape for every language, with `language` on every function and clone fragment

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

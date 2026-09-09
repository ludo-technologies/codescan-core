# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-09-09

### Added

- Class coupling (CBO) for Go and Rust. `polyscan analyze` counts, for each type, the other types of the analyzed tree that its declaration and methods refer to, and scores the result in the Coupling dimension with the thresholds pyscn uses (low up to 3, medium up to 7). Only types the tree declares count: for Go an unqualified name is a type of the same package and `pkg.T` resolves through `go.mod` to a package of the tree, for Rust a bare name resolves to a declaration in the same file or elsewhere in the tree, so the standard library and other modules never count. An embedded field or interface and an implemented trait count as inheritance. Types coupled to nothing, test files and `#[cfg(test)]` code stay out. A Go tree without a `go.mod` keeps only its same-package references, with a warning. In the report a Go or Rust type can now have both a coupling and a cohesion row, and the class count in the overview counts it once
- Class cohesion (LCOM4) for Go and Rust. `polyscan analyze` measures, for each type, how many groups its methods fall into when two methods are connected by a shared field or a call between them, and scores the result in a Cohesion dimension with the thresholds pyscn uses (low up to 2, medium up to 5). A Go type is measured over every method of its package, since they may be spread across files; a Rust type over the `impl` blocks in a file. Methods without a receiver parameter, such as Rust associated functions and Go methods with an unnamed receiver, cannot touch instance state and are listed as excluded. Test files and `#[cfg(test)]` code stay out. `--select lcom` runs it alone
- Dependency analysis for Go. `polyscan analyze` builds the package import graph of a Go tree, resolving each import through the nearest `go.mod`, and reports the same instability, abstractness, main-sequence distance, depth and longest chains it reports for JavaScript/TypeScript, scored in the Dependencies dimension. Abstractness is the share of a package's exported type declarations that are interfaces. Test files, `vendor` and `testdata` directories, and imports of other modules stay out of the graph, and a tree without a `go.mod` leaves the dimension out with a warning rather than scoring it clean

### Changed

- A Rust `impl` names its type by the bare identifier, so the methods of `impl G<T>` read `G::m` instead of `G<T>::m`, and `impl G<i32>` and `impl G<String>` are blocks of one type in the cohesion and coupling analyses. An impl for a reference type such as `&Foo` is a block of `Foo`

### Fixed

- Go, Rust and C++ analysis no longer walks into version control, dependency and build output directories. A directory whose name starts with a dot, such as `.git`, and `node_modules`, `vendor`, `target`, `build`, `dist` and `third_party` are skipped; a path named on the command line is still analyzed whatever it is called. Before, a Rust project's `target` directory and a Go project's `vendor` directory were analyzed as if they were the project's own code
- `--output` is honored for JSON and text reports. `polyscan analyze --format json --output report.json` wrote the report to stdout and ignored the path; it now writes the file, and the default for JSON and text stays stdout (#125)
- The Go cohesion analysis sees a field behind a chained index. tree-sitter-go parses `t.m[k][k]` as a type instantiation, so a method that touched a field only through a nested map or slice index was not connected to the methods sharing that field (#117)
- The `package` flag of a JavaScript/TypeScript module survives into its dependency metrics, so package and non-package modules are told apart in the report again (#124)
- A runtime error no longer prints the command usage after the error message (#121)

## [0.2.2] - 2026-09-05

### Changed

- Clone pairs for Go, Rust and C++ are reported from similarity 0.80 instead of 0.70, and the duplication penalty saturates at a 60 percent fragment ratio instead of 30 percent. Pairs between 0.70 and 0.80 were mostly functions that share a shape rather than code, such as two output formatters' `switch` statements, and the 30 percent saturation scored cobra and polyscan at 25/100 and testify and afero at 0/100 on duplication. With the new settings those repositories score 70, 70, 50 and 30, and pyscn moves from 0 to 60 (#105)

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

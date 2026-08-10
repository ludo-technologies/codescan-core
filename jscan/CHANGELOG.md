# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Report a project scale line (Micro, Small, Medium, Large, Enterprise, classified by analyzed file count) directly below the health score in the terminal summary, the text output, and the HTML report header. The `summary` object of the JSON and YAML output gains `project_scale` and `total_loc`. The scale is contextual only and does not affect the health score
- Apply `analysis.include_patterns`, `analysis.recursive`, `complexity.report_unchanged`, `dead_code.min_severity`, `dead_code.sort_by`, and `output.sort_by`, which until now were validated and then ignored. Include patterns select from the file types jscan can parse, using the same matching rules as `exclude_patterns`; a file named directly on the command line is analyzed whether or not it matches
- Warn on stderr, naming each key, when a configuration file sets keys that no command reads. Misspelled keys are reported the same way
- Report directory complexity rollups. The complexity section of the JSON, YAML, and CSV output gains a `by_directory` array with the function count, average and maximum complexity, high-risk count, and average and maximum nesting depth of each directory, and the text output and the HTML complexity tab gain a matching table. Directory paths are relative to the deepest directory that contains every analyzed file
- Report per-module quality hotspots. The analyze JSON and YAML output gains a `module_quality` array joining, per file, its line count, complexity rollups, and dead-code rollups, with the module name from dependency analysis; the text output, the CSV output, and a new Modules tab in the HTML report show the same table. The rollups are taken before `min_complexity` and `min_severity` are applied, so a filtered report still reports what each module carries

### Changed

- Read and parse each file once into a shared project snapshot instead of once per analysis, and build each file's control flow graphs once for complexity and dead code together. A full `jscan analyze` run held five sets of parse trees at its peak and now holds one: on vuejs/core (147k lines, 474 files) peak memory drops from 1.86 GB to 1.05 GB and the parse-bound analyses (complexity, deadcode, cbo, deps) run in roughly half their previous time. A run selecting a single analysis skips the shared snapshot and keeps releasing each file as it goes, so `--select complexity` and CI `check` runs stay at their previous low peaks. Analysis results are unchanged except for the dependency-analysis fix below
- Adopt `core/clone` for grouping strategies, group dedup, Type-1/2 similarity gates, pair classification, and AST feature extraction; keep JS/TS adapters (fragment extraction, comment stripping, cost model, LSH orchestration)
- Generate configuration files containing only keys that change behavior, so `jscan init` no longer produces a file that is mostly inert. The default for `dead_code.min_severity` becomes `info`, which is the severity floor the analysis has always used
- Match `analysis.include_patterns` and `analysis.exclude_patterns` without regard to case, so that `**/*.ts` selects `Widget.TS` exactly as the file collector accepts it, and `*.min.js` also skips `Vendor.MIN.JS`
- Remove the unreachable `internal/reporter` package, `service/config_loader.go`, and the five `*ConfigurationLoader` interfaces in `domain` that nothing implemented, none of which any command called

### Fixed

- Parse TypeScript with the TypeScript grammar in dependency analysis, which parsed every file with the JavaScript grammar and silently lost whatever a JS parse of a TS file drops — most visibly exports declared with TypeScript-only syntax, which made modules look unreferenced or export-free in the dependency graph. TypeScript projects will see more complete module nodes, and coupling metrics that follow from them (such as `deps_main_sequence_deviation`) can shift. The same rewiring gives dependency analysis real file names for every project, JavaScript included: dependency edges' `location.file_path` reported the placeholder `<input>` and now reports the analyzed file's path, both in `jscan analyze` and in `AnalyzeSingleFile`
- Measure the `nesting_depth` metric, which was reported as 0 for every function because the calculation was never called and, when called, counted a function's control structures instead of measuring how deeply they nest. An `else if` continues the chain its `if` opened, a `catch` clause stays at the level of its `try`, and a nested function is measured on its own
- Count one decision point per `case` label so a `switch` scores like the equivalent `if` chain instead of adding 1 no matter how many cases it has, and report the case count in the previously always-zero `switch_cases` metric. A `default` clause adds nothing, matching `else`. Switch-heavy code (reducers, dispatchers, state machines) now scores higher, so some functions cross the medium/high risk thresholds and complexity scores drop; the code did not get worse, it was under-measured. The same fix corrects a decision point that was lost when a branch sat inside a `try` block, so an `if` inside `try` now adds 1 as it does anywhere else
- Keep the braces of a `switch` body out of its parsed case list, which also stops them from appearing as empty case branches in the control flow graph
- Match `analysis.exclude_patterns` against whole path segments instead of any substring of a file's path, so the default entries `out` and `dist` no longer drop `src/routes/`, `src/layout/`, `src/checkout/`, or `src/utils/distance.ts`. Patterns containing a slash are matched against the path, with `**` spanning any number of directories, and patterns are now evaluated relative to the analyzed path so a parent directory named `build` no longer excludes an entire project. Affected projects will see more files analyzed and therefore different scores

## [0.9.0] - 2026-07-11

### Added

- Ship Agent Skills (health-check, cli-analysis, refactoring, architecture-review) and a Claude Code plugin marketplace entry so AI coding agents can run jscan analyses via `uvx add-skills` or `claude plugin install`

## [0.8.0] - 2026-07-10

### Changed

- Change the code duplication metric from clone-group density to cloned-fragment ratio (clonedFragments/totalFragments), rescaling the 0-10% penalty band to 0-30%

### Fixed

- Populate previously-empty fragment hashes in clone output
- Prefer the highest-similarity clone type when classifying clone groups, so a strong Type-2/4 pair is no longer hidden behind weaker Type-3 transitive edges and dropped by the default type filter
- Suppress clone groups fully covered by another group's larger, comparable-or-better windows, fixing double-counted duplication from overlapping windows
- Drop clone groups left without any positive-similarity backing pair after dedup

## [0.7.0] - 2026-06-12

### Changed

- Overhaul clone detection accuracy (synced with pyscn): APTED correctness fixes (key-root ordering, forest-distance subtree cost, max-based similarity normalization), Type-1/Type-2 gating on textual and normalized-AST-hash similarity, recalibrated thresholds (0.85/0.75/0.70/0.65), Type-3 disabled by default, exact complete-linkage clustering, and MinLines/MinNodes defaults raised to 10/20
- Switch the duplication score metric to clone-group density (groups per 1000 lines, 0-10% penalty scale)
- Soften CBO coupling score calibration and use architecture compliance directly as the architecture score
- Recalibrate coupling zone classification (Zone of Pain / Zone of Uselessness predicates, instability thresholds 0.2/0.8 to 0.3/0.7)
- Exclude dynamic `import()` edges from circular dependency detection; load-time cycles are still reported when a static edge exists
- Report module-scope code as `<module>` instead of `__main__`

### Fixed

- Share clones per fragment so overlapping clone groups merge correctly
- Reject overlapping same-file fragment pairs and remove strict-subset clone group members
- Merge contiguous same-reason dead code findings and skip empty-statement-only blocks

### Performance

- Optimize cross-file dead code analysis with a shared import graph
- Speed up clone detection with a Jaccard pre-filter, cached fragment features, and an LSH candidate cap

## [0.6.2] - 2026-02-19

### Fixed

- Reduce dead code false positives for Next.js and TypeScript imports

## [0.6.1] - 2026-02-15

### Fixed

- Set complexity function locations (start line, column, end line) from AST nodes
- Tidy go.mod dependencies

## [0.6.0] - 2026-02-15

### Fixed

- Stop counting nested functions' operators in parent complexity
- Resolve npx "command not found" by removing bin field from platform packages
- Add files field to main npm package to reduce package size

## [0.5.0] - 2026-02-15

### Changed

- Redesign README to match pyscn style (centered header, Quick Start, collapsible install)
- Add algorithm details to Features section
- Add demo video link

## [0.4.0] - 2026-02-15

### Changed

- Refactor dead code aggregation into dedicated service layer and align architecture docs

### Fixed

- Improve progress bar UX and speed up clone analysis (#46)

## [0.3.0] - 2026-02-14

### Changed

- Unify JSON output keys to snake_case and rename `detect_after_raise` to `detect_after_throw`

### Fixed

- Apply config `max_complexity` when CLI flag is not explicitly set
- Stop auto-discovering extensionless `.jscanrc`
- Wire config loading and harden CI workflows

## [0.2.2] - 2026-02-12

### Changed

- Switch npm distribution to per-platform packages (esbuild-style) for faster installation

## [0.2.1] - 2026-02-12

### Fixed

- Fix version not embedded in release binaries via ldflags

## [0.2.0] - 2026-02-12

### Added

- CLI analysis summary and modernize README ([#38](https://github.com/ludo-technologies/jscan/pull/38))
- Detect orphan files and unused exported functions ([#37](https://github.com/ludo-technologies/jscan/pull/37))
- Detect unused imports and exports in dead code analysis ([#36](https://github.com/ludo-technologies/jscan/pull/36))

### Fixed

- Improve dependency score accuracy ([#41](https://github.com/ludo-technologies/jscan/pull/41))
- Detect nested functions in BuildAll via recursive AST walk ([#40](https://github.com/ludo-technologies/jscan/pull/40))
- Resolve extensionless imports in dependency graph builder ([#39](https://github.com/ludo-technologies/jscan/pull/39))
- Resolve golangci-lint errors across codebase
- Adjust health score thresholds and add score to text output ([#35](https://github.com/ludo-technologies/jscan/pull/35))

## [0.1.1] - 2026-02-02

### Fixed

- Extract binary to temp dir to avoid overwriting bin/jscan script

## [0.1.0] - 2026-02-02

### Added

- JSON output format
- HTML output format with Lighthouse-style scoring
- Dead Code Service layer
- Application layer with Use Cases
- APTED (Tree Edit Distance) algorithm for clone detection
- MinHash and LSH Index for clone detection
- Clone Detector with Type 1-4 support
- Clone Grouping Strategies
- Module Analyzer for JS/TS import/export analysis
- CBO (Coupling Between Objects) metrics
- Dependency Graph with cycle detection
- DOT format for dependency visualization
- `check` command for CI/CD integration
- `init` command for config file generation
- Progress manager for long-running analysis tasks
- Parallel executor for concurrent task execution
- Default exclude patterns for common directories
- npm package distribution

### Changed

- Default output format to HTML for analyze command

### Fixed

- Clone loss bug and improved determinism in grouping strategies
- Various build and distribution fixes

## [0.1.0-alpha] - 2025-11-27

### Added

- Initial implementation with complexity analysis and dead code detection
- tree-sitter based JavaScript/TypeScript parsing
- CLI with analyze command
- Configuration file support (jscan.config.json)

[Unreleased]: https://github.com/ludo-technologies/jscan/compare/v0.6.1...HEAD
[0.6.1]: https://github.com/ludo-technologies/jscan/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/ludo-technologies/jscan/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/ludo-technologies/jscan/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/ludo-technologies/jscan/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/ludo-technologies/jscan/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/ludo-technologies/jscan/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/ludo-technologies/jscan/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/ludo-technologies/jscan/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/ludo-technologies/jscan/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/ludo-technologies/jscan/compare/v0.1.0-alpha...v0.1.0
[0.1.0-alpha]: https://github.com/ludo-technologies/jscan/releases/tag/v0.1.0-alpha

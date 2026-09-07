# polyscan analyze

Runs the full analysis and produces a report. This is the command you reach for when you want to understand a codebase.

```bash
polyscan analyze [path...]
```

`analyze` always exits with code 0 when the analysis itself succeeds, no matter how poor the results are. To fail a pipeline on the results, gate on the JSON output as shown on the [CI/CD page](../integrations/ci-cd.md).

## Synopsis

```bash
polyscan analyze .                                       # All analyses, HTML report
polyscan analyze --select complexity,deadcode src/       # Two analyses only
polyscan analyze --format json src/                      # JSON to standard output
polyscan analyze --format text src/                      # Text to standard output
polyscan analyze --no-open src/                          # HTML report, no browser
polyscan analyze -o reports/quality.html src/            # Custom output path
polyscan analyze --min-complexity 10 .                   # List only functions at or above 10
polyscan analyze src/ test/ scripts/build.ts             # Several paths at once
```

## Flags

| Flag | Short | Default | Description |
| --- | --- | --- | --- |
| `--select` | `-s` | `complexity,deadcode,clone,cbo,deps` | Comma-separated list of analyses to run. `deps` applies to Go and JavaScript/TypeScript; `deadcode` and `cbo` to JavaScript/TypeScript only |
| `--format` | `-f` | `html` | Output format. Accepts `html`, `json`, or `text` |
| `--no-open` | | `false` | Write the HTML report without opening a browser |
| `--output` | `-o` | `polyscan-report.html` | Path for the HTML report file |
| `--min-complexity` | | `1` | List only functions with at least this complexity. Scores and summaries still cover every function |

The configuration file for the JavaScript/TypeScript analysis is discovered automatically; there is no `--config` flag. See [how polyscan finds your config file](../configuration/index.md#how-polyscan-finds-your-config-file).

## Language coverage

The language of each file is detected from its extension. Complexity and clone detection cover every supported language. Dependency analysis covers Go and JavaScript/TypeScript. Dead code and coupling (CBO) exist for JavaScript/TypeScript only, and the health score is computed over the dimensions that ran: a dimension a language does not have is left out, not scored as clean.

For Go, Rust and C++, version control, dependency and build output directories are not walked: any directory whose name starts with a dot, and `node_modules`, `vendor`, `target`, `build`, `dist` and `third_party`. A path named on the command line is analyzed whatever it is called. JavaScript/TypeScript exclusions come from `jscan.config.json`. A file that cannot be read is skipped and listed under errors. A file with a syntax error is analyzed without the functions that contain it, counted as partial, and listed under warnings. C++ libraries hit this routinely, because a macro that opens a namespace or declares an attribute is a syntax error without the preprocessor.

## Output behavior

The format determines where results go and what else is printed.

=== "HTML (default)"

    polyscan writes the report to `polyscan-report.html` in the current directory, prints the absolute path, and opens the file in your default browser. The browser step is skipped when `--no-open` is set and also when polyscan detects that it is running inside an SSH session. A short score summary follows on standard output.

=== "JSON"

    The full result document goes to standard output. The score summary goes to standard error, so that redirecting standard output to a file keeps the JSON valid. Progress bars are suppressed. The [JSON schema page](../output/json-schema.md) documents every field.

=== "Text"

    The full human-readable report goes to standard output, ending with the health score breakdown. No separate summary is printed to standard error, because the text report already contains one.

## The five analyses

### Complexity {#complexity}

Measures the cyclomatic complexity of every function, which counts the number of linearly independent paths through it: one plus the number of decision points.

For JavaScript and TypeScript, polyscan builds a control flow graph for each function and derives the count from the graph. Starting from a baseline of 1, each of the following adds 1: an `if` or `else if`, a loop, a `catch`, a `case` label, a ternary, and each `&&`, `||`, or `??` operator. A `default` clause adds nothing, the same as `else`, and neither does optional chaining with `?.`. Counting each `case` label means a `switch` scores the same as the equivalent chain of `if` statements.

The other languages count decision points the same way on their own constructs:

| Language | Counted as decision points |
| --- | --- |
| Go | `if`/`else if`; every `for`; each `case` of a `switch`, type switch or `select` (`default` excluded); `&&` and `\|\|` |
| Rust | `if`/`else if`/`if let`; `for`, `while`, `while let`, `loop`; each `match` arm except the last, plus each arm guard; `let ... else`; `&&` and `\|\|`; `?` |
| C++ | `if`/`else if`; `?:`; `for`, range `for`, `while`, `do`; each `case` (`default` excluded); `catch`; `&&` and `\|\|` |

In Go, Rust and C++, function literals, closures and lambdas are not reported on their own: their decision points count toward the enclosing function, so the Go numbers match gocyclo.

Functions are assigned a risk level from two thresholds, configurable for JavaScript/TypeScript and fixed at the defaults for the other languages:

| Risk | Condition | Default range |
| --- | --- | --- |
| Low | complexity ≤ `low_threshold` | 1 to 9 |
| Medium | `low_threshold` < complexity ≤ `medium_threshold` | 10 to 19 |
| High | complexity > `medium_threshold` | 20 and above |

Functions below `--min-complexity` (or `output.min_complexity` in the config file, for JavaScript/TypeScript) are dropped from the listing; the summary and every score still cover the complete analyzed population.

### Dead code {#dead-code}

*JavaScript/TypeScript only.* Finds code that cannot run and code that nothing uses. polyscan detects two distinct kinds of problem.

Unreachable statements come from the control flow graph. Any basic block that cannot be reached from the function entry is dead. This covers statements after `return`, `break`, `continue`, and `throw`, and branches whose condition can never hold.

Unused declarations come from a cross-file pass that resolves imports and exports across every analyzed file. An exported function that no analyzed file imports is reported, as are unused imports and files that nothing references.

Every finding carries a reason code and a severity. The full set is:

| Reason code | Severity | What it means |
| --- | --- | --- |
| `unreachable_after_return` | Critical | A statement follows a `return` in the same block |
| `unreachable_after_throw` | Critical | A statement follows a `throw` |
| `unreachable_after_break` | Critical | A statement follows a `break` |
| `unreachable_after_continue` | Critical | A statement follows a `continue` |
| `unreachable_after_infinite_loop` | Critical | A statement follows a loop that never exits |
| `unreachable_branch` | Critical | A branch whose condition can never hold |
| `unused_import` | Warning | An imported name is never used in the importing file |
| `unused_exported_function` | Warning | An exported function or class is imported by no analyzed file |
| `orphan_file` | Warning | A file that no analyzed file imports |
| `unused_export` | Info | An exported value other than a function is imported by no analyzed file |

The six critical reasons come from the control flow graph. The four remaining ones come from the cross-file import and export pass.

!!! note "Cross-file findings depend on the paths you pass"

    The unused-export check can only see the files included in the run. Analyzing `src/components/` alone reports every export in it as unused, because the importers in the rest of `src/` were never read. Pass the whole source root when you care about this finding.

### Duplicate code {#clone}

Finds fragments that repeat, in every supported language. polyscan compares abstract syntax trees rather than raw text, using the APTED tree edit distance algorithm, so it still matches code whose variables have been renamed or whose statements have moved. To keep the comparison affordable on large codebases, polyscan first narrows the candidate set with MinHash fingerprints and locality-sensitive hashing, then runs the expensive tree comparison only on surviving pairs. Fragments of different languages are never compared with each other.

Clones are graded by how much the copies differ:

| Type | Description | Default threshold | Enabled by default (JS/TS) |
| --- | --- | --- | --- |
| Type 1 | Identical apart from whitespace and comments | 0.85 | Yes |
| Type 2 | Identical after renaming identifiers and literals | 0.75 | Yes |
| Type 3 | Copies with statements added, removed, or changed | 0.70 | No (JS/TS); reported for Go, Rust and C++ |
| Type 4 | Different code that computes the same thing | 0.65 | Yes (JS/TS only) |

A fragment must be at least 10 lines and 20 syntax tree nodes to be considered, which keeps short boilerplate such as getters out of the results.

In Go, Rust and C++, test code is analyzed for complexity but excluded from clone detection, because test functions share a skeleton by convention: for Go that is `*_test.go` files; for C++ it is `*_test.*`, `*_tests.*`, `test_*.*` and `*Test.*` source files and any `test` or `tests` directory; for Rust it is `#[test]` functions, items under `#[cfg(test)]`, `tests.rs` and `*_tests.rs` files and any `tests` directory.

### Coupling between objects {#cbo}

*JavaScript/TypeScript only.* Counts how many other types each unit depends on, across four kinds of dependency: imports, `new` expressions, TypeScript type annotations, and method calls on other objects.

!!! info "This metric is per file, not per class"

    Despite the name, polyscan produces one coupling entry per source file, named after the module. The terminal output and the JSON field names still say "classes", which they inherit from the shared metric definition. Read every count as a per-module count.

| Risk | Condition |
| --- | --- |
| Low | CBO ≤ 3 |
| Medium | 3 < CBO ≤ 7 |
| High | CBO > 7 |

### Dependencies {#deps}

*Go and JavaScript/TypeScript.* Builds the import graph and derives from it the circular imports, the maximum dependency depth, and the Martin coupling metrics for each module.

For JavaScript/TypeScript a module is a file. The graph resolves both ECMAScript modules and CommonJS `require` calls. Circular imports are found with Tarjan's strongly connected components algorithm. Dynamic imports are excluded from the cycle check, because a dynamic import does not create a load-time cycle.

For Go a module is a package. Each file's import paths resolve through the nearest `go.mod`: the package's import path is the module path plus its directory, so no path aliasing is involved. `_test.go` files are left out, because their imports describe the tests rather than the package, and so are the `vendor` and `testdata` directories the go tool ignores. An import of the standard library or of another module names no package in the tree and becomes no edge. The compiler forbids import cycles, so the cycle report is always empty for Go, and the value lies in the instability, the distance from the main sequence and the dependency depth. Abstractness is the share of a package's exported type declarations that are interfaces. A Go file outside any `go.mod` cannot be placed in a package; it is reported as a warning and, when no package resolves at all, the dependency dimension is left out of the score.

The full graph — nodes, edges, per-module metrics, cycles — is in the `deps` section of the [JSON output](../output/json-schema.md#deps).

## Performance notes

The analyses run concurrently, so the wall clock time is set by the slowest one rather than by the sum. Clone detection is normally the slowest, followed by dead code detection.

Dropping the analyses you do not need is the most effective way to speed up a run:

```bash
# Roughly half the work of a full run on most projects
polyscan analyze --select complexity,deadcode src/
```

The progress bar shown during an interactive run is driven by an elapsed-time estimate rather than by real progress, so it may sit near the end for a while on an unusually large repository. It is not stuck.

## See also

- [Output formats](../output/index.md) for the shape of each report
- [Configuration reference](../configuration/reference.md) for the keys that change the JavaScript/TypeScript thresholds
- [CI/CD integration](../integrations/ci-cd.md) for gating a pipeline on the results

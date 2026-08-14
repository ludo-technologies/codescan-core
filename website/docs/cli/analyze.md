# jscan analyze

Runs the full analysis and produces a report. This is the command you reach for when you want to understand a codebase rather than gate a pull request.

```bash
jscan analyze [path...]
```

`analyze` always exits with code 0 when the analysis itself succeeds, no matter how poor the results are. Use [`jscan check`](check.md) when you need a command that fails.

## Synopsis

```bash
jscan analyze src/                                    # All analyses, HTML report
jscan analyze --select complexity,deadcode src/       # Two analyses only
jscan analyze --json src/                             # JSON to standard output
jscan analyze --text src/                             # Text to standard output
jscan analyze --no-open src/                          # HTML report, no browser
jscan analyze -o reports/quality.html src/            # Custom output path
jscan analyze -c config/strict.json src/              # Explicit config file
jscan analyze src/ test/ scripts/build.ts             # Several paths at once
```

## Flags

| Flag | Short | Default | Description |
| --- | --- | --- | --- |
| `--select` | `-s` | `complexity,deadcode,clone,cbo,deps` | Comma-separated list of analyses to run |
| `--format` | `-f` | `html` | Output format. Accepts `html`, `json`, `text`, `yaml`, or `csv` |
| `--json` | | `false` | Shorthand for `--format json` |
| `--text` | | `false` | Shorthand for `--format text` |
| `--html` | | `false` | Shorthand for `--format html`, which is already the default |
| `--no-open` | | `false` | Write the HTML report without opening a browser |
| `--output` | `-o` | `jscan-report.html` | Path for the HTML report file |
| `--config` | `-c` | discovered | Path to a configuration file |

If both `--json` and `--text` are given, JSON wins.

## Output behavior

The format determines where results go and what else is printed.

=== "HTML (default)"

    jscan writes the report to `jscan-report.html` in the current directory, prints the absolute path, and opens the file in your default browser. The browser step is skipped when `--no-open` is set and also when jscan detects that it is running inside an SSH session. A short score summary follows on standard output.

=== "JSON"

    The full result document goes to standard output. The score summary goes to standard error, so that redirecting standard output to a file keeps the JSON valid. Progress bars are suppressed. The [JSON schema page](../output/json-schema.md) documents every field.

=== "YAML"

    The full result document goes to standard output formatted as YAML, matching the JSON response structure. The score summary goes to standard error. Progress bars are suppressed.

=== "CSV"

    Tabular results for complexity, dead code findings, clone pairs, and dependency edges go to standard output in comma-separated format. The score summary goes to standard error. Progress bars are suppressed.

=== "Text"

    The full human-readable report goes to standard output, ending with the health score breakdown. No separate summary is printed to standard error, because the text report already contains one.

## The five analyses

### Complexity {#complexity}

Measures the cyclomatic complexity of every function, which counts the number of linearly independent paths through it. jscan builds a control flow graph for each function and derives the count from the graph, so the number reflects real branching rather than a text heuristic.

Starting from a baseline of 1, each of the following adds 1: an `if` or `else if`, a loop, a `catch`, a `case` label, a ternary, and each `&&`, `||`, or `??` operator. A `default` clause adds nothing, the same as `else`, and neither does optional chaining with `?.`.

Counting each `case` label means a `switch` scores the same as the equivalent chain of `if` statements: a four-case switch and four `if` statements both score 5. The number of case labels seen in a function is reported as `switch_cases` in the JSON output.

Functions are assigned a risk level from two thresholds, both configurable:

| Risk | Condition | Default range |
| --- | --- | --- |
| Low | complexity ≤ `low_threshold` | 1 to 9 |
| Medium | `low_threshold` < complexity ≤ `medium_threshold` | 10 to 19 |
| High | complexity > `medium_threshold` | 20 and above |

Functions below `output.min_complexity` are dropped from the report entirely. The default is 1, which keeps everything.

### Dead code {#dead-code}

Finds code that cannot run and code that nothing uses. jscan detects two distinct kinds of problem.

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

Finds fragments that repeat. jscan compares abstract syntax trees rather than raw text, using the APTED tree edit distance algorithm, so it still matches code whose variables have been renamed or whose statements have moved. To keep the comparison affordable on large codebases, jscan first narrows the candidate set with MinHash fingerprints and locality-sensitive hashing, then runs the expensive tree comparison only on surviving pairs.

Clones are graded into four types:

| Type | Description | Default threshold | Enabled by default |
| --- | --- | --- | --- |
| Type 1 | Identical apart from whitespace and comments | 0.85 | Yes |
| Type 2 | Identical after renaming identifiers and literals | 0.75 | Yes |
| Type 3 | Copies with statements added, removed, or changed | 0.70 | No |
| Type 4 | Different code that computes the same thing | 0.65 | Yes |

Type 3 is off by default because near-miss matches produce a high false positive rate in day-to-day use.

A fragment must be at least 10 lines and 20 syntax tree nodes to be considered, which keeps short boilerplate such as getters out of the results.

### Coupling between objects {#cbo}

Counts how many other types each unit depends on, across four kinds of dependency: imports, `new` expressions, TypeScript type annotations, and method calls on other objects.

!!! info "In jscan this metric is per file, not per class"

    Despite the name, jscan produces one coupling entry per source file, named after the module. The terminal output and the JSON field names still say "classes", which they inherit from the shared metric definition. Read every count as a per-module count.

| Risk | Condition |
| --- | --- |
| Low | CBO ≤ 3 |
| Medium | 3 < CBO ≤ 7 |
| High | CBO > 7 |

### Dependencies {#deps}

Builds the module import graph, resolving both ECMAScript modules and CommonJS `require` calls. From the graph jscan derives the circular imports, the maximum dependency depth, and the Martin coupling metrics for each module.

Circular imports are found with Tarjan's strongly connected components algorithm. Dynamic imports are excluded from the cycle check, because a dynamic import does not create a load-time cycle.

For a graph you can look at rather than a summary, use [`jscan deps`](deps.md).

## Performance notes

The five analyses run concurrently in separate goroutines, so the wall clock time is set by the slowest one rather than by the sum. Clone detection is normally the slowest, followed by dead code detection.

Dropping the analyses you do not need is the most effective way to speed up a run:

```bash
# Roughly half the work of a full run on most projects
jscan analyze --select complexity,deadcode src/
```

The progress bar shown during an interactive run is driven by an elapsed-time estimate rather than by real progress, so it may sit near the end for a while on an unusually large repository. It is not stuck.

## See also

- [`jscan check`](check.md) for a command that fails on threshold violations
- [Output formats](../output/index.md) for the shape of each report
- [Configuration reference](../configuration/reference.md) for the keys that change these thresholds

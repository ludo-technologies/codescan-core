# JSON Schema

`polyscan analyze --format json` writes a single document to standard output, covering every analyzed language. This page describes its structure. The score summary is written to standard error, so redirecting standard output gives you valid JSON with nothing else mixed in.

```bash
polyscan analyze --format json src/ > report.json
```

## Top level

```json
{
  "version": "0.1.0",
  "generated_at": "2026-08-04T16:20:39+09:00",
  "duration_ms": 5,
  "complexity": { },
  "dead_code": { },
  "clone": { },
  "cbo": { },
  "deps": { },
  "module_quality": [ ],
  "summary": { }
}
```

| Field | Type | Notes |
| --- | --- | --- |
| `version` | string | The polyscan version. `dev` for a locally built binary |
| `generated_at` | string | RFC 3339 timestamp |
| `duration_ms` | integer | Wall clock time of the whole run |
| `complexity` | object | Present only when complexity analysis ran |
| `dead_code` | object | Present only when dead code detection ran (JavaScript/TypeScript) |
| `clone` | object | Present only when clone detection ran |
| `cbo` | object | Present only when coupling analysis ran (JavaScript/TypeScript) |
| `deps` | object | Present only when dependency analysis ran (Go, JavaScript/TypeScript) |
| `module_quality` | array | Per-file rollups joined across the analyses that ran |
| `summary` | object | Always present |

The five analysis keys are omitted entirely when `--select` excludes them or when no analyzed language has them, so consumers should check for their presence rather than assume it.

## `summary`

This is the object most scripts want. It carries the health score, the per-category scores, and the headline counts.

```json
{
  "total_files": 2,
  "analyzed_files": 2,
  "skipped_files": 0,
  "complexity_enabled": true,
  "dead_code_enabled": true,
  "clone_enabled": true,
  "cbo_enabled": true,
  "deps_enabled": true,
  "arch_enabled": false,
  "deps_total_modules": 2,
  "deps_modules_in_cycles": 0,
  "deps_max_depth": 1,
  "deps_main_sequence_deviation": 0.6,
  "arch_compliance": 0,
  "total_functions": 4,
  "functions_parsed": 4,
  "average_complexity": 4.5,
  "high_complexity_count": 0,
  "medium_complexity_count": 0,
  "dead_code_files": 2,
  "dead_code_count": 3,
  "critical_dead_code": 1,
  "warning_dead_code": 2,
  "info_dead_code": 0,
  "total_clones": 0,
  "clone_pairs": 0,
  "clone_groups": 0,
  "code_duplication_percentage": 0,
  "cbo_classes": 2,
  "high_coupling_classes": 0,
  "medium_coupling_classes": 0,
  "average_coupling": 0.5,
  "total_loc": 49,
  "project_scale": "Micro",
  "health_score": 91,
  "grade": "A",
  "complexity_score": 100,
  "dead_code_score": 65,
  "duplication_score": 100,
  "coupling_score": 100,
  "dependency_score": 85,
  "architecture_score": 0
}
```

A few fields need explanation.

`total_functions` is every function polyscan analyzed, in every language, which is what every complexity figure here and the health score describe. `--min-complexity` and the other report filters change which functions `complexity.functions` lists, never what is counted here. `functions_parsed` is retained for output compatibility and carries the same number.

The `*_enabled` flags say which dimensions actually ran and therefore which the health score was computed over. A dimension can be off because `--select` excluded it or because no analyzed language has it: a pure Go project reports `dead_code_enabled: false` even under the default selection. See [the health score page](health-score.md#the-formula).

`cbo_classes`, `high_coupling_classes`, and `medium_coupling_classes` count modules rather than classes, despite the names. See [the analyze reference](../cli/analyze.md#cbo).

`arch_enabled` is always `false` and `architecture_score` is always `0`, because architecture validation is not implemented in polyscan. Ignore both.

`grade` is one of `A`, `B`, `C`, `D`, `F`, or `N/A`. The last appears only when the summary failed validation, in which case `health_score` is 0 as well.

`total_files`, `analyzed_files` and `skipped_files` describe the whole run, whichever analyses were selected: a file that could not be read or parsed is counted as skipped and charged the parse-error penalty even when complexity analysis did not run. The complexity object below carries its own file counts, which cover only the files that analysis saw, and `dead_code_files` counts the JavaScript/TypeScript files the dead code analysis covered, which is the divisor of the dead code penalty.

`project_scale` is a size label derived from `analyzed_files`. See [Project scale](health-score.md#project-scale) for the thresholds. `total_loc` is the number of lines the clone analysis read, so it is `0` when clone analysis is turned off.

## `complexity`

```json
{
  "version": "0.1.0",
  "generated_at": "2026-08-04T16:20:39+09:00",
  "functions": [ ],
  "by_directory": [ ],
  "summary": { },
  "warnings": [ ],
  "errors": [ ],
  "config": { }
}
```

Each entry in `functions` looks like this:

```json
{
  "name": "classify",
  "file_path": "main.go",
  "language": "Go",
  "start_line": 5,
  "start_column": 1,
  "end_line": 16,
  "metrics": {
    "complexity": 4,
    "nodes": 0,
    "edges": 0,
    "nesting_depth": 2,
    "if_statements": 0,
    "loop_statements": 0,
    "exception_handlers": 0,
    "switch_cases": 0
  },
  "risk_level": "low"
}
```

`language` names the language the function was analyzed as: `JavaScript`, `TypeScript`, `Go`, `Rust`, or `C++`.

`nesting_depth` is the deepest chain of nested control structures in the function, in every language: the function body is depth 0, an `else if` continues the chain its `if` opened rather than starting a deeper one, a `catch` clause stays at the level of its `try`, and a nested function is measured separately. A Go function literal, a Rust closure or a C++ lambda is not a separate function, so the control structures inside it count toward the function that contains it. For JavaScript/TypeScript functions, `nodes` and `edges` describe the control flow graph the complexity was derived from. For the other languages the graph and statement-breakdown fields are `0`, because their complexity is counted from decision points rather than a control flow graph. `risk_level` is `low`, `medium`, or `high`.

File paths for JavaScript/TypeScript are reported exactly as polyscan resolved them, which means they are absolute when you passed an absolute path and relative when you passed a relative one. Go, Rust and C++ paths are shortened to be relative to the working directory when the file lies under it.

`by_directory` groups the reported functions by the directory they live in, always present and empty when nothing was reported:

```json
{
  "directory_path": "src/checkout",
  "function_count": 12,
  "average_complexity": 4.75,
  "max_complexity": 14,
  "high_risk_function_count": 1,
  "average_nesting_depth": 1.5,
  "max_nesting_depth": 3
}
```

`directory_path` is relative to the deepest directory that contains every analyzed file, so the shared prefix of your selection is stripped and files sitting directly in that directory are reported as `.`. The rows are ranked worst first: high-risk functions, then maximum complexity, then average complexity. They describe every analyzed function rather than the ones the report lists, so `--min-complexity` does not shrink them along with `functions`.

The `summary` object describes the same complete population:

```json
{
  "total_functions": 7,
  "functions_parsed": 7,
  "average_complexity": 1.71,
  "max_complexity": 3,
  "min_complexity": 1,
  "files_analyzed": 2,
  "total_files": 2,
  "skipped_files": 0,
  "low_risk_functions": 7,
  "medium_risk_functions": 0,
  "high_risk_functions": 0,
  "complexity_distribution": {
    "1": 4,
    "2": 1,
    "3": 2
  }
}
```

`complexity_distribution` counts the analyzed functions that have each cyclomatic complexity, keyed by that complexity. It is the one field that lets a consumer plot the distribution a filtered report was scored on. It is absent when no function was analyzed.

`total_files` counts the files the request covered and `skipped_files` those that could not be read or parsed. Their contents are absent from every figure above, so read `skipped_files` before trusting the aggregates.

## `dead_code`

*JavaScript/TypeScript only.*

```json
{
  "version": "0.1.0",
  "generated_at": "2026-08-04T16:20:39+09:00",
  "files": [ ],
  "summary": { },
  "warnings": [ ],
  "errors": [ ],
  "config": { }
}
```

Each entry in `files` separates findings inside functions from findings about the file as a whole:

```json
{
  "file_path": "/home/you/project/src/cart.ts",
  "functions": [
    {
      "name": "totalPrice",
      "file_path": "/home/you/project/src/cart.ts",
      "findings": [
        {
          "location": {
            "file_path": "/home/you/project/src/cart.ts",
            "start_line": 20,
            "end_line": 20,
            "start_column": 0,
            "end_column": 0
          },
          "function_name": "totalPrice",
          "code": "",
          "reason": "unreachable_after_return",
          "severity": "critical",
          "description": "Code after return statement is unreachable"
        }
      ],
      "total_blocks": 18,
      "dead_blocks": 1,
      "reachable_ratio": 0.7777777777777778,
      "critical_count": 1,
      "warning_count": 0,
      "info_count": 0
    }
  ],
  "file_level_findings": [ ],
  "total_findings": 2,
  "total_functions": 2,
  "affected_functions": 1,
  "dead_code_ratio": 0.5
}
```

Findings inside a function appear under `functions[].findings`. Findings about the file itself, such as an unused export or an orphan file, appear under `file_level_findings`. A consumer that reads only one of the two will miss findings.

Both arrays may be `null` rather than empty when a file has no findings of that kind, so guard for it.

The `code` field is present in the schema but is not populated, because context extraction is controlled by `dead_code.show_context`, which polyscan does not read yet.

The `reason` values are listed in [the analyze reference](../cli/analyze.md#dead-code). `severity` is `critical`, `warning`, or `info`.

The `summary` object carries a `findings_by_reason` map, which is the most convenient thing to chart over time:

```json
{
  "total_files": 2,
  "total_functions": 4,
  "total_findings": 3,
  "files_with_dead_code": 2,
  "functions_with_dead_code": 1,
  "critical_findings": 1,
  "warning_findings": 2,
  "info_findings": 0,
  "findings_by_reason": {
    "unreachable_after_return": 1,
    "unused_exported_function": 2
  },
  "total_blocks": 42,
  "dead_blocks": 1,
  "overall_dead_ratio": 0.023809523809523808
}
```

## `clone`

```json
{
  "version": "0.1.0",
  "generated_at": "2026-08-04T16:20:39+09:00",
  "duration_ms": 3,
  "clone_pairs": [ ],
  "clone_groups": [ ],
  "statistics": { },
  "success": true
}
```

`clone_pairs` lists every pair of similar fragments with its similarity value and clone type. `clone_groups` collects fragments that are mutually similar, which is the more useful view when the same code has been copied several times. Every fragment carries a `language` field, and fragments of different languages are never paired with each other. An `error` field appears alongside `success` when detection failed.

`statistics.total_fragments` and `statistics.total_clones` are the two numbers behind the duplication percentage in the summary, which is `total_clones ÷ total_fragments × 100`.

## `cbo`

*JavaScript/TypeScript only.*

```json
{
  "version": "0.1.0",
  "generated_at": "2026-08-04T16:20:39+09:00",
  "classes": [ ],
  "summary": { },
  "warnings": [ ],
  "errors": [ ],
  "config": { }
}
```

!!! warning "This section uses different field naming"

    Entries in `classes` use PascalCase keys, while every other part of the document uses snake_case. This is an inconsistency in the output rather than in this documentation, and a parser written against the rest of the schema will not read this section correctly.

```json
{
  "Name": "report",
  "FilePath": "/home/you/project/src/report.js",
  "StartLine": 1,
  "EndLine": 11,
  "Metrics": {
    "CouplingCount": 1,
    "InheritanceDependencies": 0,
    "TypeHintDependencies": 0,
    "InstantiationDependencies": 0,
    "AttributeAccessDependencies": 0,
    "ImportDependencies": 1,
    "DependentClasses": ["cart"]
  },
  "RiskLevel": "low",
  "IsAbstract": false,
  "BaseClasses": null
}
```

`CouplingCount` is the CBO value. `DependentClasses` names what this module depends on, and the four `*Dependencies` counters break the total down by how the dependency was formed.

## `deps`

*Go and JavaScript/TypeScript.*

```json
{
  "version": "0.1.0",
  "generated_at": "2026-08-04T16:20:46+09:00",
  "graph": { },
  "analysis": { }
}
```

`graph` holds the nodes and edges of the import graph. A node is a JavaScript/TypeScript file, identified by its path, or a Go package, identified by its import path with `name` holding the package name and `file_path` its directory. Every node carries the `abstractness` the graph builder measured: the export count for a JavaScript module, the share of exported types that are interfaces for a Go package. A Go edge joins two packages once, with `weight` counting the files that import the target, and carries no `location`. `analysis` holds the derived results, including the circular dependencies, the maximum depth, the per-module Martin metrics, and the coupling analysis with its Zone of Pain and main-sequence module lists.

## `module_quality`

One entry per file, joining what each analysis measured about it, across every language:

```json
{
  "module_name": "cart",
  "file_path": "src/cart.ts",
  "lines_of_code": 214,
  "analyzed_function_count": 12,
  "average_complexity": 4.75,
  "max_complexity": 14,
  "high_risk_function_count": 1,
  "exception_handler_count": 2,
  "dead_code_finding_count": 3,
  "dead_code_block_count": 1
}
```

`module_name` comes from dependency analysis and is omitted when that analysis did not run or the file is not JavaScript/TypeScript; Go dependency nodes are packages, not files, so they name no entry here. The complexity columns come from complexity analysis and the dead-code columns from dead code detection, so a run that skipped one of them leaves those columns at zero rather than dropping the file.

Unlike the rest of the report, these counts are taken before the presentation filters: `--min-complexity` and `min_severity` change what `complexity.functions` and `dead_code.files` show without changing what a module is measured as carrying. The entries are ranked worst first: high-risk functions, then maximum complexity, then average complexity, then dead-code findings.

## Recipes

```bash
# Just the score, for a dashboard
polyscan analyze --format json src/ 2>/dev/null | jq '.summary.health_score'

# Fail a script below grade B
score=$(polyscan analyze --format json src/ 2>/dev/null | jq '.summary.health_score')
[ "$score" -ge 75 ] || { echo "Score $score is below 75"; exit 1; }

# The ten most complex functions as a table, with their language
polyscan analyze --format json --select complexity src/ 2>/dev/null \
  | jq -r '.complexity.functions[:10][]
           | [.metrics.complexity, .language, .name, "\(.file_path):\(.start_line)"]
           | @tsv'

# Every critical dead code finding, from both places they can appear
polyscan analyze --format json --select deadcode src/ 2>/dev/null \
  | jq -r '.dead_code.files[]
           | (.functions // [] | .[].findings // []), (.file_level_findings // [])
           | .[] | select(.severity == "critical")
           | "\(.location.file_path):\(.location.start_line) \(.reason)"'

# Count findings by reason
polyscan analyze --format json --select deadcode src/ 2>/dev/null \
  | jq '.dead_code.summary.findings_by_reason'
```

## Stability

The JSON shape is not covered by a stability guarantee yet. Fields are more likely to be added than removed, but treat the structure as subject to change between releases, and pin the polyscan version in any pipeline that parses it.

# JSON Schema

`jscan analyze --json` writes a single document to standard output. This page describes its structure. The score summary is written to standard error, so redirecting standard output gives you valid JSON with nothing else mixed in.

```bash
jscan analyze --json src/ > report.json
```

## Top level

```json
{
  "version": "0.4.1",
  "generated_at": "2026-08-04T16:20:39+09:00",
  "duration_ms": 5,
  "complexity": { },
  "dead_code": { },
  "clone": { },
  "cbo": { },
  "deps": { },
  "summary": { }
}
```

| Field | Type | Notes |
| --- | --- | --- |
| `version` | string | The jscan version. `dev` for a locally built binary |
| `generated_at` | string | RFC 3339 timestamp |
| `duration_ms` | integer | Wall clock time of the whole run |
| `complexity` | object | Present only when complexity analysis ran |
| `dead_code` | object | Present only when dead code detection ran |
| `clone` | object | Present only when clone detection ran |
| `cbo` | object | Present only when coupling analysis ran |
| `deps` | object | Present only when dependency analysis ran |
| `summary` | object | Always present |

The five analysis keys are omitted entirely when `--select` excludes them, so consumers should check for their presence rather than assume it.

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

`total_functions` is the count **after** the `output.min_complexity` filter. `functions_parsed` is the count before it. When the two differ, the report you are looking at is a filtered view.

`cbo_classes`, `high_coupling_classes`, and `medium_coupling_classes` count modules rather than classes in jscan, despite the names. See [the analyze reference](../cli/analyze.md#cbo).

`arch_enabled` is always `false` and `architecture_score` is always `0`, because architecture validation is not implemented in jscan. Ignore both.

`grade` is one of `A`, `B`, `C`, `D`, `F`, or `N/A`. The last appears only when the summary failed validation, in which case `health_score` is 0 as well.

`project_scale` is a size label derived from `analyzed_files`. See [Project scale](health-score.md#project-scale) for the thresholds. `total_loc` is the number of lines the clone analysis read, so it is `0` when clone analysis is turned off.

## `complexity`

```json
{
  "version": "0.4.1",
  "generated_at": "2026-08-04T16:20:39+09:00",
  "functions": [ ],
  "summary": { },
  "warnings": [ ],
  "errors": [ ],
  "config": { }
}
```

Each entry in `functions` looks like this:

```json
{
  "name": "shippingCost",
  "file_path": "/home/you/project/src/cart.ts",
  "start_line": 25,
  "start_column": 7,
  "end_line": 35,
  "metrics": {
    "complexity": 9,
    "nodes": 16,
    "edges": 22,
    "nesting_depth": 0,
    "if_statements": 6,
    "loop_statements": 1,
    "exception_handlers": 1,
    "switch_cases": 0
  },
  "risk_level": "low"
}
```

`nodes` and `edges` describe the control flow graph the complexity was derived from. `risk_level` is `low`, `medium`, or `high`, determined by your configured thresholds.

File paths are reported exactly as jscan resolved them, which means they are absolute when you passed an absolute path and relative when you passed a relative one.

## `dead_code`

```json
{
  "version": "0.4.1",
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

The `code` field is present in the schema but is not populated, because context extraction is controlled by `dead_code.show_context`, which jscan does not read yet.

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
  "version": "0.4.1",
  "generated_at": "2026-08-04T16:20:39+09:00",
  "duration_ms": 3,
  "clone_pairs": [ ],
  "clone_groups": [ ],
  "statistics": { },
  "success": true
}
```

`clone_pairs` lists every pair of similar fragments with its similarity value and clone type. `clone_groups` collects fragments that are mutually similar, which is the more useful view when the same code has been copied several times. An `error` field appears alongside `success` when detection failed.

`statistics.total_fragments` and `statistics.total_clones` are the two numbers behind the duplication percentage in the summary, which is `total_clones ÷ total_fragments × 100`.

## `cbo`

```json
{
  "version": "0.4.1",
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

```json
{
  "version": "0.4.1",
  "generated_at": "2026-08-04T16:20:46+09:00",
  "graph": { },
  "analysis": { }
}
```

`graph` holds the nodes and edges. `analysis` holds the derived results, including `circular_dependencies`, `max_depth`, and `coupling_analysis`.

The same structure is produced by `jscan deps --format json`, which is the better command to use when you want only this section.

## `jscan check --json`

The check command emits a different and much smaller document. It is documented on [the check page](../cli/check.md#json-output).

## Recipes

```bash
# Just the score, for a dashboard
jscan analyze --json src/ 2>/dev/null | jq '.summary.health_score'

# Fail a script below grade B
score=$(jscan analyze --json src/ 2>/dev/null | jq '.summary.health_score')
[ "$score" -ge 75 ] || { echo "Score $score is below 75"; exit 1; }

# The ten most complex functions as a table
jscan analyze --json --select complexity src/ 2>/dev/null \
  | jq -r '.complexity.functions[:10][]
           | [.metrics.complexity, .name, "\(.file_path):\(.start_line)"]
           | @tsv'

# Every critical dead code finding, from both places they can appear
jscan analyze --json --select deadcode src/ 2>/dev/null \
  | jq -r '.dead_code.files[]
           | (.functions // [] | .[].findings // []), (.file_level_findings // [])
           | .[] | select(.severity == "critical")
           | "\(.location.file_path):\(.location.start_line) \(.reason)"'

# Count findings by reason
jscan analyze --json --select deadcode src/ 2>/dev/null \
  | jq '.dead_code.summary.findings_by_reason'
```

## Stability

The JSON shape is not covered by a stability guarantee yet. Fields are more likely to be added than removed, but treat the structure as subject to change between releases, and pin the jscan version in any pipeline that parses it.

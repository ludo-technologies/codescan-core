# jscan check

Runs a subset of the analyses against thresholds and sets a process exit code. This is the command to put in a continuous integration pipeline.

```bash
jscan check [path...]
```

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Every check passed |
| 1 | At least one threshold was violated |
| 2 | The analysis could not run, for example because a path does not exist or no source file was found |

Distinguishing 1 from 2 matters in a pipeline. Exit code 1 means jscan worked and your code did not meet the bar. Exit code 2 means jscan itself could not do its job, which usually points at a wrong path or a misconfigured job.

## Synopsis

```bash
jscan check src/                                # Defaults
jscan check --max-complexity 10 src/            # Fail above complexity 10
jscan check --allow-dead-code src/              # Report dead code without failing
jscan check --allow-circular-deps src/          # Tolerate circular imports
jscan check --max-cycles 3 src/                 # Tolerate up to three cycles
jscan check --select complexity,deps src/       # Skip dead code detection
jscan check --json src/                         # Machine-readable result
jscan check --verbose src/                      # Show locations and a summary
```

## Flags

| Flag | Short | Default | Description |
| --- | --- | --- | --- |
| `--max-complexity` | | `10` | Fail when any function exceeds this cyclomatic complexity |
| `--allow-dead-code` | | `false` | Report dead code findings without failing |
| `--allow-circular-deps` | | `false` | Report circular imports without failing |
| `--max-cycles` | | `0` | Number of dependency cycles tolerated before failing |
| `--select` | `-s` | `complexity,deadcode,deps` | Analyses to run |
| `--verbose` | `-v` | `false` | Print file locations and a summary block |
| `--json` | | `false` | Emit the result as JSON on standard output |
| `--config` | `-c` | discovered | Path to a configuration file |

`--select` accepts only `complexity`, `deadcode`, and `deps`. Clone detection and coupling analysis are not available in `check`, because both are too slow for a gate. Run them through [`jscan analyze`](analyze.md) instead.

When you do not pass `--max-complexity` on the command line, jscan uses `complexity.max_complexity` from the configuration file if that value is greater than 0. An explicit flag always wins over the file.

## The default gate is strict

Running `jscan check` with no flags fails on **any** dead code finding, including warnings. The threshold is zero findings, not zero critical findings.

```console
$ jscan check src/
FAIL: Quality check failed
  Violations: 2
  [ERROR] deadcode: Found 1 critical dead code issues
  [WARN] deadcode: Found 2 warning-level dead code issues
```

The warning that dominates real projects is `unused_exported_function`, which fires whenever an exported function is not imported by another analyzed file. In a library package that is the normal state of every public export, since the importers live in other repositories entirely. The same happens when you analyze one subdirectory of a larger application.

Most projects should therefore start permissive and tighten over time:

```bash
# A reasonable starting point
jscan check --allow-dead-code --max-complexity 15 src/
```

Once the obvious problems are fixed, drop `--allow-dead-code` and lower `--max-complexity` toward 10.

## What each check does

### Complexity

Every function is compared against `--max-complexity`. Each function above the limit produces its own violation, so the count in the output is a count of functions rather than of files.

```text
[ERROR] complexity: Function 'handleRequest' has complexity 24
       at src/server/router.ts:88
```

The location line appears only with `--verbose`.

### Dead code

Unless `--allow-dead-code` is set, any finding at all fails the check. jscan emits at most two violations here, one aggregating the critical findings and one aggregating the warnings, each carrying a count rather than a per-finding entry. Use [`jscan analyze`](analyze.md) when you need the individual locations.

Findings at info severity are counted in the totals but do not produce a violation of their own.

### Dependencies

Circular imports fail the check when their count exceeds `--max-cycles`, which defaults to 0. Setting `--allow-circular-deps` disables the check regardless of `--max-cycles`.

With `--verbose`, each individual cycle is listed after the summary violation, one line per cycle, with the module path that forms it.

## Verbose output

`--verbose` changes both the passing and the failing output. On success it reports what was checked:

```console
$ jscan check --verbose --allow-dead-code src/
PASS: All quality checks passed
  Files analyzed: 2
  Duration: 3ms
  Complexity: checked (max: 10)
  Dead code: checked
  Dependencies: checked
```

On failure it adds the source location of each violation and a closing summary with the totals.

## JSON output

`--json` writes a single document to standard output and suppresses the progress display. The exit code is unchanged, so the pipeline still fails on a violation.

```json
{
  "passed": false,
  "exit_code": 1,
  "violations": [
    {
      "category": "deadcode",
      "rule": "no-dead-code",
      "severity": "error",
      "message": "Found 1 critical dead code issues",
      "actual": "1",
      "threshold": "0"
    }
  ],
  "summary": {
    "files_analyzed": 2,
    "total_violations": 1,
    "complexity_checked": true,
    "deadcode_checked": true,
    "deps_checked": true,
    "high_complexity_functions": 0,
    "dead_code_findings": 3,
    "circular_dependencies": 0
  },
  "duration_ms": 2,
  "generated_at": "2026-08-04T16:20:56+09:00",
  "version": "0.4.1"
}
```

Violations carry a `location` field as well when the check that produced them knows one, which today means the complexity check.

| Category | Rule | Raised when |
| --- | --- | --- |
| `complexity` | `max-complexity` | A function exceeds the complexity limit |
| `deadcode` | `no-dead-code` | Dead code was found and `--allow-dead-code` is unset |
| `deps` | `max-cycles` | Cycle count exceeds `--max-cycles` |
| `deps` | `circular-dependency` | One entry per cycle, added only with `--verbose` |

## See also

- [CI/CD integration](../integrations/ci-cd.md) for complete pipeline configurations
- [`jscan analyze`](analyze.md) for the detailed findings behind a failure

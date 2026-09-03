# Quick Start

This page walks through a first run on a real project. It takes about five minutes and assumes you have polyscan available, either through `npx` or through a global install. If you do not, see [installation](installation.md).

## 1. Run the analysis

Point polyscan at the directory that holds your source:

```bash
polyscan analyze .
```

polyscan collects every supported source file below that directory — JavaScript, TypeScript, Go, Rust and C++, detected by extension — runs the analyses in parallel, writes an HTML report to `polyscan-report.html`, and opens it in your browser. It also prints a summary to the terminal:

```console
$ polyscan analyze .
HTML report written to /home/you/project/polyscan-report.html

📊 Analysis Summary:
Health Score: 95/100 (Grade: A)
Project Scale: Micro (3 files, 7 functions, 43 LOC)
Total time: 6ms

📈 Detailed Scores:
  Complexity:      100/100 ✅  (avg: 1.9, high-risk: 0 functions)
  Dead Code:        80/100 ✅  (3 issues, 1 critical)
  Duplication:     100/100 ✅  (0.0% fragments cloned, 0 groups)
  Coupling (CBO):  100/100 ✅  (avg: 1.0, 0/2 high-coupling)
  Dependencies:     95/100 ✅  (0 cycles, depth: 1)
```

The health score is a single number from 0 to 100 with a letter grade attached, computed over the dimensions that ran. Complexity and duplication cover every language; dead code, coupling and dependencies come from the JavaScript/TypeScript files, and a project without any lists only the dimensions it has. The [health score page](../output/health-score.md) gives the exact formula. The project scale line reports the size of the repository and does not affect the score.

!!! tip "Working over SSH or in a container?"

    polyscan skips the browser step automatically when it detects an SSH session. To suppress it in any other situation, pass `--no-open`.

## 2. Read the terminal output instead

If you would rather stay in the terminal, ask for text output. This prints the full findings to standard output and writes no file:

```bash
polyscan analyze --format text src/
```

The text report lists every function with its complexity, every dead code finding with its file and line, every clone group, the coupling table, and the dependency summary. It ends with the same health score breakdown.

## 3. Narrow the analysis

Running every analysis on a large repository takes longer than you may want during an edit and re-run loop. Use `--select` to run only the ones you care about:

```bash
# Only complexity
polyscan analyze --select complexity src/

# Complexity and clone detection together
polyscan analyze --select complexity,clone src/
```

The five values accepted by `--select` are `complexity`, `deadcode`, `clone`, `cbo`, and `deps`; the last three apply to JavaScript/TypeScript files only. Omitting the flag runs all five.

## 4. Gate a pipeline on the result

`polyscan analyze` always exits 0 when the analysis itself succeeds, because its job is to report rather than to judge. To fail a pipeline, gate on the JSON output:

```bash
polyscan analyze --format json src/ > report.json
jq -e '.summary.health_score >= 75' report.json
```

`jq -e` exits non-zero when the expression is false, which fails the CI step. The [CI/CD page](../integrations/ci-cd.md) has complete pipeline configurations and more precise gates, such as failing only on critical dead code.

## 5. Configure the JavaScript/TypeScript analysis

The JavaScript/TypeScript analysis reads a `jscan.config.json` when the project has one — the configuration format carried over from jscan, the analyzer that merged into polyscan. It supplies complexity thresholds and exclude patterns:

```json title="jscan.config.json"
{
  "complexity": {
    "low_threshold": 10,
    "medium_threshold": 20
  }
}
```

Be aware that polyscan reads only part of that file. The [configuration guide](../configuration/index.md) states plainly which keys take effect and which are accepted but not applied. There is no configuration file for the other languages yet.

## Where to go next

- The [CLI reference](../cli/index.md) documents every command and flag.
- The [configuration reference](../configuration/reference.md) lists every key in the config file.
- The [CI/CD page](../integrations/ci-cd.md) has ready-made GitHub Actions and GitLab CI jobs.
- The [TypeScript guide](../guides/typescript-projects.md) covers path aliases, type-only imports, and monorepos.
- Arriving from jscan? The [migration page](migrating-from-jscan.md) maps every old command to its replacement.

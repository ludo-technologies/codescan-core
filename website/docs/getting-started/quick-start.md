# Quick Start

This page walks through a first run on a real project. It takes about five minutes and assumes you have jscan available, either through `npx` or through a global install. If you do not, see [installation](installation.md).

## 1. Run the analysis

Point jscan at the directory that holds your source:

```bash
jscan analyze src/
```

jscan collects every JavaScript and TypeScript file below that directory, runs all five analyses in parallel, writes an HTML report to `jscan-report.html`, and opens it in your browser. It also prints a summary to the terminal:

```console
$ jscan analyze src/
Analyzing 2 files...
📊 Unified HTML report generated and opened: /home/you/project/jscan-report.html

📊 Analysis Summary:
Health Score: 91/100 (Grade: A)
Total time: 5ms

📈 Detailed Scores:
  Complexity:      100/100 ✅  (avg: 4.5, high-risk: 0 functions)
  Dead Code:        65/100 ⚠️  (3 issues, 1 critical)
  Duplication:     100/100 ✅  (0.0% fragments cloned, 0 groups)
  Coupling (CBO):  100/100 ✅  (avg: 0.5, 0/2 high-coupling)
  Dependencies:     85/100 ✅  (0 cycles, depth: 1)
```

The health score is a single number from 0 to 100 with a letter grade attached. It starts at 100 and subtracts a penalty for each category. The [health score page](../output/health-score.md) gives the exact formula.

!!! tip "Working over SSH or in a container?"

    jscan skips the browser step automatically when it detects an SSH session. To suppress it in any other situation, pass `--no-open`.

## 2. Read the terminal output instead

If you would rather stay in the terminal, ask for text output. This prints the full findings to standard output and writes no file:

```bash
jscan analyze --text src/
```

The text report lists every function with its complexity, every dead code finding with its file and line, every clone group, the coupling table, and the dependency summary. It ends with the same health score breakdown.

## 3. Narrow the analysis

Running all five analyses on a large repository takes longer than you may want during an edit and re-run loop. Use `--select` to run only the ones you care about:

```bash
# Only complexity
jscan analyze --select complexity src/

# Complexity and dead code together
jscan analyze --select complexity,deadcode src/
```

The five values accepted by `--select` are `complexity`, `deadcode`, `clone`, `cbo`, and `deps`. Omitting the flag runs all five.

## 4. Add a quality gate

`jscan analyze` always succeeds, because its job is to report rather than to judge. When you want a command that fails, use `jscan check`:

```console
$ jscan check src/
FAIL: Quality check failed
  Violations: 2
  [ERROR] deadcode: Found 1 critical dead code issues
  [WARN] deadcode: Found 2 warning-level dead code issues
$ echo $?
1
```

The exit code is 0 when everything passes, 1 when a threshold is violated, and 2 when the analysis itself could not run. That makes it usable directly as a continuous integration step.

!!! warning "The default gate is strict"

    With no flags, `jscan check` fails on any dead code finding at all, including the warning that an exported function is never imported. That warning fires constantly in library packages, whose exports are consumed outside the analyzed directory. Most projects should start with `jscan check --allow-dead-code src/` and tighten from there. The [check reference](../cli/check.md) covers each threshold flag.

## 5. Create a configuration file

Rather than repeating flags, write them down once:

```bash
jscan init
```

This creates `jscan.config.json` in the current directory. Add `--interactive` for a short wizard that asks about your framework and how strict you want the thresholds to be, then writes a file tuned to those answers.

Be aware that jscan reads only part of that file today. The [configuration guide](../configuration/index.md) states plainly which keys take effect and which are accepted but not yet applied.

## 6. Look at the dependency graph

The `deps` command exports your module graph in Graphviz DOT format:

```bash
jscan deps src/ --dot | dot -Tsvg -o deps.svg
```

This needs Graphviz installed, which provides the `dot` program. The [dependency graph guide](../guides/dependency-graph.md) explains how to read the colors and what the coupling numbers in each tooltip mean.

## Where to go next

- The [CLI reference](../cli/index.md) documents every command and flag.
- The [configuration reference](../configuration/reference.md) lists every key in the config file.
- The [CI/CD page](../integrations/ci-cd.md) has ready-made GitHub Actions and GitLab CI jobs.
- The [TypeScript guide](../guides/typescript-projects.md) covers path aliases, type-only imports, and monorepos.

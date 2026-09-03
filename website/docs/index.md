---
title: polyscan
hide:
  - navigation
---

<div class="polyscan-hero" markdown>
<div class="polyscan-hero__copy" markdown>

<p class="polyscan-hero__eyebrow">JavaScript · TypeScript · Go · Rust · C++</p>

# Find what to fix first

<p class="polyscan-hero__lede">polyscan reads your source, scores the codebase from 0 to 100, and produces an HTML report that ranks the problems worth your attention. One command covers every supported language in one report: it runs with no build step, no type checker, and no project configuration.</p>

```bash
npx polyscan analyze .
```

[Get started](getting-started/quick-start.md){ .md-button .md-button--primary }
[Browse the CLI reference](cli/index.md){ .md-button }

<p class="polyscan-hero__meta">Written in Go. Parses with tree-sitter. Runs analyses in parallel.</p>

</div>
<div class="polyscan-hero__art" markdown>

<span class="polyscan-score__grade">91/100 &nbsp;Grade A</span>

<div class="polyscan-score__row"><span class="polyscan-score__label">Complexity</span><span class="polyscan-score__bar"><span class="polyscan-score__fill" style="width:100%"></span></span><span class="polyscan-score__value">100</span></div>
<div class="polyscan-score__row"><span class="polyscan-score__label">Dead Code</span><span class="polyscan-score__bar"><span class="polyscan-score__fill polyscan-score__fill--warn" style="width:65%"></span></span><span class="polyscan-score__value">65</span></div>
<div class="polyscan-score__row"><span class="polyscan-score__label">Duplication</span><span class="polyscan-score__bar"><span class="polyscan-score__fill" style="width:100%"></span></span><span class="polyscan-score__value">100</span></div>
<div class="polyscan-score__row"><span class="polyscan-score__label">Coupling</span><span class="polyscan-score__bar"><span class="polyscan-score__fill" style="width:100%"></span></span><span class="polyscan-score__value">100</span></div>
<div class="polyscan-score__row"><span class="polyscan-score__label">Dependencies</span><span class="polyscan-score__bar"><span class="polyscan-score__fill" style="width:85%"></span></span><span class="polyscan-score__value">85</span></div>

</div>
</div>

## What polyscan measures

polyscan looks at your code from five angles. Each one contributes a category score, and the analyzed categories combine into a single health score. Complexity and duplicate code cover every supported language; dead code, dependencies and class design run for JavaScript and TypeScript, and a dimension a language does not have is left out of the score rather than counted as clean. The [health score page](output/health-score.md) explains exactly how the arithmetic works.

<div class="grid cards" markdown>

-   :material-sine-wave: **Complexity**

    ---

    Cyclomatic complexity per function, which counts the number of independent paths through the code, in every supported language. High values mark the functions that are hardest to read and to test.

    [Read more](cli/analyze.md#complexity)

-   :material-content-duplicate: **Duplicate code**

    ---

    Copy-pasted and structurally similar code, in every supported language. polyscan compares syntax trees rather than text, so it still matches fragments after variables have been renamed or statements reordered.

    [Read more](guides/reduce-duplicate-code.md)

-   :material-broom: **Dead code** <small>JS/TS</small>

    ---

    Statements that can never execute, exported functions that nothing imports, and imports that nothing uses. polyscan finds these by building a control flow graph for every function and walking it from the entry point.

    [Read more](cli/analyze.md#dead-code)

-   :material-graph-outline: **Dependencies** <small>JS/TS</small>

    ---

    The module import graph, including circular imports and the Martin coupling metrics, derived from resolving every import across the analyzed files.

    [Read more](guides/dependency-graph.md)

-   :material-vector-link: **Class design** <small>JS/TS</small>

    ---

    Coupling between objects, which counts how many other types each class or module depends on. Classes near the top of this list are the ones that break when anything else changes.

    [Read more](cli/analyze.md#cbo)

-   :material-code-json: **Machine-readable output**

    ---

    The same results as one JSON document per run, with the language on every function and clone fragment, for dashboards, bots, and CI gates built on `jq`.

    [Read more](output/json-schema.md)

</div>

## Three commands worth knowing

```bash
# Full analysis with an HTML report that opens in your browser
polyscan analyze .

# Full findings in the terminal, ending with the health score
polyscan analyze --format text src/

# JSON to stdout, for scripts and CI gates
polyscan analyze --format json src/ > report.json
```

## Working with Python instead?

The same analyses are available for Python in [pyscn](https://docs.codescan.dev/), which shares its core algorithms with polyscan. And if you are arriving from jscan, the JavaScript/TypeScript analyzer that merged into polyscan, the [migration page](getting-started/migrating-from-jscan.md) maps every old command to its replacement.

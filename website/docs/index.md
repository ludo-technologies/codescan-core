---
title: jscan
hide:
  - navigation
---

<div class="jscan-hero" markdown>
<div class="jscan-hero__copy" markdown>

<p class="jscan-hero__eyebrow">JavaScript and TypeScript</p>

# Find what to fix first

<p class="jscan-hero__lede">jscan reads your JavaScript and TypeScript source, scores the codebase from 0 to 100, and produces an HTML report that ranks the problems worth your attention. It runs as a single command and needs no build step, no type checker, and no project configuration.</p>

```bash
npx jscan analyze src/
```

[Get started](getting-started/quick-start.md){ .md-button .md-button--primary }
[Browse the CLI reference](cli/index.md){ .md-button }

<p class="jscan-hero__meta">Written in Go. Parses with tree-sitter. Runs analyses in parallel.</p>

</div>
<div class="jscan-hero__art" markdown>

<span class="jscan-score__grade">91/100 &nbsp;Grade A</span>

<div class="jscan-score__row"><span class="jscan-score__label">Complexity</span><span class="jscan-score__bar"><span class="jscan-score__fill" style="width:100%"></span></span><span class="jscan-score__value">100</span></div>
<div class="jscan-score__row"><span class="jscan-score__label">Dead Code</span><span class="jscan-score__bar"><span class="jscan-score__fill jscan-score__fill--warn" style="width:65%"></span></span><span class="jscan-score__value">65</span></div>
<div class="jscan-score__row"><span class="jscan-score__label">Duplication</span><span class="jscan-score__bar"><span class="jscan-score__fill" style="width:100%"></span></span><span class="jscan-score__value">100</span></div>
<div class="jscan-score__row"><span class="jscan-score__label">Coupling</span><span class="jscan-score__bar"><span class="jscan-score__fill" style="width:100%"></span></span><span class="jscan-score__value">100</span></div>
<div class="jscan-score__row"><span class="jscan-score__label">Dependencies</span><span class="jscan-score__bar"><span class="jscan-score__fill" style="width:85%"></span></span><span class="jscan-score__value">85</span></div>

</div>
</div>

## What jscan measures

jscan looks at your code from five angles. Each one contributes a category score, and the five combine into a single health score. The [health score page](output/health-score.md) explains exactly how the arithmetic works.

<div class="grid cards" markdown>

-   :material-broom: **Dead code**

    ---

    Statements that can never execute, exported functions that nothing imports, and imports that nothing uses. jscan finds these by building a control flow graph for every function and walking it from the entry point.

    [Read more](cli/analyze.md#dead-code)

-   :material-content-duplicate: **Duplicate code**

    ---

    Copy-pasted and structurally similar code. jscan compares syntax trees rather than text, so it still matches fragments after variables have been renamed or statements reordered.

    [Read more](guides/reduce-duplicate-code.md)

-   :material-sine-wave: **Complexity**

    ---

    Cyclomatic complexity per function, which counts the number of independent paths through the code. High values mark the functions that are hardest to read and to test.

    [Read more](cli/analyze.md#complexity)

-   :material-graph-outline: **Dependencies**

    ---

    The module import graph, including circular imports and the Martin coupling metrics. jscan can export the graph as Graphviz DOT for rendering.

    [Read more](guides/dependency-graph.md)

-   :material-vector-link: **Class design**

    ---

    Coupling between objects, which counts how many other types each class or module depends on. Classes near the top of this list are the ones that break when anything else changes.

    [Read more](cli/analyze.md#cbo)

-   :material-check-decagram: **Quality gates**

    ---

    A separate command built for continuous integration. It runs a subset of the analyses, prints a short verdict, and sets a process exit code your pipeline can act on.

    [Read more](cli/check.md)

</div>

## Three commands worth knowing

```bash
# Full analysis with an HTML report that opens in your browser
jscan analyze src/

# Fast pass or fail gate for CI, which exits non-zero on a violation
jscan check src/

# Dependency graph rendered with Graphviz
jscan deps src/ --dot | dot -Tsvg -o deps.svg
```

## Working with Python instead?

The same analyses are available for Python in [pyscn](https://docs.codescan.dev/), which shares its core algorithms with jscan through the `polyscan` repository.

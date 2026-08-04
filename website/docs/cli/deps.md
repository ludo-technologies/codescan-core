# jscan deps

Builds the module dependency graph and writes it as text, JSON, or Graphviz DOT.

```bash
jscan deps [path...]
```

## Synopsis

```bash
jscan deps src/                                 # Text summary
jscan deps --dot src/ > deps.dot                # DOT to a file
jscan deps --dot src/ | dot -Tsvg -o deps.svg   # Render directly
jscan deps --format json src/                   # JSON for further processing
jscan deps --dot --min-coupling 5 src/          # Only the busiest modules
jscan deps --dot --rank-dir LR src/             # Left to right layout
jscan deps --include-external src/              # Include node_modules packages
```

## Flags

| Flag | Short | Default | Description |
| --- | --- | --- | --- |
| `--format` | `-f` | `text` | Output format. Accepts `text`, `json`, or `dot` |
| `--dot` | | `false` | Shorthand for `--format dot` |
| `--output` | `-o` | standard output | Write to this file instead |
| `--include-external` | | `false` | Include packages resolved into `node_modules` |
| `--include-types` | | `true` | Include TypeScript type-only imports as edges |
| `--no-cycles` | | `false` | Skip cycle detection, and stop grouping cycles in DOT output |
| `--max-depth` | | `0` | Limit the depth rendered in DOT output. 0 means unlimited |
| `--min-coupling` | | `0` | Render only nodes whose coupling is at least this value |
| `--no-legend` | | `false` | Omit the legend from DOT output |
| `--rank-dir` | | `TB` | DOT layout direction. Accepts `TB`, `LR`, `BT`, or `RL` |
| `--config` | `-c` | discovered | Path to a configuration file |

`--max-depth`, `--min-coupling`, `--no-legend`, and `--rank-dir` affect DOT rendering only. They are accepted but ignored for text and JSON output.

## Text output

The default format prints a summary of the graph rather than the graph itself:

```console
$ jscan deps src/
Analyzing 2 files...

=== Dependency Graph Analysis ===

Generated: 2026-08-04T16:20:46+09:00
Version: 0.4.1

Summary:
  Total modules: 2
  Total dependencies: 1
  Root modules (entry points): 1
  Leaf modules (no dependencies): 1
  Max depth: 1

Coupling Analysis:
  Average coupling: 1.00
  Average instability: 0.50
  Highly coupled modules: 0
  Stable modules: 1

Entry Points:
  - /home/you/project/src/report.js

Analysis completed in 1ms
```

A root module is one that nothing imports, which usually means an application entry point or a file only your tests reach. A leaf module imports nothing else in the analyzed set. Max depth is the length of the longest import chain.

## DOT output

`--dot` writes a Graphviz document. Render it with the `dot` program, which comes with a Graphviz installation:

```bash
jscan deps src/ --dot | dot -Tsvg -o deps.svg
jscan deps src/ --dot | dot -Tpng -o deps.png
```

Each node is colored by risk level and carries a tooltip with its coupling numbers:

| Color | Meaning |
| --- | --- |
| Green | Low risk |
| Yellow | Medium risk |
| Red | High risk |

The tooltip shows the module's role, its afferent coupling `Ca`, its efferent coupling `Ce`, and its instability. Those terms are defined on the [dependency graph guide](../guides/dependency-graph.md).

Unless `--no-cycles` is passed, modules that form a circular import are grouped into a visual cluster, which makes cycles easy to spot in a large graph.

Large graphs become unreadable quickly. Two flags help:

```bash
# Drop the leaves and keep the hubs
jscan deps src/ --dot --min-coupling 5 | dot -Tsvg -o hubs.svg

# Wide graphs usually read better horizontally
jscan deps src/ --dot --rank-dir LR | dot -Tsvg -o deps.svg
```

## JSON output

`--format json` emits the full graph together with the analysis results, which suits scripting and dashboards:

```bash
# List every module involved in a circular import
jscan deps src/ --format json \
  | jq -r '.analysis.circular_dependencies.circular_dependencies[].description'
```

The document contains a `graph` object with the nodes and edges, and an `analysis` object holding the cycle detection results, the depth calculation, and the coupling metrics.

## External and type-only dependencies

By default the graph contains only your own modules. Packages from `node_modules` are left out, which keeps the picture about your architecture rather than about your dependency tree. Pass `--include-external` when you want to see which third-party packages each module pulls in.

TypeScript type-only imports, written as `import type { Foo } from "./foo"`, are included by default because they still express a design dependency. They disappear at compile time, so pass `--include-types=false` if you want the graph to reflect only what exists at runtime.

## See also

- [Reading the dependency graph](../guides/dependency-graph.md) for what the metrics mean
- [`jscan check`](check.md) for failing a build on circular imports

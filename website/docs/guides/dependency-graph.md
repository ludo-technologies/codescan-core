# Read the Dependency Graph

`polyscan analyze --select deps` builds the import graph of the Go packages and the JavaScript/TypeScript files and reports metrics derived from it. For Go the module in every metric below is a package, resolved through `go.mod`; the Go compiler rejects import cycles, so the cycle section is always empty for Go. The numbers come from Robert Martin's package metrics, which are widely used but easy to misread. This guide explains what each one means and what to do about a bad value.

```bash
polyscan analyze --format text --select deps src/
polyscan analyze --format json --select deps src/ 2>/dev/null | jq '.deps.analysis'
```

## The vocabulary

Every metric is built from two counts. For a given module:

**Afferent coupling, written `Ca`.** The number of modules that import this one. High `Ca` means many things depend on you.

**Efferent coupling, written `Ce`.** The number of modules this one imports. High `Ce` means you depend on many things.

The two describe opposite kinds of exposure. A module with high `Ca` is dangerous to change, because the change ripples outward. A module with high `Ce` is fragile, because it breaks when anything it uses changes.

In the JSON output, both counts are per module in `.deps.analysis.ModuleMetrics`, as `AfferentCoupling` and `EfferentCoupling`, alongside `Instability`, `Abstractness`, `Distance`, and a `RiskLevel`.

## Instability

```text
instability = Ce ÷ (Ca + Ce)
```

The result runs from 0 to 1.

An instability of **0** means nothing this module imports and many things import it. It is maximally stable: hard to change, safe to depend on. Type definitions and shared constants belong here.

An instability of **1** means it imports things and nothing imports it. It is maximally unstable: easy to change, and nothing breaks when you do. Application entry points and route handlers belong here.

Neither extreme is a problem in itself. The problem is a module in the wrong place. A stable module is only appropriate when its contents rarely need to change, so a stable module full of business rules is a liability, since every rule change forces a change to something many modules depend on.

The rule of thumb is that dependencies should point toward stability. A module should import things more stable than itself. When an unstable module is imported by a stable one, the stable module inherits the instability of what it depends on.

## Abstractness and the main sequence

Abstractness measures how much of a module is interface rather than implementation. Instability measures how exposed it is to change. Plotting the two against each other gives a line from `(0, 1)` to `(1, 0)`, called the main sequence, which represents a healthy balance: concrete modules should be unstable, abstract modules should be stable.

The **main sequence deviation** is the distance from that line, between 0 and 1. Zero is on the line and 1 is as far from it as possible.

The two bad corners are:

**Abstract and unstable**, at the top right. The module is full of interfaces that nothing uses. This is usually over-engineering, or the remains of an abstraction whose implementations were deleted.

**Concrete and stable**, at the bottom left. Many modules import a module full of concrete implementation. This is the more common and more painful case, since every change to that implementation touches everything downstream. The fix is to extract an interface and let dependents import that instead.

The coupling analysis in the JSON output names the modules in each zone explicitly: `.deps.analysis.CouplingAnalysis.ZoneOfPain` lists the stable-but-concrete modules, and `MainSequence` the healthy ones. The deviation feeds the health score, contributing up to 3 penalty points. See [the health score page](../output/health-score.md#dependencies).

## Circular dependencies

A cycle is a group of modules that import each other, directly or through a chain. polyscan finds them with Tarjan's strongly connected components algorithm, which finds every cycle in one pass.

Cycles matter for practical reasons rather than aesthetic ones. Module initialization order becomes undefined, so one module in the cycle sees a partially initialized version of another, which shows up as `undefined` at runtime in a way that is hard to trace. Bundlers cannot tree-shake across a cycle, so dead code survives into your bundle. Neither module can be understood or tested alone.

Dynamic imports written as `import("./module")` are excluded from cycle detection, because a dynamic import resolves at call time rather than at load time and therefore does not create a load-time cycle. This is often the cheapest way to break a cycle you cannot otherwise untangle.

Find them in the text report's dependency section, or from the JSON:

```bash
polyscan analyze --format json --select deps src/ 2>/dev/null \
  | jq '.deps.analysis.CircularDependencies'
```

Or fail the build on them:

```bash
polyscan analyze --format json --select deps src/ 2>/dev/null \
  | jq -e '.summary.deps_modules_in_cycles == 0'
```

The usual fix is to find the thing both modules need and move it into a third module that both import. A cycle between `user.ts` and `order.ts` usually means there is a shared type or helper wanting to live in its own file.

## Depth

Maximum depth is the length of the longest import chain. polyscan compares it against what a healthy graph of your size would have:

```text
expected depth = max(3, ⌈log₂(module count + 1)⌉ + 1)
```

A graph of 100 modules is expected to be about 8 deep. Exceeding the expectation costs up to 3 points in the health score.

Deep chains make change expensive, because a modification at the bottom must be understood at every level above it. They usually come from layering that has grown one wrapper at a time. The chains themselves are listed in `.deps.analysis.LongestChains`.

## Rendering the graph

There is no built-in image export, but the nodes and edges are in the JSON output under `.deps.graph`, so a few lines of `jq` produce a Graphviz document:

```bash
polyscan analyze --format json --select deps src/ 2>/dev/null \
  | jq -r '"digraph deps {",
           (.deps.graph.edges[][] | "  \"\(.from)\" -> \"\(.to)\";"),
           "}"' \
  | dot -Tsvg -o deps.svg
```

Rendering needs Graphviz, which provides the `dot` program. Install it with `brew install graphviz`, `apt install graphviz`, or the equivalent for your system. The HTML report's Architecture tab covers most needs without any of this: it lists every module with its metrics, the cycles, and the longest chains.

## A caution about path aliases

polyscan does not resolve TypeScript path aliases. An import written `@/lib/util` becomes a module node of its own rather than resolving to `src/lib/util.ts`, which inflates the module count and drops the real edge.

Every metric on this page is computed from the graph, so every one of them is affected. In a project that uses aliases throughout, treat the coupling numbers as a lower bound and the cycle list as incomplete. Projects using relative imports get accurate results. The [TypeScript guide](typescript-projects.md#path-aliases-are-not-resolved) covers this in detail.

## A worked reading

```console
$ polyscan analyze --format text --select deps src/
=== Dependency Analysis ===

Summary:
  Total modules: 2
  Total dependencies: 1
  Entry points: 1
  Leaf modules: 1
  Max depth: 1

No circular dependencies detected.
```

Two modules and one edge between them. One module imports nothing, so it is a leaf and a stable module with instability 0. The other imports it and nothing imports the other, so it is an entry point with instability 1.

There are no cycles and the depth of 1 is well under the expected 3, so the only dependency penalty comes from the main sequence deviation.

## See also

- [`polyscan analyze` reference](../cli/analyze.md#deps) for the analysis itself
- [JSON schema](../output/json-schema.md#deps) for where these numbers live in the output
- [Health score](../output/health-score.md#dependencies) for how these metrics affect the score

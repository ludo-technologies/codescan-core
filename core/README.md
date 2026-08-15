# polyscan core

Shared Go module that provides language-agnostic code analysis algorithms for the polyscan family of analyzers: [pyscn](https://github.com/ludo-technologies/pyscn) (Python) and [jscan](https://github.com/ludo-technologies/polyscan/tree/main/jscan) (JavaScript/TypeScript, in this monorepo).

## Packages

| Package | Description |
|---------|-------------|
| `apted/` | APTED tree edit distance algorithm with configurable cost models and normalization |
| `cbo/` | Coupling Between Objects (CBO) metric over language-provided dependency references |
| `cfg/` | Control Flow Graph data structures, reachability analysis, McCabe cyclomatic complexity, dead code detection |
| `clone/` | AST feature extraction, clone classification, 5 clone grouping strategies (Connected, KCore, StarMedoid, CompleteLinkage, Centroid), and group dedup post-passes |
| `dfa/` | Data-flow analysis framework over CFGs with language-injected variable reference extraction |
| `domain/` | Shared type definitions, thresholds, and error taxonomy |
| `graph/` | Directed graph abstraction, Tarjan SCC cycle detection, longest dependency chains over the SCC condensation, Robert Martin coupling metrics |
| `lcom/` | LCOM4 lack-of-cohesion metric over method/attribute access maps |
| `lsh/` | Locality-Sensitive Hashing index and MinHash signatures |
| `nesting/` | Nesting depth analysis with language-injected nesting classifiers |
| `semantic/` | CFG structural feature extraction for semantic clone similarity |
| `source/` | File collection with include/exclude filters |
| `util/` | Common utilities (browser opener) |

## Install

```
go get github.com/ludo-technologies/polyscan/core
```

## Design

Language-specific behavior is injected via interfaces and callbacks:

- **`graph.DirectedGraph`** — pyscn's `map[string]*ModuleNode` and jscan's `domain.DependencyGraph` both implement this
- **`cfg.StatementClassifier`** — abstracts return/break/continue/throw detection across languages
- **`cfg.ComplexityContributor`** — allows language-specific complexity additions (e.g. jscan's logical operators)
- **`graph.CouplingConfig.AbstractnessFunc`** — injects language-specific abstractness calculation
- **`clone.GroupingStrategy[T]`** — generic grouping with Go 1.24 generics for type-safe usage
- **`apted.CostModel`** — language-specific tree edit cost models (PythonCostModel, JavaScriptCostModel stay in each project)
- **`dfa.RefExtractor`** — language-specific extraction of variable definitions and uses
- **`nesting.NestingClassifier`** — language-specific rules for which AST nodes introduce nesting

## Requirements

Go 1.24+

## License

MIT

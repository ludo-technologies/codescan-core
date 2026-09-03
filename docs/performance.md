# Performance characteristics

How the JavaScript/TypeScript analysis in polyscan (formerly jscan) and the shared `core/` algorithms behave as a codebase grows, which
knobs change that behaviour, and how to reproduce the numbers.

## Where the time goes

A default `polyscan analyze` run over JavaScript/TypeScript reads and parses every file once into a shared
`service.ProjectSnapshot`, then executes five analyses concurrently over the
shared parse trees. Complexity and deadcode also share per-file CFG
construction through the snapshot — the CFGs are built by whichever analysis
reaches a file first.

| Analysis | Dominant cost | Scaling |
|---|---|---|
| (snapshot) | read + parse, once for all analyses | linear in source size |
| complexity | CFG construction (shared with deadcode) | linear in source size |
| deadcode | CFG construction + reachability + module analysis | linear in source size |
| cbo | import/dependency extraction | linear in source size |
| deps | graph construction, SCC, chain search | linear in modules + edges |
| clone | APTED tree edit distance over fragment pairs | **superlinear** — see below |

Clone detection dominates every other analysis combined on any codebase past a
few thousand lines. The other four are effectively "parse the files", and their
combined throughput tracks parsing speed — which is why parsing once instead of
five times collapsed their cost.

### Clone detection is the one superlinear stage

Clone detection compares code fragments pairwise. Exhaustively that is O(n²) in
fragment count, with each comparison costing an APTED tree edit distance —
itself roughly O(m²·keyroots) in fragment size.

Two mechanisms keep this tractable:

1. **LSH candidate generation.** Above 500 fragments (or `lsh.auto_threshold`
   when a config file sets it), or when the estimated exhaustive pair count
   exceeds the retained-pair cap, MinHash signatures and a banded LSH index
   replace the full pairwise sweep with a candidate set. This turns the
   quadratic sweep into roughly linear candidate generation plus verification of
   the candidates only.
2. **Cheap rejection before APTED.** Candidate pairs pass a MinHash similarity
   estimate and a feature-Jaccard pre-filter before any tree edit distance is
   computed. Only survivors reach APTED.

Candidates are generated and verified in bounded chunks rather than collected up
front. Candidate count scales with fragments times candidates-per-query, which
on a large repository is orders of magnitude more pairs than are ever reported,
so holding them all would cost more memory than the analysis itself.

Fragments larger than 500 nodes take a bounded heuristic path rather than exact
APTED (`core/apted`: label/shape profile distance, capped key roots), so a
single very large function cannot dominate a run.

## Measured throughput

Measured on an Apple M4 (10 cores) against
[vuejs/core](https://github.com/vuejs/core) `packages/`, 147k lines of
TypeScript across 452 files, all five analyses enabled, JSON output. Each figure
is the median of three runs on an otherwise idle machine.

| Corpus | Lines | Wall time | Throughput | Peak RSS |
|---|---|---|---|---|
| `packages/reactivity/src` | 3.9k | 0.04 s | ~98k LOC/s | 44 MB |
| `packages/compiler-core/src` | 11k | 0.13 s | ~85k LOC/s | 95 MB |
| `packages/runtime-core/src` | 21k | 0.18 s | ~117k LOC/s | 154 MB |
| `packages/` (whole repo) | 147k | 7.4 s | ~20k LOC/s | 1.05 GB |

Throughput drops on the largest corpus because clone detection's candidate set
grows faster than the source does — the other four analyses stay flat.

For scale, the same machine and corpus before the shared snapshot (each
analysis parsing independently) measured 8.5 s and 1.86 GB peak RSS on the
whole repo: sharing one set of parse trees cut peak memory by ~44% and, on the
parse-bound smaller corpora, wall time by a third or more.

Per-analysis figures on the full 147k-line corpus, measured in isolation
(`BenchmarkPipeline*`, same machine; a standalone run reads and parses inline,
so these include one parse pass):

| Analysis | Wall time | Throughput |
|---|---|---|
| complexity | 0.31 s | 485k LOC/s |
| cbo | 0.40 s | 371k LOC/s |
| deadcode | 0.42 s | 357k LOC/s |
| deps | 0.63 s | 238k LOC/s |
| clone | 6.6 s | 22k LOC/s |

Clone detection is 90% of a default run. Dropping it with `--select` puts a
147k-line codebase comfortably under two seconds.

## Memory

Peak resident memory is driven by the parse trees and the clone-detection
fragment set, which are both held for the whole run:

- Roughly **7–11 MB per 1k lines** of TypeScript at default settings, with the
  ratio improving as the corpus grows.
- 147k lines peaks around 1.05 GB.
- A run with several analyses selected shares one set of parse trees through
  `service.ProjectSnapshot`; the snapshot lives until the last analysis
  finishes, so peak memory carries one parse tree per file regardless of how
  many analyses run. What `--select` then changes is everything past parsing —
  clone fragments and APTED trees are the largest single block.
- A run with a single analysis selected has nobody to share with and skips the
  snapshot: each file is read, parsed, and analyzed inside the fan-out and
  released as soon as its results are extracted, so only about one parse tree
  per worker is live at a time. On the 147k-line corpus a complexity-only or
  deadcode-only run peaks under 140 MB. The exceptions hold whole-project
  state by nature: `deps` needs every module's AST at once to build the graph,
  and `clone` keeps every fragment's AST until it is converted for APTED.
- Clone detection releases each fragment's parser AST reference as soon as its
  APTED tree is built, so in a clone-only run parse trees are freed during
  fragment preparation, before the APTED sweep that dominates the run. In a
  shared-snapshot run the snapshot owns the trees instead, and the release's
  job is to keep fragments from pinning them beyond it. It depends on the tree
  converter not copying parser nodes into `TreeNode.OriginalNode`: parser
  nodes carry a parent pointer, so retaining one pins the whole file's AST.

Memory scales with source size, not with project history or file count.

Concurrency trades memory for time, in two ways. Several files are parsed at
once, so more parse trees are live simultaneously than in a serial run. And
because per-file results are collected into per-path slots and aggregated after
the fan-out finishes, every file's findings stay resident until the whole
analysis is done, where a serial loop could release each file's results as it
consumed them. On small inputs this shows up as a higher peak (83 MB rather than
56 MB for a 3.9k-line package) against a 5× reduction in wall time.

## Concurrency

Work is parallelized at two levels, both bounded by `runtime.NumCPU()`:

- **Across analyses.** The five analyses run concurrently over the shared
  snapshot.
- **Within an analysis.** Snapshot construction fans per-file read and parse
  out across workers, each analysis fans its per-file work over the parsed
  files the same way (`service.analyzeFilesConcurrently`), and clone detection
  verifies LSH candidate pairs across workers.

Clone detection's verification is parallel only on the LSH path. Below the LSH
auto-threshold the exhaustive sweep still runs on a single goroutine, so a
project small enough to skip LSH uses one core for the dominant analysis. That
sweep is quadratic but bounded, and it is fast in absolute terms — the cost
model and APTED work described above is what made it usable.

Results do not depend on scheduling. Per-file results are collected into
preallocated per-path slots, and clone pairs are assembled on a single goroutine
in candidate order, so pair IDs and clone identities are stable. A given input
produces byte-identical output on every run. The one exception is a run that is
cancelled part-way: workers abandon their remaining candidates wherever they
happen to be, so partial output is not reproducible.

The practical consequence: wall time improves close to linearly with core count
for the parse-bound analyses, and clone detection's LSH verification stage scales
the same way. Candidate *generation* (LSH indexing) remains single-threaded.

## Tuning

The single biggest lever is which analyses run at all:

```bash
polyscan analyze --select complexity,deadcode,cbo,deps src/   # drops the dominant cost
```

The rest live in the config file — polyscan reads `jscan.config.json`,
`.jscanrc.json`, and `.jscan.toml`, and falls back to `.pyscn.toml` or
`pyproject.toml` for a shared polyscan setup. Keys below are given in their TOML
form, largest effect first:

| Setting | Key | Default | Effect |
|---|---|---|---|
| Fragment floor | `analysis.min_lines` | 5 | Raising it cuts fragment count, and comparison work falls with the square of that |
| Fragment floor | `analysis.min_nodes` | 10 | Same, by AST size instead of line count |
| LSH acceleration | `lsh.enabled` | `auto` | `false` forces the exhaustive O(n²) sweep — viable only for small trees |
| LSH auto threshold | `lsh.auto_threshold` | 200 (500 with no config file) | Fragment count at which `auto` switches LSH on |
| MinHash pre-filter | `lsh.similarity_threshold` | 0.50 | Raising it rejects more candidates before any APTED work |
| LSH banding | `lsh.bands`, `lsh.rows`, `lsh.hashes` | 32 / 4 / 128 | More bands finds more candidates and costs more; more rows per band is stricter |

Raising `analysis.min_lines` is usually the cheapest large win.

## Reproducing the numbers

End-to-end benchmarks run against a corpus you point them at, and skip when the
environment variable is unset:

```bash
cd polyscan
JSCAN_BENCH_CORPUS=/path/to/js-or-ts-repo \
  go test -run='^$' -bench=BenchmarkPipeline -benchmem -timeout=30m ./internal/js/service/
```

Each benchmark reports `LOC/s` alongside the usual `ns/op` and allocation
counts.

Profiling the same corpus:

```bash
JSCAN_BENCH_CORPUS=/path/to/repo go test -run='^$' -bench=BenchmarkPipelineClone \
  -benchtime=1x -cpuprofile=cpu.prof -memprofile=mem.prof -o service.test ./service/
go tool pprof -http=:8080 service.test cpu.prof
go tool pprof -sample_index=alloc_space -http=:8081 service.test mem.prof
```

Algorithm-level benchmarks, which need no corpus:

```bash
cd core
go test -run='^$' -bench=. -benchmem ./apted/   # tree edit distance by tree size
go test -run='^$' -bench=. -benchmem ./lsh/     # MinHash signing, LSH build and query
```

`core/apted` covers 100–5000 node trees on both the exact and the bounded
large-tree path; `core/lsh` covers index construction at 1k/10k/100k fragments
plus the dense-bucket case where many fragments hash alike.

## Algorithmic notes

Complexities below are in fragment count `n`, fragment size `m`, module count
`V`, and import edge count `E`.

| Component | Complexity | Notes |
|---|---|---|
| `core/apted` exact | O(m²·keyroots) per pair | Used below 500 nodes; per-node insert/delete costs are precomputed once per comparison rather than per DP cell |
| `core/apted` large-tree | O(m) profile + capped DP | Bounded heuristic above 500 nodes, exact APTED above 2000 is not attempted |
| `core/lsh` MinHash | O(numHashes · features) per fragment | 128 hashes by default |
| `core/lsh` index build | O(bands) per fragment | Insert is constant-work per band; it does not scan the bucket |
| `core/lsh` query | O(bands + candidates) | Capped by `maxCandidates` |
| Clone detection with LSH | O(n · candidates) verifications | Each verification is one APTED comparison |
| Clone detection without LSH | O(n²) verifications | Exhaustive; the reason LSH auto-enables |
| `core/graph` SCC | O(V + E) | Tarjan |
| `core/graph` longest chain | O(V + E) | Condensation plus memoized DAG traversal — see below |
| `core/graph` top-N chains | O((V + E) · N) | Same condensation; every component keeps its N best chains |

The last two rows are worth stating explicitly: dependency-chain reporting must
never enumerate simple paths. On a real import graph with cycles, exhaustive
enumeration does not terminate in practical time.

What the chain guarantees, then, is that no other simple path crosses more
strongly connected components — it is maximal in dependency layers. Components
are weighted by module count when chains are compared, so a chain through a
large cycle outranks an equally deep chain through single modules; between
equally heavy chains, the one whose components sort first by name wins, so the
ranking depends on the graph alone and not on traversal order. Within a
component the route is not guaranteed maximal: the component that ends the chain
is walked greedily through as many members as it can reach, and components the
chain passes through are crossed by the shortest route between the edges that
enter and leave them. Recovering the longest route through a cycle is the
NP-hard problem again. `MaxDepth` counts the edges of that concrete chain, so it
is always a depth some real import path achieves.

`LongestChains(ctx, N)` ranks chains globally rather than per start node: each
component memoizes its N best chains, built from the memoized chains of the
components it depends on, and the global top N is drawn from those lists. A
chain's tail is a chain in its own right, so a reported list can hold a chain
and its suffix. Both the condensation and the ranking pass observe context
cancellation, so a caller with a deadline can abandon the search.

### `MaxDepth` values changed

Reported `deps` depth moved with this rewrite, and on graphs with cycles it
moved down. The previous implementation combined memoization with a
path-dependent cycle cutoff, which both made the result depend on traversal
order and let a chain count an edge back into a node it had already visited. A
graph that is one 30-module cycle reported a depth of 30; the longest simple
path through 30 nodes has 29 edges, which is what it reports now.

`MaxDepth` feeds `DependencyPenalty` in `core/domain/scoring.go`, so the
dependency score and the overall grade can move with it — by at most 3 points
out of 100, since the depth penalty is capped there. A project pinned to a
health-score threshold in CI may therefore see its score shift slightly on the
first run after upgrading, in either direction.

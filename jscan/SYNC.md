# pyscn → jscan sync management

Language-independent algorithms now live in `core/` and are consumed directly by both tools (jscan: Phase 2b, #9–#11; pyscn: Phase 3, [polyscan#12](https://github.com/ludo-technologies/polyscan/issues/12), completed 2026-07-23). Changes to those algorithms land in `core/` itself and need no porting between pyscn and jscan. This file now tracks only the **case-by-case** and **reference-only** areas — pyscn changes there are reviewed and selectively ported to jscan, manually or by an agent.
The `/sync-pyscn` command reads this file to run a sync.

- Upstream: https://github.com/ludo-technologies/pyscn (local clone: `../pyscn`)
- **Sync baseline SHA**: `81b7ec0cfceb64e1930237027684364fb8b0f07a` (2026-07-28, v1.29.0)
  - The initial catch-up (clone detection / scoring / misc, 2026-06-11–12) compared jscan against pyscn's state up to `fb3fe92`. Exceptions that were deliberately not ported are listed under "Pending changes".
  - Update this value to pyscn's `origin/main` HEAD on each sync run.

## Sync policy classification

| Classification | Meaning |
|---|---|
| **sync** | Language-independent algorithm. Port essentially all pyscn changes |
| **case-by-case** | Shared concept but needs language adaptation. Decide per change whether to port |
| **reference-only** | Language-specific. Never port code. Only report significant design-direction changes to a human |

No row currently uses **sync**: every area that was language-independent enough to sync 1:1 has moved into `core/` and is consumed directly by both tools (see Sync history). The classification stays defined here in case a newly-shared, not-yet-core-extracted algorithm shows up again.

## File mapping

### internal/analyzer

| pyscn | jscan | Classification | Notes |
|---|---|---|---|
| `internal/analyzer/apted_tree.go` | `internal/analyzer/apted_tree.go` | reference-only | jscan retains only the JS/TS parser-to-`core/apted.TreeNode` converter |
| `internal/analyzer/apted_cost.go` | `internal/analyzer/apted_cost.go` | reference-only | Cost models are language-specific (Python AST vs JS/TS AST) |
| `internal/analyzer/lsh_index.go` | `internal/analyzer/lsh_index.go` | case-by-case | Thin candidate-index adapter (int fragment IDs, deterministic order, `WithMaxCandidates` cap) kept local in both tools around `core/lsh`; not itself in core, so still worth diffing when either side changes it |
| `internal/analyzer/clone_detector.go` | `internal/analyzer/clone_detector.go` | case-by-case | Pipeline orchestration + fragment extraction stay language-side; grouping/dedup/gates/classifier come from `core/clone` |
| `internal/analyzer/cfg.go` | `internal/analyzer/cfg.go` | case-by-case | Both tools alias `core/cfg` types for internal source compatibility; rarely needs edits unless `core/cfg`'s public shape changes |
| `internal/analyzer/cfg_builder.go` | `internal/analyzer/cfg_builder.go` | reference-only | Control-flow semantics are language-specific (try/except/match vs try/catch/switch/hoisting) |
| `internal/analyzer/reachability.go` | `internal/analyzer/reachability.go` | case-by-case | Both tools retain a thin compatibility result projection over `core/cfg` reachability |
| `internal/analyzer/complexity.go` | `internal/analyzer/complexity.go` | case-by-case | JS contributes logical and ternary operators and enriches the shared result |
| `internal/analyzer/dead_code.go` | `internal/analyzer/dead_code.go` | case-by-case | JS retains line/snippet/reason enrichment and unused-code analysis |
| `internal/analyzer/cbo.go` | `internal/analyzer/cbo.go` | reference-only | AST traversal for dependency collection is language-specific |
| `internal/analyzer/coupling_metrics.go` | `internal/analyzer/coupling_metrics.go` | case-by-case | Both tools delegate Martin metrics/Tarjan-adjacent coupling math to `core/graph`, supplying their own abstractness signal and risk/zone summaries |
| `internal/analyzer/circular_detector.go` | `internal/analyzer/circular_detector.go` | case-by-case | Both tools delegate Tarjan SCC to `core/graph`; each filters dynamic/lazy imports and enriches cycle results independently |
| `internal/analyzer/dependency_graph.go` | `internal/analyzer/dependency_graph.go` | case-by-case | Graph construction is shared; ModuleInfo contents are language-specific |
| `internal/analyzer/textual_similarity.go` | *(adopted `core/clone`)* | case-by-case | Type-1 gate in `core/clone.TextualSimilarityAnalyzer`; jscan injects `removeJSComments` as `CommentStripper` (`javascript_comments.go`) |
| `internal/analyzer/module_analyzer.go` | `internal/analyzer/module_analyzer.go` | reference-only | Import resolution is language-specific (`__init__.py` vs ESM/CJS/Node builtins) |

### domain (scoring, type definitions)

| pyscn | jscan | Classification | Notes |
|---|---|---|---|
| `domain/analyze.go` | `domain/analyze.go` | case-by-case | Health-score summation, normalization, and grade mapping (`GradeFromScore`, `IsHealthyScore`) now come from `core/domain` in both tools — each has its own parity test against core (jscan: `analyze_test.go`; pyscn: `TestGradeParityWithCore`), so grade computation is identical by construction. `calculateComplexityPenalty` (jscan: high/medium count ratio, ESLint-aligned; pyscn: max of avg-McCabe/cognitive/nesting) and `calculateDeadCodePenalty` (jscan: per-file rate; pyscn: log-normalized) stay intentionally divergent — do not overwrite |
| `domain/system_analysis.go` | `domain/system_analysis.go` | case-by-case | Same as above |
| `domain/complexity.go` | `domain/complexity.go` | case-by-case | Keep thresholds and risk-level definitions aligned |
| `domain/clone.go` | `domain/clone.go` | case-by-case | Keep similarity thresholds and clone-type definitions aligned |
| `domain/cbo.go` | `domain/cbo.go` | case-by-case | |
| `domain/dead_code.go` | `domain/dead_code.go` | case-by-case | |
| `domain/output.go` | `domain/output.go` | case-by-case | Mind JSON output schema compatibility |
| `domain/errors.go` | `domain/errors.go` | case-by-case | |

### Claude Code plugin / Agent Skills

| pyscn | jscan | Classification | Notes |
|---|---|---|---|
| `.claude-plugin/marketplace.json` | `.claude-plugin/marketplace.json` | case-by-case | Marketplace/plugin naming differs (`pyscn-mcp` vs `jscan`) |
| `.claude-plugin/pyscn-mcp/.claude-plugin/plugin.json` | `.claude-plugin/jscan/.claude-plugin/plugin.json` | case-by-case | jscan has no MCP server, so the plugin is skills-only (no `mcpServers`) |
| `.claude-plugin/pyscn-mcp/skills/*/SKILL.md` | `.claude-plugin/jscan/skills/*/SKILL.md` | case-by-case | Same 4 skills (health-check / cli-analysis / refactoring / architecture-review). Content rewritten for jscan's CLI: `npx jscan@latest`, `--select clone` (singular), `--text`/`--json` to stdout (no `.pyscn/reports/`-style report dir), no LCOM/communities/mockdata/di, and the "MCP Alternative" sections dropped until jscan has an MCP server |

### jscan-specific (no upstream)

- `internal/analyzer/unused_code.go` — cross-file dead code detection (includes the Next.js App Router exception conventions)

### pyscn-specific, unported features (future porting candidates)

Out of sync scope, but reference points when considering porting features to jscan:

- Cognitive complexity: `cognitive_complexity.go`, `nesting_depth.go`, `raw_metrics.go`
- LCOM4: `lcom.go`, `domain/lcom.go` — the union-find (shared-variable + call-edge unions) and risk assessment now live in `core/lcom.ComputeLCOM4` / `AssessRisk` (pyscn Phase 3, PR #673); adopting this in jscan means pulling from `core/lcom` directly, not porting pyscn's original implementation. Python method/attribute extraction (decorator exclusion, `@property` reclassification, ctypes `_fields_`) stays pyscn-local
- DFA (unused-variable detection): `dfa.go`, `dfa_builder.go` — def-use chain construction and reaching-def linking now live in `core/dfa` (pyscn Phase 3, PR #673) via the `RefExtractor`/`ParamExtractor` extension points; Python def/use extraction stays pyscn-local
- DI anti-pattern detection: `di_antipattern_detector.go` plus `di_*.go`, `*_detector.go`, `framework_patterns.go`
- Split similarity-analysis structure (remaining): `structural_similarity.go`, `semantic_similarity.go`, `similarity_analyzer.go`, multi-dimensional `clone_classifier` — Type-1/2 gates are in `core/clone` (jscan adopted); semantic evidence penalties (disjoint literals, missing strong signals, incompatible return categories) now live in `core/semantic.ApplySemanticEvidence` (pyscn Phase 3, PR #673); structural similarity still fully unported
- Re-export resolution: `reexport_resolver.go`
- Improvement suggestions: `domain/suggestion.go`
- MCP server: `mcp/`, `cmd/pyscn-mcp/`
- Module / directory quality rollups: `domain/module_quality.go`, `app/module_quality.go`, `app/directory_complexity.go`, `service/directory_complexity_formatter.go` plus `AnalyzeResponse.ModuleQuality`, `ComplexityResponse.ByDirectory` / `ModuleRollups`, `DeadCodeResponse.ModuleRollups` — new pyscn feature (2026-07-26–27, #686 per-module hotspots + #533/#687 directory rollups) joining per-file complexity, dead-code and module metadata into a `module_quality` array, and aggregating reported complexity per project-root-relative directory into `by_directory`, with text/CSV/HTML tables for both. ~20 files across domain/app/service; rollups are deliberately computed *before* presentation filters (`min_complexity`, dead-code severity), which is why both response types carry a `json:"-"` `ModuleRollups` map. Two of the six module-complexity fields (`AverageCognitiveComplexity`) and both directory nesting fields have no jscan backing metric — jscan computes `NestingDepth` per function but not cognitive complexity. Tracked for jscan as [polyscan#31](https://github.com/ludo-technologies/polyscan/issues/31)
- Module community detection: `internal/analyzer/community_metrics.go`, `domain/community.go`, `app/community_usecase.go`, `service/community_analysis_service.go`, `service/community_formatter.go`, `service/community_config_loader.go` — new pyscn feature (2026-06-22–30, #582/#583/#602) clustering modules into communities (modularity, cross-community edges, bridge modules, package/layer alignment) with an opt-in Health Score penalty (`MaxCommunityPenalty`, max 10). Entirely new subsystem with CLI/config/MCP wiring across ~15 files; not present in jscan at all. The Health Score integration in `domain/analyze.go` only activates when `CommunitiesEnabled` and `CommunityCount >= 2`, so it has no effect on jscan's (unmodified) grade computation as long as the feature stays unported

## Pending changes

(Record changes here that a sync run decided not to port this time.)

Skipped during the clone detection catch-up (2026-06-11):

- **Docstring skipping** (`SkipDocstrings` and the related fields in `apted_tree.go` / `clone_detector.go` / `domain/clone.go`) — Python-specific. JS/TS ASTs have no docstring statements
- **Boilerplate cost model** (`NewPythonCostModelWithBoilerplateConfig`, `ReduceBoilerplateSimilarity` / `BoilerplateMultiplier`) — depends on Python framework patterns such as dataclass/Pydantic (`framework_patterns.go` is an unported feature)
- **Multi-dimensional classifier and DFA** (`CloneClassifier`, `EnableMultiDimensionalAnalysis` / `EnableSemanticAnalysis` / `EnableDFA`) — depend on pyscn-specific unported features (semantic/structural similarity, DFA)
- **Type-4 CFG gating** (`87babcc`, `9efebd4`) — changes inside `semantic_similarity.go` (unported). Port together with the semantic analysis
- **`CloneConfigurationLoader.MergeConfig`** — config-loader-layer change. jscan's loader implementation differs, so out of scope
- **star_medoid graph optimization** (`buildSimilarityGraph` / `mostSimilarMedoid`) — performance-only change, structurally different from jscan's StarMedoid implementation (iterative reassignment over domain.Clone). The `averageGroupSimilarity` change to count only existing pairs was ported
- **pyscn's removal of the content-less `ExtractFragments`** — jscan tests still use it, so both variants were kept

Skipped during the scoring catch-up (2026-06-11):

- **LCOM cohesion scoring** (`CohesionMediumWeight` / `CohesionSaturationRatio`, `calculateCohesionPenalty`, `CohesionScore`, the LCOM summary fields, `57fcc66`) — depends on the unported LCOM4 feature. Port together with `lcom.go` / `domain/lcom.go`
- **Cognitive complexity / RawMetrics fields in `domain/complexity.go`** (`8aa6d21`, `ce469ce`) — unported features
- **Mock/synthetic fixture detection** (`MockData*` fields, `15bd414`) — pyscn-specific lint feature
- **Suggestions** (`Suggestions` field, `1eaf9f6`) — depends on unported `domain/suggestion.go`
- **Analyze config plumbing** (`AnalyzeOutputFormatter`, `AnalyzeExecutionConfig`, `AnalyzeConfigurationLoader`, `Enabled`/`ReportUnchanged`/`Request` fields, `222d190`, `779200a`, `13644b4`, `e79a414`, `d6fd3da`, `ca75d12`) — jscan's config-loader layer differs, out of scope
- **Architecture style presets / warn rules / neutral prefixes / cohesion-responsibility validation** (`ad79460`, `1c9839d`, `61f0431`, `3708845`, `ddcf1c5`) — jscan does not implement architecture validation (the `ArchitectureAnalysisResult` types are unused scaffolding); only the `WeightedViolations` field was ported for type alignment
- **`AbstractClassCount`** (`f63540d`) — Python ABC detection; jscan computes abstractness its own way in `coupling_metrics.go` from TS interfaces/abstract classes
- **Django migration exclusion / Python default patterns** (`a33f1c7`, `DefaultPythonModuleIncludePatterns`) — Python-specific

Skipped during the misc catch-up (2026-06-12):

- **APTED DP-matrix reuse + parallel pair verification** (`c324d2f`, `e912ca2`) — landed in pyscn after the clone-detection catch-up point (`73acc60`). Large performance rework of `apted.go` / `apted_tree.go` / `clone_detector.go` (worker pool, `newWorkerDetector` / `effectiveWorkers`, key-root cache invalidation, streamed LSH verification). Port as its own dedicated change; the LSH int-ID/`WithMaxCandidates` part was already ported in the misc catch-up
- **complexity.go AST-based statement metrics + CognitiveComplexity** — the fix targets Python `match` decision-point double-counting (jscan counts JS branches its own way via logical/ternary operators), and `CalculateCognitiveComplexity` is an unported feature. Port together with `cognitive_complexity.go` if ever
- **cfg.go `ModuleNode` / `complexitySourceNode`** — unnecessary in jscan: the module-level CFG already sets `FunctionNode` to the Program node, so location/nesting info is available
- **`domain/defaults.go` constants refactor** (`DefaultAnalysisIncludePatterns`, `DefaultCBOLowThreshold` etc. in `domain/cbo.go` / `domain/dead_code.go`) — config-layer cosmetics with Python file patterns; jscan keeps its own defaults
- **`DeadCodeResponse.Request` field** — analyze-config plumbing (already skipped as a category during the scoring catch-up)

Skipped during the 2026-07-08 sync (`fb3fe92`..`249b121`):

- **`refactor(clone): integrate statistics into detector result types`** (`45b2058`) — pure internal Go API refactor (`DetectClones*` now returns a `*CloneDetectionResult` struct instead of a `([]*ClonePair, []*CloneGroup)` tuple); no algorithmic or output change. jscan's `CloneDetector`/service-layer split is already architecturally different (jscan builds `domain.Clone` directly via `fragmentToClone`, no separate service-layer conversion step), so there's nothing equivalent to restructure
- **CBO nested-class-scope exclusion** (`441ddc2`, `8b228b4`, issue #547) — excludes a locally/class-body-scoped nested class from CBO coupling counts, with Python-scoping-specific resolution (function-local vs. class-body visibility rules, method-signature-annotation evaluation scope). `cbo.go` is reference-only (AST traversal is language-specific); the underlying idea (don't count a helper class/function scoped inside the analyzed class as coupling) may be worth an independent JS/TS-native implementation later, but the resolution logic itself is not portable
- **CBO enum-member collapsing** (`67b243f`) — collapses `EnumClass.MEMBER`-style attribute reads into one coupling edge per imported class, keyed off Python's `import ... as ...` dotted-name resolution (`isImportedDependency`, `looksLikeClassReference`), which jscan's `cbo.go` has no equivalent of
- **CBO namespace-import grouping** (`06c76a4`, issue #517) — `GroupNamespaceImports` option collapsing `alias.Member` references for `import mod as alias`; Python-import-specific, with config plumbing through `domain.CBORequest`/service/config-loader layers that don't map onto jscan's config system
- **Cognitive-metrics-based risk / Health Score integration** (`3f159b4`) — makes `calculateComplexityPenalty` also weigh `AverageCognitiveComplexity` and `AverageNestingDepth` (unported features; see "pyscn-specific, unported features"). `calculateComplexityPenalty` is also flagged in the file mapping as an intentional jscan divergence (ESLint-aligned high/medium ratio vs. pyscn's average-complexity curve) — do not overwrite
- **Module community detection** (`9bc832e`, `1b0590b`, `785e72b`, `8b03fbb`, `fd0db46`, `7a63058`, #582/#583/#602) — new pyscn subsystem; see "pyscn-specific, unported features" for details

Skipped during the 2026-07-25 sync (`249b121`..`6af3ee7`):

- **CBO Python-scoping/import refinements** (`0a4c469`/#637 same-file top-level peer exclusion, `28d1378` name-collision hardening, `acbd2c8` module-constant exclusion, `826491b` PEP 563 annotation-only deps, `ff01735`/#644 Cython primitives, `c3edc9e` module-qualified enum coupling, `0238bab` acronym constructor coupling) — `cbo.go` is reference-only. Design direction worth noting: pyscn keeps tightening CBO noise exclusion; the *concepts* of same-file peer-class exclusion and module-constant exclusion may deserve an independent JS/TS-native implementation later
- **Sparse request bool overrides / sentinel unification** (`6a12c8b`, `f1e9a4b`, `a70b5a5`) — pyscn config-loader-layer rework (`FlagTracker` removal, `BoolPtr` sparse requests, `ShowDetails` response resolution). jscan's config-loader layer differs; already-skipped category (analyze-config plumbing)
- **LCOM4 regression test** (`21e512e`) — LCOM is an unported feature

Skipped during the 2026-07-28 sync (`6af3ee7`..`81b7ec0`):

- **Architecture-suggestion refinements** (`3e7d83c`/#647 dedup + `file_path`/`start_line`, `a146222` preserve distinct violations, `ab8f944` prioritize critical suggestions, `0edce40` identity-dimension tests, `5554baf` per-problem violation merge + shared category cap) — all in `domain/suggestion.go`, which is on the unported-features list ("Improvement suggestions"). jscan has no suggestion subsystem to apply these to; port them together with `domain/suggestion.go` if it is ever adopted
- **Module quality rollups** (`91075ec`, `270eb98`, `ca0c48b`, `ed9602c`, `499a05f`, `5e193e9`, `8c32d21`, `db2de9b`, `6a51bc8`, `0043feb`, plus `fe247fa`/#686 and the unmapped app/service/report commits `3093066`, `77af996`, `52ddb7f`, `9f60bd7`, `9394e7c`, `72cde27`, `df7803f`, `f2b7a3d`, `9487428`, `4c26413`) — new pyscn subsystem; see "pyscn-specific, unported features" and [polyscan#31](https://github.com/ludo-technologies/polyscan/issues/31)
- **Directory quality rollups** (`d985d0f`, `21c8636`, `f0713b7`, `3308c94`, `b6e5e3c`, `be35e03`, `6209e60`, plus the unmapped `39ca5ec`, `5b7226f`, `535c8e6`, `72337d2`, `af33f81`, `d9d8c64`, `ce8d982`, `d39ddef`, `86e5875`, `86d3ed2`, `3dd97bf`, `40dd367` — #533/#687) — second half of the same feature; same tracking issue. Note `d9d8c64` (buffer the analyze text report and propagate a single write error) is a genuinely portable formatter fix, but it exists only to wrap the new directory/module sections — jscan's `AnalyzeFormatter` writes with `fmt.Fprint` throughout and has no equivalent error-propagation seam to fix in isolation
- **MCP registry listing** (`300e1c9`, `9c3ab20`) — jscan has no MCP server

## Sync history

| Date | pyscn SHA | Summary |
|---|---|---|
| 2026-06-11 | `f0457d7` | Set the baseline to the state jscan's initial implementation was based on (v1.4.1). The ~210 commits since then are the unported backlog targeted by the initial catch-up |
| 2026-06-11 | `73acc60` | Initial catch-up: clone detection area done. APTED correctness fixes (ascending key roots, forest-distance subtreeCost, max(size) normalization), bounded large-tree approximation (same-shape distance, label/shape profiles), Jaccard pre-filter, Type-1 textual-match gate and Type-2 syntactic gate (ported textual/syntactic similarity), threshold recalibration (0.85/0.75/0.70/0.65), Type-3 disabled by default, overlapping-range pair rejection (isOverlappingLocation), strict-subset group member removal (group_dedup), complete_linkage rewritten as agglomerative clustering, MinLines/MinNodes 10/20, LSH auto-enable by estimated pair count, introduced `parser.OrderedChildren` |
| 2026-06-11 | `8650522` | Initial catch-up: scoring area done. Duplication metric switched to K-Core group density (groups per 1000 lines × 20, capped at 10%) with 0–10% penalty scale (`0886cfb`, `760bc87`), CBO coupling calibration softened (medium weight 0.3, saturation 0.40, `333c9ac`, `9c84a3d`), architecture score now uses compliance directly (`3c9927c`), `WeightedViolations` exposed (`e6b9920`), module-scope label `__main__` → `<module>` via `domain.ModuleFunctionName` (`2cc013c`) |
| 2026-06-12 | `fb3fe92` | Initial catch-up: misc area done (catch-up complete; baseline moved to `fb3fe92`). LSH index switched to int fragment IDs with `WithMaxCandidates` cap (default 1024) and sorted deterministic candidates, reachability `allSuccessorsReturn` memoized (removed `copyVisited`), lazy-import cycle exclusion ported as dynamic-`import()` edge skipping in circular detection (#460), coupling zone classification aligned (Zone of Pain requires D≥0.5/Ca≥2/I≤0.3/A≤0.3, main sequence D≤0.2, instability defaults 0.3/0.7), dead-code findings merged per contiguous same-reason region with EndLine sort tiebreak, bare-`;` (empty statement) dead-code noise filtered (`empty_statement` now maps to `NodeEmptyStatement`), domain error codes TIMEOUT/CANCELLED/NOT_IMPLEMENTED/INTERNAL added |
| 2026-07-11 | `3c6f8a8` (targeted) | Targeted port of the Agent Skills mechanism (`.claude-plugin/`): marketplace + skills-only plugin manifest and the 4 CLI-first skills (health-check, cli-analysis, refactoring, architecture-review) adapted to jscan's CLI; README gains an "AI Agent Integration" section (`uvx add-skills ludo-technologies/jscan`, Claude Code plugin install). Baseline SHA intentionally left at `249b121` — mapped algorithm files were not re-synced |
| 2026-07-08 | `249b121` | Normal sync (`fb3fe92`..`249b121`, 18 commits touching mapped files). Fragment hash now computed (FNV-64a hex of Type-1 normalized content) and forwarded to clone output (`012cd15`, issue #488); clone-group classification now prefers the highest-similarity pair's type over majority vote, fixing high-similarity pairs being hidden behind Type-3 transitive edges and dropped by the default type filter (`d9dcada`, issue #525); added `dedupeCoveredGroups` post-grouping pass to suppress whole groups covered by another group's larger overlapping windows (`e2a0b1b`, issue #518) and `filterCloneGroupsWithoutBackingPairs` to drop groups left with no positive-similarity backing pair (`10d116d`); duplication metric switched from K-Core group density to fragment ratio (clonedFragments/totalFragments × 100, 0–30% scale replacing 0–10%) (`4faf98f`, `b58cfbd`). Skipped: internal detector-result-type refactor (no behavior change), 4 CBO commits (Python-scoping/import-specific, `cbo.go` is reference-only), cognitive-metrics risk scoring (depends on unported CognitiveComplexity), and the new module-community-detection feature (6 commits, entirely new subsystem) — see "Pending changes" |
| 2026-07-12 | *(monorepo `core/`)* | Phase 2b clone adoption (#9 / `c57f3b4`): jscan deleted local `grouping_strategy.go`, `textual_similarity.go`, `syntactic_similarity.go`, `ast_features.go` and wired `core/clone` (grouping strategies, dedup passes, similarity gates, `PairClassifier`). Language-side keeps fragment extraction, TreeConverter, JS cost model, LSH orchestration, and `removeJSComments` as `CommentStripper`. Further algorithm changes for clone grouping/gates go to `core/clone`, not jscan copies |
| 2026-07-18 | *(monorepo `core/`)* | Phase 2b CFG/graph/LSH adoption (#11): jscan adopted `core/cfg` data structures, reachability, complexity, dead-code post-processing; `core/graph` Tarjan SCC and Martin metrics; `core/lsh` MinHash/index kernels; and shared `core/domain` scoring calculators. JavaScript classifiers, CFG construction, source enrichment, dynamic-import filtering, LSH candidate caps, and jscan-specific scoring penalties remain language-side. Intentional core semantics: exception edges contribute to complexity, unreachable registered blocks are included in complexity, and isolated dependency nodes use I=0/D=1. Reachability prunes only normal fallthrough after terminators so exception/control-transfer targets remain reachable |
| 2026-07-18 | *(monorepo `core/`)* | Phase 2b APTED adoption (#10): jscan deleted its local APTED algorithm/tree/default-cost copies and now uses `core/apted` directly with max-size normalization. Language-side keeps only the JS/TS parser converter and JavaScript cost model. |
| 2026-07-25 | `6af3ee7` (v1.27.0) | Normal sync (`249b121`..`6af3ee7`, 33 commits touching mapped paths; first run of the recreated `/sync-pyscn` command). Ported: function-coverage disclosure (#597, `92c436d`+`ca95d2e`) — `FunctionsParsed` (pre-`min_complexity`-filter parsed count) added to `ComplexitySummary`/`AnalyzeSummary`, propagated through the complexity service and analyze summary, surfaced as `functions_parsed` in JSON and as "N reported / M parsed" in text and HTML output when filtering drops functions. Already in sync via core adoption: LSH `FindCandidatesLimit` cap delegation (`a3854e5` ≙ polyscan#20) and lazy-predecessor-edge filtering (`e5b2482` ≙ jscan's `loadTimeGraph.Predecessors`). 11 pyscn core-adoption migrate commits carry nothing to port. Skipped: 7 CBO Python-scoping refinements (reference-only), config-loader plumbing (`6a12c8b`, `f1e9a4b`, `a70b5a5`), LCOM test — see "Pending changes" |
| 2026-07-28 | `81b7ec0` (v1.29.0) | Normal sync (`6af3ee7`..`81b7ec0`, 76 commits, 22 touching mapped paths). Ported: `FunctionsParsed` now excludes functions dropped by `report_unchanged` (pyscn `e9f9e6f`, a follow-up to the #597 disclosure ported last run) — the count moves inside `filterFunctions`, after the report-unchanged check and before the min/max complexity filters, so the "N reported / M parsed" line no longer blames `min_complexity` for report-unchanged drops. Verified by unit tests plus an E2E run on a 3-function fixture: `1 / 3` before, `1 / 1` after with `report_unchanged: false`, `3 / 3` with it on. Nothing this run belongs in `core/` — the new pyscn rollups are presentation-layer joins over language-side results, not shared algorithm. Skipped: 5 architecture-suggestion commits (unported `domain/suggestion.go`), the module + directory quality rollup subsystem (~20 files, [polyscan#31](https://github.com/ludo-technologies/polyscan/issues/31)), MCP registry listing — see "Pending changes" |
| 2026-07-24 | *(pyscn Phase 3 done, `152d9d0`)* | pyscn completed [polyscan#12](https://github.com/ludo-technologies/polyscan/issues/12): adopted `core/clone` (#664), `core/apted` (#669), `core/cfg` (#670), `core/graph` (#672), `core/lsh`+`core/lcom`+`core/dfa`+`core/semantic` (#673), and `core/domain` scoring/grade mapping (#674). Every **sync**-classified row in this file is retired: rows whose files were deleted on both sides (`apted.go`, `minhash.go`, `ast_features.go`, `grouping_strategy.go`/`*_grouping.go`/`group_dedup.go`, `syntactic_similarity.go`) were removed outright; rows whose files remain as thin core-delegating wrappers on both sides (`lsh_index.go`, `cfg.go`, `reachability.go`, `coupling_metrics.go`, `circular_detector.go`, `domain/analyze.go`, `domain/system_analysis.go`) were reclassified to **case-by-case**. Grade computation is identical between pyscn and jscan by construction — both call `core/domain.GradeFromScore`/`IsHealthyScore` directly and each has its own unit test asserting parity with core |

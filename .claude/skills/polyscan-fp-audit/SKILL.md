---
name: polyscan-fp-audit
description: Run polyscan against one JavaScript/TypeScript, Go, Rust or C++ repository and triage findings for false positives using sub-agents. Auto-files clear polyscan bugs as GitHub issues (with `auto-filed` label, dedup, rate limit) and accumulates cross-repo tuning patterns into draft files. Outputs a markdown report under `.polyscan/audit/results/`. Accepts a repo URL/path as argument, or auto-picks the next pending entry from `.polyscan/audit/queue.md`.
---

# polyscan FP Audit Skill

Run polyscan (built from this checkout) against a target repository, then triage findings to surface likely false positives.

All paths below are relative to the monorepo root (the directory holding `core/`, `polyscan/`, `jscan/`). The CLI lives in `polyscan/`; the audit workspace is `.polyscan/audit/` at the root (gitignored).

## Input

- `$ARGUMENTS` may be:
  - A GitHub URL: `https://github.com/owner/repo`
  - A local path to an already-cloned repo
  - Empty: pick the next `- [ ]` entry from `.polyscan/audit/queue.md`. If the queue is empty or missing, invoke the **`polyscan-repo-discovery`** skill first to populate it.

## Steps

### 1. Build polyscan (skip if recent)

Skip the build if `polyscan/polyscan` exists, was modified within the last 24h **and** is newer than the current `HEAD`:

```bash
if [ -x polyscan/polyscan ] \
   && [ "$(find polyscan/polyscan -mtime -1 -print 2>/dev/null)" ] \
   && [ polyscan/polyscan -nt .git/HEAD ]; then
  echo "skip build: polyscan binary is fresh"
else
  make -C polyscan build
fi
```

Verify `polyscan/polyscan --version` runs. If the build fails, stop and report.

### 2. Resolve the target

Slug = `<owner>-<repo>`, lowercase, non-alnum → `-`.

- **URL** (preferred — tarball download, no `.git/`):
  ```bash
  mkdir -p .polyscan/audit/repos/<slug>
  curl -fsSL "https://codeload.github.com/<owner>/<repo>/tar.gz/HEAD" \
    | tar -xz -C .polyscan/audit/repos/<slug> --strip-components=1

  # Record the audited commit for the report
  SHA=$(curl -fsSL -H "Accept: application/vnd.github.sha" \
    "https://api.github.com/repos/<owner>/<repo>/commits/HEAD")
  ```
  If the directory already exists from a prior run, `rm -rf` it first (tarballs can't be incrementally updated).
  For private repos, swap to `gh api repos/<owner>/<repo>/tarball/HEAD > /tmp/<slug>.tar.gz` and extract.
- **Local path**: use as-is, slug = basename. Don't download or delete.
- **From queue**: read the first `- [ ]` line, extract URL, then proceed as the URL case.

Record the repo's **primary language** (from the queue line, or by counting source files by extension) — the report and issue titles carry it.

### 3. Run polyscan

Create `TS=$(date +%Y%m%d_%H%M%S)` and `OUT=.polyscan/audit/results/<slug>/$TS/raw`.

```bash
mkdir -p "$OUT"
# `--format json` streams the full report to stdout; the health summary goes to stderr.
polyscan/polyscan analyze --format json "<repo-path>" > "$OUT/analyze.json" 2> "$OUT/analyze.stderr" || true
```

Don't abort on non-zero exit. Capture stderr; if `analyze.json` is empty or not valid JSON (`jq -e . "$OUT/analyze.json"`), stop and report. `analyze.stderr` also carries per-analysis warnings such as `JavaScript clone analysis error: ...` — those are findings in their own right (see step 5, `clear-bug`).

The single `analyze.json` holds every analysis under these top-level keys:

| Key | Languages | Per-finding path |
|---|---|---|
| `complexity` | all | `.complexity.functions[]` → `name`, `file_path`, `language`, `start_line`, `end_line`, `metrics.complexity`, `risk_level` |
| `clone` | all | `.clone.clone_pairs[]` → `clone1`/`clone2` (`language`, `location.{file_path,start_line,end_line}`, `content`, `line_count`), `similarity`, `type`; `.clone.clone_groups[]` |
| `dead_code` | JS/TS only | `.dead_code.files[].functions[].findings[]` → `location`, `reason`, `severity`, `description` |
| `cbo` | Go, Rust, JS/TS | `.cbo.classes[]` → `Name`, `FilePath`, `language` (Go, Rust; absent for JS/TS), `StartLine`, `Metrics.CouplingCount`, `Metrics.DependentClasses`, `RiskLevel` |
| `lcom` | Go, Rust | `.lcom.classes[]` → `name`, `file_path`, `language`, `start_line`, `metrics.lcom4`, `metrics.method_groups`, `risk_level` |
| `deps` | Go, JS/TS | `.deps.analysis`, `.deps.graph` (Go nodes are packages keyed by import path) |
| `summary` | — | `health_score`, `grade`, `total_loc`, `total_files`, `skipped_files`, per-dimension scores |

To narrow scope, add `--select complexity,deadcode,clone,cbo,lcom,deps`.

**Known structural zeros — never report these as bugs.** For Go/Rust/C++ the generic engine fills only `metrics.complexity` and `metrics.nesting_depth`; `nodes`, `edges`, `if_statements`, `loop_statements`, `exception_handlers` and `switch_cases` are always `0`. Clone fragments always have `hash: ""`, `complexity: 0` and `start_col`/`end_col` `0`. `cbo.classes[]` for JS lists one pseudo-class per file (module-level coupling), so a `Name` equal to the file basename is expected. For Go and Rust a `cbo` entry is one type, and `DependentClasses` only ever names types declared in the analyzed tree: standard library, third-party and other-module types are absent by design, a type coupled to nothing is not listed, and a Go tree without `go.mod` lists same-package dependencies only (a warning says so). For Go `deps.analysis.CircularDependencies` is always empty (the compiler forbids import cycles), Go edges carry no `location`, and Go files outside any `go.mod`, `_test.go` files, and `vendor`/`testdata` directories are absent from the graph by design. A Go type in `lcom.classes[]` is measured over the methods of its whole package and placed in the first file that declares one, so `file_path` need not be the file that declares the type; `excluded_methods` counts methods without a receiver parameter, which is expected for Rust `new`-style associated functions.

### 4. Cluster findings

Read the JSON output. Group findings by `(analysis, language, rule_or_pattern)`. Examples:

- `complexity / go / cyclomatic >= 20`
- `clone / rust / type-2-similarity-high`
- `clone / cpp / cross-file-header-impl`
- `deadcode / ts / unreachable_after_return`
- `cbo / ts / coupling >= 10`
- `cbo / go / coupling >= 8`
- `lcom / go / lcom4 >= 6`

For each cluster, record the total count and pick **3–5 representative samples** (prefer variety: different files, different sizes, mix of risk levels). Also collect:

- `summary.skipped_files` and the `analyze.stderr` warnings — a skipped valid source file or a crashed analysis is its own cluster (`engine / <lang> / skipped-file`).
- Findings whose `file_path` lies under `vendor/`, `third_party/`, `node_modules/`, `target/`, `build/`, `dist/`, `_deps/`, or matches generated-file markers. The generic (Go/Rust/C++) collector has **no directory exclusions**, so these are a known `tuning` pattern (`generic-scans-vendored-dirs`); count them, sample 1–2, and don't let them crowd out the clusters that matter.

### 5. Triage in parallel

Spawn one **`Explore` Agent per cluster**, all in a single message so they run concurrently. Brief each agent:

> "Triage polyscan findings of type **`<analysis> / <language> / <pattern>`** in repo `<repo-path>`.
>
> Findings to evaluate (JSON):
> ```json
> <cluster sample, including file path, line range, metric values, and clone content if present>
> ```
>
> For each finding:
> 1. Read the cited file/lines plus enough surrounding context (callers, enclosing type, tests, build files) to judge intent
> 2. Decide a verdict:
>    - `TP` — polyscan is correct, this is a real issue
>    - `FP` — polyscan is wrong. Language-specific FP patterns to check:
>      - **Go**: `// Code generated ... DO NOT EDIT.` files; table-driven tests; `if err != nil { return }` chains inflating complexity; `switch` over enum-like constants; `_test.go` clones that mirror a fixture; cgo shims
>      - **Rust**: `macro_rules!`/derive-expanded code; trait impls that must be repeated per type (`impl From<X> for Y`); exhaustive `match` on a large enum; `#[cfg(...)]` variants of the same function; `tests/` fixtures
>      - **C++**: the same declaration in a header and its definition in a `.cpp`; template specializations; `switch` dispatch on opcodes; `.h` files in `include/` that also exist in `src/`; generated protobuf/flatbuffers code
>      - **JS/TS**: `.d.ts` declarations; bundled/minified output that escaped exclusion; `switch` cases with `return` before `break` flagged as dead; overloads; generated GraphQL/OpenAPI clients; test snapshots
>      - **Any language**: two files that are the same intentional boilerplate (CLI entrypoints, example programs); a fixture directory of deliberately duplicated samples
>    - `unsure` — needs human judgment
> 3. Write 1–2 sentences explaining *why*, citing specific code constructs
> 4. Also classify `bug_class` for downstream auto-filing:
>    - `clear-bug` — structural anomaly in polyscan's output that any maintainer would call a bug. Examples: line range is `0-0` or `end_line < start_line`; `metrics.complexity` is negative, `0` for a non-empty function, or off by orders of magnitude; reported function/class name doesn't exist at that location; `language` field doesn't match the file extension; polyscan crashed on, or silently skipped, a valid source file (see `analyze.stderr` and `summary.skipped_files`); a clone pair where `clone1` and `clone2` are the same location; the same finding duplicated. **Narrow category — only pick this when you can point at the exact JSON field that is wrong.** The structural zeros listed in the brief are NOT bugs.
>    - `tuning` — polyscan works as designed but the heuristic misfires on a recognizable, generalizable pattern (e.g. generated code not excluded, header/impl duplicates counted as clones, error-handling chains counted as complexity). Most FPs land here.
>    - `none` — TP, or an FP too repo-specific to generalize
> 5. Assign a `pattern_slug` (kebab-case, generic — `go-generated-code-not-excluded`, `cpp-header-impl-clone`, not `foo-repo-bar-file`), and for `clear-bug` a priority:
>    - `P0` — crash, or output that is wrong for every file of a language
>    - `P1` — wrong metric or location on a common construct
>    - `P2` — wrong on an uncommon construct
>    - `P3` — cosmetic (naming, ordering)
>    and `good_first_issue: true` when the fix is plausibly local to one function.
>
> Return as JSON array:
> ```json
> [
>   {
>     "id": "<file:line>",
>     "verdict": "TP|FP|unsure",
>     "reason": "...",
>     "evidence": "<key code snippet or construct name>",
>     "bug_class": "clear-bug|tuning|none",
>     "pattern_slug": "<kebab-case>",
>     "priority": "P0|P1|P2|P3",
>     "good_first_issue": true
>   }
> ]
> ```
>
> Do not modify any files. Read-only triage."

### 6. Auto-file `clear-bug` findings (with dedup + rate limit)

For each finding with `bug_class == "clear-bug"`:

1. **Dedup** against existing auto-filed issues:
   ```bash
   gh issue list -R ludo-technologies/polyscan -s all -L 50 \
     --label auto-filed --search "<pattern_slug>" --json number,title
   ```
   If any result mentions the same `pattern_slug`, **skip filing** — note "deduped against #<n>" in the report. Only `auto-filed`-labeled issues are considered for dedup; manually-filed issues are intentionally ignored.

2. **Verify claims against raw JSON.** Before drafting the issue body, for every specific metric value, field name, or count you're about to cite in "Actual Output", extract it directly from `raw/analyze.json` for that exact finding (via `jq`) and use that literal value — do not restate the triage sub-agent's prose `evidence` field if it paraphrases a number. If the value you were about to cite doesn't match what's actually in `raw/analyze.json`, the finding is not a real `clear-bug`: downgrade it to `unsure`/`none` and do not file.

3. **Rate limit** (skip if either is exceeded):
   - **Per-audit cap**: 2 auto-filed issues per audit run. If exceeded, file the rest as drafts only.
   - **Per-day cap**: 5 auto-filed issues per UTC day. Read `.polyscan/audit/issues.jsonl`, count entries where `ts` starts with today's date.

4. **File the issue**:
   ```bash
   gh issue create -R ludo-technologies/polyscan \
     --title "[BUG][auto] <lang>/<pattern_slug>: <one-line description>" \
     --label bug --label auto-filed --label "<priority>" \
     $( [ "<good_first_issue>" = "true" ] && printf -- '--label "good first issue"' ) \
     --body-file <draft-path>
   ```
   `<lang>` is `go`, `rust`, `cpp`, `js` or `ts`. `<priority>` is the `P0`/`P1`/`P2`/`P3` label from step 5. All labels are assumed to already exist in the repo — if `gh issue create` fails with "label not found", stop and surface the error rather than auto-creating.

   Body should include: bug description, repro steps (a minimal synthetic source file in the affected language if possible), expected behavior, actual output (cite the JSON snippet), polyscan version (`polyscan/polyscan --version`), priority rationale (one line, citing the rubric in step 5), and "Found via the FP-audit skill in repo `<owner/repo>@<sha>`". Keep it short; no test plan.

5. **Log it** by appending to `.polyscan/audit/issues.jsonl`:
   ```json
   {"ts": "2026-09-05T19:30:00Z", "slug": "<owner-repo>", "lang": "go", "pattern_slug": "...", "issue_url": "https://github.com/.../issues/N"}
   ```

### 6b. Append `tuning` findings to draft files

For each unique `pattern_slug` with `bug_class == "tuning"`:

1. Path: `.polyscan/audit/issues/draft-<pattern_slug>.md`
2. If the draft file doesn't exist, create it with this skeleton:
   ```markdown
   # [tuning draft] <pattern_slug>

   <one-paragraph explanation of the FP pattern, written generically; name the language(s) it applies to>

   ## Repos hitting this

   <!-- audit log appended below; one entry per repo -->
   ```
3. **Append** a new section per repo (don't duplicate if the repo+pattern already appears):
   ```markdown
   ### <owner/repo>@<sha-short> (<lang>) — audited 2026-09-05
   - **Findings**: `<file>:<line>` (<verdict>) — <reason>
   - **Evidence**: <snippet>
   ```
4. These drafts are **never auto-filed**. The user reviews them periodically (e.g., once 3+ repos hit the same pattern, they decide whether to file).

### 7. Aggregate the report

Write `.polyscan/audit/results/<slug>/$TS/report.md`:

```markdown
# polyscan FP Audit — <owner/repo>

- **Repo**: <url-or-path> (commit `<sha>`)
- **Language**: <primary language> (<file counts per language from the report>)
- **polyscan**: `<version>` (commit `<polyscan-sha>`)
- **Date**: 2026-09-05
- **LOC analyzed**: <summary.total_loc> across <summary.total_files> files (<summary.skipped_files> skipped)
- **Health**: <summary.health_score> (<summary.grade>)

## Summary

| Cluster | Total | Sampled | TP | FP | Unsure | FP rate (sample) |
|---|---|---|---|---|---|---|
| clone / go / type-2-similarity-high | 42 | 5 | 4 | 1 | 0 | 20% |
| ...

## Issues filed

- `<pattern_slug>` [P1] → #N — <one-line>
- `<pattern_slug>` [P2] → deduped against #M (skipped)
- `<pattern_slug>` [P0] → rate-limited (drafted to `.polyscan/audit/issues/draft-<slug>.md`)

## Tuning drafts updated

- `.polyscan/audit/issues/draft-<pattern_slug>.md` (now N repos)

## Notable false positives

### `<analysis> / <language> / <pattern>` — `<file>:<line>`
**Verdict**: FP
**Why**: <reason from sub-agent>
**Evidence**:
```<lang>
<minimal snippet>
```

(repeat for each FP / interesting unsure)

## Notable true positives (sanity check)

(1–2 confirmed TPs to verify the tool is working)

## Raw

- `raw/analyze.json`
- `raw/analyze.stderr`
```

### 8. Update the queue

If the target came from `queue.md`:
- Flip `- [ ]` to `- [x]`, append `— audited 2026-09-05` with a relative link to the report
- Move the line from `## Pending` to `## Audited`

### 9. Clean up the clone

If the target was downloaded (not a local path), delete the source tree to save disk:

```bash
rm -rf .polyscan/audit/repos/<slug>
```

The `raw/analyze.json` and `report.md` already contain everything needed for retrospective analysis — the source can be re-fetched if needed.

### 10. Report to user

One paragraph: language, total findings, suspected FP rate per cluster, **# of issues filed (with URLs) and # of tuning drafts touched**, link to the report file. Don't paste the full report inline.

## Notes

- All work stays under `.polyscan/audit/` (gitignored) **except** auto-filed issues, which are public on GitHub. Treat the `auto-filed` label as the kill-switch: a single `gh issue list -R ludo-technologies/polyscan -l auto-filed --json number -q '.[].number' | xargs -n1 gh issue close -R ludo-technologies/polyscan` can roll everything back if the heuristics drift.
- Sub-agents are read-only on the local filesystem — they investigate and judge, they don't modify code.
- The `clear-bug` category is intentionally narrow. When in doubt, prefer `tuning` (which only drafts, never auto-files) — false issues are far more costly than missed ones.
- If a cluster is huge (>200 findings), still only sample 5 — the rate from a sample is what matters.
- Python repos are out of scope: polyscan has no Python analyzer; use `pyscn-fp-audit` in the pyscn checkout for those.
- Avoid running on this monorepo (`polyscan` itself) — that's not the audit target. `polyscan/testdata/` is a fixture tree full of intentional clones and dead code.

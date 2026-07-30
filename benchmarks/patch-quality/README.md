# Patch quality benchmark

How much does a code change cost you in maintainability, and does it matter who
wrote it?

SWE-bench measures whether a patch is *correct*. This measures whether it is
*clean*. It scores the same 500 SWE-bench Verified issues with a polyscan
analyzer, comparing each model's patch against the commit a human maintainer
actually wrote to fix that same issue.

Nothing here calls a model API. Every patch already exists: the human fixes ship
with SWE-bench, and the model patches are published leaderboard submissions.
Running the whole thing costs nothing but disk and time.

## What it measures

For every (issue, patch) pair:

```
git checkout base_commit
analyze the files the patch touches      -> before
git apply the patch
analyze the same files                   -> after
delta = after - before
```

Only **complexity** and **dead code** are used. Both are computed per function
from the control-flow graph of a single file, so they stay exact when only part
of a repository is analyzed. Coupling, cohesion, clone detection and dependency
analysis are excluded on purpose: they need the whole codebase, and on a subset
they undercount whatever the missing files would have contributed. Including
them would make each score depend on which files a patch happened to touch.

The headline metric is the **regression rate**: the share of patches that made
any measured metric worse. The human patch on the same issue is the baseline.

"Worse" is judged **per function**, comparing each function against itself. The
maximum complexity over a set of files is nearly useless for this: django's
`sql/query.py` holds a 21-branch method, so nothing a patch does to a 4-branch
method next to it moves the file's maximum at all. Measured that way the
benchmark saw about a quarter of the complexity increases it should have.

## Quick start

```bash
# No dependencies. pyscn is the only thing you need on PATH.
export PYSCN_BIN=/path/to/pyscn   # or just have pyscn on PATH

python3 fetch_data.py                      # ~100 MB
python3 clone_repos.py --repo django/django   # ~300 MB, full history
python3 harness.py --repo django/django --out results/django.jsonl
python3 report.py results/django.jsonl --resolved-only
```

`clone_repos.py` with no `--repo` clones all twelve repositories, which is
several GB. Start with django; it is 231 of the 500 instances.

All twelve result sets are committed, so the report can be reproduced without
running anything:

```bash
python3 report.py results/*.jsonl.gz --resolved-only
```

`report.py` pools several result sets, and refuses to mix sets produced by
different analyzer builds.

Result sets are committed gzipped (a few MB of JSONL compresses to about
100 KB); `report.py` reads either form. Each one starts with a metadata record
naming the analyzer build that produced it, so a committed number can be tied to
a specific pyscn version.

The correctness gate travels inside the result set: the harness stamps every
record with that model's resolved status. `data/` is gitignored, so reading the
gate from there instead meant `--resolved-only` silently gated every model down
to zero instances and printed an empty table from a fresh clone.

## Read the correctness gate column first

`--resolved-only` restricts each model to the instances it actually solved, and
it changes the answer. Over django, 8 of 14 models look worse than the human
patch at p<0.05 without the gate; 3 do with it.

Most of that gap is survivorship. A patch that fails its tests is often failing
because it broke or gutted something, and broken code has less measurable
complexity than working code, so ungated numbers credit failure as
simplification. The outcome columns catch the blatant cases — `broke_syntax`,
`not_a_diff`, `destructive` — but nothing short of running the tests catches a
patch that is syntactically fine and semantically wrong.

What the gate does *not* buy is a clean read on capability. Among the seven
models in the controlled scaffold group, resolve rate and regression rate
correlate at r = +0.87 once gated. Do not read that as "stronger models write
messier code": a model that solves more instances is being measured on harder
ones, and harder issues take larger patches. Instance difficulty is not
controlled for here, which is exactly why the report pairs each model against
the human on *the same* instances instead of comparing rates across models.

## Four traps

Each of these silently turns a bad patch into an apparent improvement. They are
handled in the harness, and they are the reason this is not a fifty-line script.

### 1. Created and deleted files are asymmetric

Agents leave scratch files behind: `check_url_parts.py`, `final_verification.py`,
`reproduce_bug.py`. Those files do not exist at `base_commit`, so they have no
"before" measurement, but they do get measured "after". Every delta inflates.

Deletion is the same problem pointing the other way, and it is easier to miss. A
file the patch removes *does* have a "before" measurement and has no "after", so
leaving it in the measured set books its entire complexity as an improvement.
Renames are a delete plus a create.

The harness measures only paths that exist on both sides, and counts created and
deleted files separately. The created-file count turns out to be interesting on
its own: it varies from 0 to over 5 scratch files per solved issue, tracks the
agent scaffold rather than the model, and two submissions of the same model under
different scaffolds land at opposite ends of the range.

### 2. A file that does not parse scores perfectly

No parse means no functions, and no functions means no complexity, no dead code,
nothing to flag. A patch that introduces a syntax error looks like the best
refactor in the run.

This is real, not hypothetical. One submission corrupted
`django/db/models/sql/query.py` with `return 0 objs`; the file went from 105
functions and 425 total complexity to zero and zero, and ranked as the single
largest quality improvement in the run. Across django, 71 of 3407 records
are patches that broke a file they touched; `o4-mini` and `qwen2.5-coder-32b`
account for 47 of them.

The harness records the patch as `broke_syntax` instead of as a delta. It gets
there two ways, on purpose. pyscn reports unparseable files in
`complexity.Errors`, but matching that is matching on message text, which is not
an interface — if the wording ever changes, the defence fails silently and in the
worst direction. So the harness also parses every measured file itself with
`ast`. Neither source is trusted alone, and both are applied differentially: a
file that already failed to parse *before* the patch is not blamed on the patch.

Note that pyscn's own health score does not do this — an unparseable file scores
Grade A and `pyscn check` exits 0
([pyscn#690](https://github.com/ludo-technologies/pyscn/issues/690)).

### 3. Mass deletion passes the syntax check

The same submission deleted 2,124 of 2,671 lines from that file in another
instance, and the result was still valid Python. Parsing cannot catch this.

Every such case in the django run was `unresolved` — the tests caught what the
parser could not. That is the strongest argument for the correctness gate: it is
the only defence against a patch that is syntactically fine and semantically
destructive. The harness flags them with a `destructive` field, and the report
excludes flagged pairs from the paired comparison rather than averaging them in:
a gutted file is not evidence of clean code, and counting it as a
*non*-regression would drag the model's rate down.

### 4. The analyzer's own defaults decide what gets measured

`pyscn --min-complexity` defaults to 5, and it filters the reported function
list rather than just a display threshold. Left at the default, django's
`sql/query.py` reports 29 functions summing to 277 complexity when the file
actually holds 105 summing to 425 — 72% of functions invisible.

Worse, the threshold makes the measurement non-monotonic in both directions. A
function simplified from 6 to 4 drops out of the report and reads as a *deleted
function*, which also feeds the `destructive` heuristic. A function that grows
from 3 to 8 appears from nowhere with no "before" value to compare against, so
the regression goes uncounted. The harness pins `--min-complexity 1`.

The general problem outlives that one flag: absence of metrics for a file is
ambiguous between "has no functions" and "the analyzer declined to look". A
silently skipped file contributes zeros to both sides, so the delta stays honest,
but the file and line counts quietly include code nobody measured. The analyzer
adapter cross-checks its own parse against the report and returns the difference
as `skipped_files`; if everything a patch touched was skipped, the outcome is
`all_files_skipped` rather than an analyzer error.

## Where the data comes from

| what | where | note |
| --- | --- | --- |
| instances, `base_commit`, human patches | HuggingFace `princeton-nlp/SWE-bench_Verified` | 500 rows |
| model patches | `s3://swe-bench-submissions` over HTTPS | no credentials needed with a prefix |
| resolved status | `swe-bench/experiments` on GitHub | two formats, see below |

Model patches are **not** in the `swe-bench/experiments` GitHub repository,
despite its README documenting `all_preds.jsonl` in the layout. At HEAD there are
zero, and searching history only four submissions ever committed one. The S3
bucket the submission metadata points at has them for around 100 submissions and
is publicly readable, but only with a prefix — a bare bucket listing returns
AccessDenied.

Resolved status comes in two shapes. The `verified` split uses
`results/results.json`, already a list of resolved ids. The `bash-only` split
uses `per_instance_details.json`, a per-instance dict that also carries dollar
cost and API call counts. `fetch_data.py` normalises both.

## Comparing models fairly

Submissions differ by agent scaffold, retry policy and prompt, not only by model.
Two submissions in different scaffold groups are not a model comparison.

`config/models.json` tags each submission with a `scaffold`, and the report
groups by it. The `mini-swe-agent-v1.0.0` group is the useful one: seven models
run through an identical harness, so the model is the only variable.

Directory names lie, too. `20250516_cortexa_o3` is not o3 — it is NVIDIA's
Nemotron-CORTEXA, an ensemble over a dozen models with two or more attempts per
instance.

## Interpreting the output

Effect sizes are small. Over django, a typical model raises some measured metric
on 29–58 of its 110–168 gated instances, against 19–43 for the human patch on
those same instances. The two sides move together, because most of what drives a
regression is the issue rather than who fixed it — which is the reason for
pairing.

The report pairs each model against the human on the same instances and runs a
two-sided exact McNemar test on the discordant pairs. On django alone, 3 of 14
models reach p<0.05 and 2 survive Bonferroni correction across 14 comparisons:
`gpt-5` (p=0.001) and `devstral-small` (p=0.000), both OpenHands. They sit at
opposite ends of that group's resolve rate, so this is not a capability ordering.

Every other model, including the whole controlled scaffold group, is
indistinguishable from the human baseline at this sample size. Read the result as
"two submissions are measurably messier than the maintainer's own patch, and
eleven are not", and note that one repository out of twelve is a thin basis for
either half of that.

## Adding an analyzer

`analyzer.py` exposes one interface: `run(repo_root, rel_files)` returning an
`Analysis` of per-file metrics, files that failed to parse, and files the
analyzer skipped. Adding jscan for
JavaScript and TypeScript means implementing that against SWE-bench Multilingual;
nothing in the harness is Python-specific.

## Layout

```
fetch_data.py     instances, patches, resolved status
clone_repos.py    the twelve repositories, full history
analyzer.py       analyzer adapters
harness.py        before/after measurement
report.py         pairing, McNemar, scaffold grouping
config/           which submissions to score
results/          committed result sets, gzipped JSONL
tests/            unit tests for the parsing and statistics (stdlib unittest)
```

```bash
python3 -m unittest discover -s tests
```

`data/` and `repos/` are gitignored; they are ~400 MB for django alone and fully
reproducible from the two fetch scripts.

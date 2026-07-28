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

## Quick start

```bash
pip install nothing            # stdlib only
export PYSCN_BIN=/path/to/pyscn   # or just have pyscn on PATH

python3 fetch_data.py                      # ~100 MB
python3 clone_repos.py --repo django/django   # ~300 MB, full history
python3 harness.py --repo django/django --out results/django.jsonl
python3 report.py results/django.jsonl --resolved-only
```

`clone_repos.py` with no `--repo` clones all twelve repositories, which is
several GB. Start with django; it is 231 of the 500 instances.

A django result set is committed, so the report can be reproduced without
running anything:

```bash
python3 report.py results/django.jsonl.gz --resolved-only
```

Result sets are committed gzipped (2.5 MB of JSONL compresses to about 100 KB);
`report.py` reads either form.

## Read the correctness gate column first

`--resolved-only` restricts each model to the instances it actually solved. This
is not a refinement, it changes the conclusion.

Without the gate, regression rate correlates strongly with model capability
(r = -0.91 across a seven-model controlled set). With it, that correlation
disappears (r = +0.03). The apparent relationship was never strong models
writing cleaner code; it was weak models producing patches that break or delete
things, which scores as a large quality *improvement* for reasons the next
section explains.

## Three traps

Each of these silently turns a bad patch into an apparent improvement. They are
handled in the harness, and they are the reason this is not a fifty-line script.

### 1. New files are asymmetric

Agents leave scratch files behind: `check_url_parts.py`, `final_verification.py`,
`reproduce_bug.py`. Those files do not exist at `base_commit`, so they have no
"before" measurement, but they do get measured "after". Every delta inflates.

The harness measures only files that existed at `base_commit`, and counts new
files separately. That count turns out to be interesting on its own: it varies
from 0 to over 5 scratch files per solved issue, tracks the agent scaffold rather
than the model, and two submissions of the same model under different scaffolds
land at opposite ends of the range.

### 2. A file that does not parse scores perfectly

No parse means no functions, and no functions means no complexity, no dead code,
nothing to flag. A patch that introduces a syntax error looks like the best
refactor in the run.

This is real, not hypothetical. One submission corrupted
`django/db/models/sql/query.py` with `return 0 objs`; it went from 128 functions
and 391 total complexity to zero and zero, and ranked as the single largest
quality improvement in the run.

The harness reads `complexity.Errors` from the analyzer report and records the
patch as `broke_syntax` instead of as a delta. Note that pyscn's own health score
does not do this — an unparseable file scores Grade A and `pyscn check` exits 0
([pyscn#690](https://github.com/ludo-technologies/pyscn/issues/690)).

### 3. Mass deletion passes the syntax check

The same submission deleted 2,124 of 2,671 lines from that file in another
instance, and the result was still valid Python. Parsing cannot catch this.

Every such case in the django run was `unresolved` — the tests caught what the
parser could not. That is the strongest argument for the correctness gate: it is
the only defence against a patch that is syntactically fine and semantically
destructive. The harness also flags them with a `destructive` field so they can
be counted rather than silently dropped.

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

Effect sizes are small and event counts are low. Over django, a typical model
regresses 7–16 of ~130 solved instances against 2–7 for the human. Ratios built
on that many events move a lot with one or two instances.

The report pairs each model against the human on the same instances and runs a
two-sided exact McNemar test on the discordant pairs. On django alone, 3 of 13
models reach p<0.05 and none survive Bonferroni correction. The honest reading is
that models trend slightly worse than human patches but the difference is not
resolvable at this sample size — which is the argument for running more than one
repository.

## Adding an analyzer

`analyzer.py` exposes one interface: `run(repo_root, rel_files)` returning
per-file metrics and a list of files that failed to parse. Adding jscan for
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
```

`data/` and `repos/` are gitignored; they are ~400 MB for django alone and fully
reproducible from the two fetch scripts.

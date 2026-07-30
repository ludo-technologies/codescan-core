#!/usr/bin/env python3
"""Measure the code-quality cost of a patch.

For every (instance, patch source) pair:

    git checkout base_commit
    analyze the files the patch touches          -> before
    git apply the patch
    analyze the same files                       -> after
    record the delta

"Patch source" is either `gold` (the human commit that actually fixed the issue,
shipped with SWE-bench) or a model submission. Every source is measured the same
way, so the human patch acts as the baseline.

Run `fetch_data.py` and `clone_repos.py` first. See README.md for the
measurement traps this handles; they are not optional refinements, each one
turns a broken patch into an apparent quality improvement if ignored.
"""

import argparse
import glob
import json
import os
import re
import subprocess
import sys

from analyzer import AnalyzerError, get_analyzer

# A patch that drops this fraction of the functions in the files it touches is
# destructive rather than an improvement. These are reported separately and
# excluded from the paired comparison; in practice the correctness gate also
# excludes them, which is the main argument for applying it. See README, trap 3.
DESTRUCTIVE_FUNC_RATIO = 0.7
DESTRUCTIVE_MIN_FUNCS = 10

SCHEMA_VERSION = 2


# ---------------------------------------------------------------------- git

def git(repo, *args, check=True):
    proc = subprocess.run(["git", "-C", repo, *args],
                          capture_output=True, text=True)
    if check and proc.returncode != 0:
        raise RuntimeError(f"git {' '.join(args)}: {proc.stderr.strip()[:300]}")
    return proc


def reset_to(repo, commit):
    git(repo, "checkout", "-f", "-q", commit)
    git(repo, "clean", "-fdq", check=False)


# ------------------------------------------------------------------- diffs

def _header_path(line, prefix):
    """Path out of a `--- a/x` or `+++ b/x` header line, or None for /dev/null."""
    raw = line[4:].split("\t")[0].strip()
    if raw == "/dev/null":
        return None
    if raw.startswith(prefix):
        raw = raw[len(prefix):]
    return raw or None


_UNSET = object()
_HUNK = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@")


def parse_diff(patch_text):
    """Per-file entries of a unified diff.

    Each entry is {old, new, changed}: the pre-image path (None if the file is
    created), the post-image path (None if deleted), and the number of added
    plus removed lines.

    Hunk bodies are consumed by the line counts in their own `@@` header rather
    than by looking for the next thing that resembles a header. Source code
    contains lines beginning with `+++ ` and `--- `, and a patch that adds one
    would otherwise be read as starting a new file.
    """
    entries = []
    pending_old = _UNSET
    left = right = 0

    for line in patch_text.splitlines():
        if left or right:
            if line.startswith("\\"):        # "\ No newline at end of file"
                continue
            kind = line[:1]
            if kind == "-":
                left = max(0, left - 1)
                entries[-1]["changed"] += 1
            elif kind == "+":
                right = max(0, right - 1)
                entries[-1]["changed"] += 1
            else:                            # context, or a blank context line
                left, right = max(0, left - 1), max(0, right - 1)
            continue

        m = _HUNK.match(line)
        if m and entries:
            left, right = int(m.group(2) or 1), int(m.group(4) or 1)
        elif line.startswith("--- "):
            pending_old = _header_path(line, "a/")
        elif line.startswith("+++ ") and pending_old is not _UNSET:
            entries.append({"old": pending_old, "new": _header_path(line, "b/"),
                            "changed": 0})
            pending_old = _UNSET
    return entries


def classify_paths(entries, extensions):
    """Split a parsed diff into modified / created / deleted paths of interest.

    Deleted paths matter as much as created ones, and for the mirror reason. A
    file that exists at base_commit and is gone afterwards gets a "before"
    measurement and no "after" one, so leaving it in the measured set turns a
    deletion into a pure quality improvement. Renames are a delete plus a
    create. See README, trap 1.
    """
    modified, created, deleted = set(), set(), set()

    def keep(path):
        return path is not None and path.endswith(extensions)

    for e in entries:
        old, new = e["old"], e["new"]
        if old == new:
            if keep(old):
                modified.add(old)
            continue
        if keep(old):
            deleted.add(old)
        if keep(new):
            created.add(new)
    return sorted(modified), sorted(created), sorted(deleted)


def changed_lines(entries, measured):
    """Added plus removed lines within the files actually measured.

    Patch size is the natural denominator for "how much complexity did this add
    per line changed", but the raw diff also contains scratch files and test
    files that are never measured. Counting those inflates the denominator and
    makes a bloated patch look efficient.
    """
    return sum(e["changed"] for e in entries
               if e["new"] in measured or (e["new"] is None
                                           and e["old"] in measured))


def apply_patch(repo, patch_text, base_commit):
    # SWE-bench prediction patches are frequently missing their final newline,
    # which git rejects as "corrupt patch at line N". Without this, most
    # submissions fail to apply for a reason that has nothing to do with them.
    if not patch_text.endswith("\n"):
        patch_text += "\n"
    proc = None
    for extra in (["-p1"], ["-p1", "--3way"], ["-p1", "--ignore-whitespace"]):
        proc = subprocess.run(["git", "-C", repo, "apply", *extra, "-"],
                              input=patch_text, capture_output=True, text=True)
        if proc.returncode == 0:
            return True, ""
        # A failed `--3way` leaves conflict markers in the worktree. Measuring
        # those, or letting the next strategy apply on top of them, would be
        # worse than the failure itself.
        reset_to(repo, base_commit)
    return False, proc.stderr.strip()[:200]


# ------------------------------------------------------------------ metrics

def aggregate(per_file):
    agg = {"max_cc": 0, "sum_cc": 0, "n_func": 0, "n_high_risk": 0,
           "dead_critical": 0, "dead_warning": 0, "dead_info": 0}
    for rec in per_file.values():
        agg["max_cc"] = max(agg["max_cc"], rec["max_cc"])
        for key in ("sum_cc", "n_func", "n_high_risk",
                    "dead_critical", "dead_warning", "dead_info"):
            agg[key] += rec[key]
    return agg


def function_deltas(before, after):
    """Per-function movement between two measurements.

    The aggregate max_cc is almost useless as a regression signal: it is the
    maximum over every function in every measured file, so a patch only moves it
    by touching the single most complex function in the whole set. Comparing
    functions to themselves is what actually detects "this got harder to read".
    """
    worst, worst_name = 0, None
    new_max, removed = 0, 0
    for path, arec in after.items():
        brec = before.get(path) or {"functions": {}}
        for fname, ainfo in arec["functions"].items():
            binfo = brec["functions"].get(fname)
            if binfo is None:
                new_max = max(new_max, ainfo["cc"])
                continue
            delta = ainfo["cc"] - binfo["cc"]
            if delta > worst:
                worst, worst_name = delta, f"{path}::{fname}"
    for path, brec in before.items():
        arec = after.get(path) or {"functions": {}}
        removed += sum(1 for fname in brec["functions"]
                       if fname not in arec["functions"])
    return {"worst_func_cc_increase": worst, "worst_func": worst_name,
            "new_func_max_cc": new_max, "n_func_removed": removed}


# -------------------------------------------------------------------- core

def evaluate(analyzer, repo_path, base_commit, sources, instance_id):
    reset_to(repo_path, base_commit)
    records = []

    for source_name, patch_text in sources:
        entries = parse_diff(patch_text)
        modified, created, deleted = classify_paths(entries,
                                                    analyzer.extensions)
        # Measure only files that already exist at base_commit and still exist
        # afterwards. Created files have no "before" state and deleted ones have
        # no "after", so either one in the measured set skews every delta in a
        # known direction. Both are counted instead -- how much scratch an agent
        # leaves behind is interesting on its own. See README, trap 1.
        files = [f for f in modified
                 if os.path.isfile(os.path.join(repo_path, f))]
        # A path the diff calls a modification but that is absent at
        # base_commit is a creation the diff header failed to mark.
        created = sorted(set(created) | (set(modified) - set(files)))

        rec = {"instance_id": instance_id, "source": source_name,
               "n_files": len(files), "files": files,
               "n_new_files": len(created), "new_files": created,
               "n_deleted_files": len(deleted), "deleted_files": deleted}

        if not entries:
            # 752 of the published model patches are not diffs at all but the
            # stderr of a failed git command ("fatal: not a git repository").
            # That is a scaffold failure, not a patch that touched no Python.
            rec["status"] = "not_a_diff"
            rec["error"] = patch_text.strip()[:200]
            records.append(rec)
            continue

        if not files:
            rec["status"] = "no_measurable_files"
            records.append(rec)
            continue

        try:
            before = analyzer.run(repo_path, files)

            applied, err = apply_patch(repo_path, patch_text, base_commit)
            if not applied:
                rec["status"] = "apply_failed"
                rec["error"] = err
                records.append(rec)
                reset_to(repo_path, base_commit)
                continue

            after = analyzer.run(repo_path, files)

            broke = [f for f in after.parse_errors
                     if f not in before.parse_errors]
            if broke:
                # The patch left a measured file unparseable. Its metrics would
                # read as a perfect score, so this is its own outcome rather
                # than a quality delta. See README, trap 2.
                rec["status"] = "broke_syntax"
                rec["broken_files"] = broke
                records.append(rec)
                reset_to(repo_path, base_commit)
                continue

            # Files the analyzer declined to measure contribute zeros to both
            # sides. Symmetric, so the delta is unbiased, but they still pad the
            # file and line counts with code nobody looked at.
            skipped = sorted(set(before.skipped) | set(after.skipped))
            # A file the patch removed without saying so in the diff header. It
            # has no "after" state, so it belongs with the deletions, not in a
            # comparison where its absence reads as an improvement.
            vanished = [f for f in files if f not in after.metrics]
            if vanished:
                deleted = sorted(set(deleted) | set(vanished))
                rec["n_deleted_files"] = len(deleted)
                rec["deleted_files"] = deleted
            drop = set(skipped) | set(vanished)
            measured = [f for f in files if f not in drop]

        except (AnalyzerError, subprocess.SubprocessError, OSError) as exc:
            rec["status"] = "error"
            rec["error"] = str(exc)[:300]
            records.append(rec)
            reset_to(repo_path, base_commit)
            continue

        rec["skipped_files"] = skipped
        rec["n_skipped_files"] = len(skipped)
        rec["n_files"] = len(measured)
        rec["files"] = measured

        if not measured:
            # Nothing the patch touched survived to be compared. Its own
            # outcome, rather than the analyzer failure this used to surface as:
            # 29 django instances were dropped with a "produced no report"
            # error that said nothing about which files were involved or why.
            rec["status"] = "all_files_skipped"
            records.append(rec)
            reset_to(repo_path, base_commit)
            continue

        bmetrics = {f: before.metrics[f] for f in measured}
        ametrics = {f: after.metrics[f] for f in measured}
        b, a = aggregate(bmetrics), aggregate(ametrics)

        rec["status"] = "ok"
        rec["before"] = b
        rec["after"] = a
        rec["delta"] = {k: a[k] - b[k] for k in b}
        rec.update(function_deltas(bmetrics, ametrics))
        rec["patch_lines"] = changed_lines(entries, set(measured))
        rec["destructive"] = (
            b["n_func"] >= DESTRUCTIVE_MIN_FUNCS
            and a["n_func"] < DESTRUCTIVE_FUNC_RATIO * b["n_func"])
        records.append(rec)

        reset_to(repo_path, base_commit)

    return records


def load_sources(data_dir):
    models = {}
    for path in sorted(glob.glob(os.path.join(data_dir, "preds_*.jsonl"))):
        name = os.path.basename(path)[len("preds_"):-len(".jsonl")]
        preds = {}
        with open(path) as fh:
            for line in fh:
                line = line.strip()
                if line:
                    r = json.loads(line)
                    preds[r["instance_id"]] = r.get("model_patch") or ""
        models[name] = preds
    return models


def load_resolved(data_dir):
    resolved = {}
    for path in glob.glob(os.path.join(data_dir, "results_*.json")):
        name = os.path.basename(path)[len("results_"):-len(".json")]
        with open(path) as fh:
            resolved[name] = set(json.load(fh).get("resolved") or [])
    return resolved


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--data", default="data",
                    help="directory written by fetch_data.py")
    ap.add_argument("--repos", default="repos",
                    help="directory written by clone_repos.py")
    ap.add_argument("--repo", default="django/django",
                    help="which SWE-bench repository to evaluate")
    ap.add_argument("--language", default="python")
    ap.add_argument("--analyzer-bin", default=None)
    ap.add_argument("--limit", type=int, default=0, help="0 means all")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    analyzer = get_analyzer(args.language, binary=args.analyzer_bin)

    with open(os.path.join(args.data, "instances.json")) as fh:
        meta = json.load(fh)
    models = load_sources(args.data)
    resolved = load_resolved(args.data)

    repo_path = os.path.abspath(
        os.path.join(args.repos, args.repo.replace("/", "__")))
    if not os.path.isdir(os.path.join(repo_path, ".git")):
        sys.exit(f"{repo_path} is not a git clone; run clone_repos.py first")

    ids = sorted(i for i, m in meta.items() if m["repo"] == args.repo)
    if args.limit:
        ids = ids[:args.limit]
    print(f"{len(ids)} instances from {args.repo}, "
          f"{len(models) + 1} sources each", file=sys.stderr)

    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    written = 0
    # Written as we go. A full run is hours long and buffering it meant a crash
    # at instance 200 lost all 200.
    with open(args.out, "w") as fh:
        meta_line = {
            "record_type": "meta",
            "schema_version": SCHEMA_VERSION,
            "repo": args.repo,
            "language": args.language,
            "analyzer": analyzer.version(),
            "n_instances": len(ids),
            "sources": ["gold"] + sorted(models),
        }
        fh.write(json.dumps(meta_line) + "\n")
        fh.flush()

        for n, iid in enumerate(ids, 1):
            m = meta[iid]
            sources = [("gold", m["patch"])]
            for model, preds in sorted(models.items()):
                patch = preds.get(iid)
                if patch:
                    sources.append((model, patch))

            print(f"[{n}/{len(ids)}] {iid}", file=sys.stderr)
            try:
                recs = evaluate(analyzer, repo_path, m["base_commit"],
                                sources, iid)
            except RuntimeError as exc:
                print(f"    skipped: {exc}", file=sys.stderr)
                continue

            for r in recs:
                r["repo"] = args.repo
                r["difficulty"] = m.get("difficulty")
                if r["source"] != "gold":
                    r["resolved"] = iid in resolved.get(r["source"], set())
                fh.write(json.dumps(r) + "\n")
                written += 1
            fh.flush()

    print(f"wrote {written} records to {args.out}", file=sys.stderr)


if __name__ == "__main__":
    main()

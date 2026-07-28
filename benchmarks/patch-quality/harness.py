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

Run `fetch_data.py` and `clone_repos.py` first. See README.md for the three
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
# destructive rather than an improvement. These are reported separately; in
# practice the correctness gate already excludes them, which is the main
# argument for applying it. See README, trap 3.
DESTRUCTIVE_FUNC_RATIO = 0.7
DESTRUCTIVE_MIN_FUNCS = 10


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


def patched_files(patch_text, extensions):
    """Repo-relative paths of interest that a unified diff touches."""
    found = []
    for pattern in (r"^\+\+\+ b/(.+)$", r"^--- a/(.+)$"):
        for line in patch_text.splitlines():
            m = re.match(pattern, line)
            if not m:
                continue
            path = m.group(1).strip()
            if path.endswith(extensions) and path != "/dev/null":
                found.append(path)
    return sorted(set(found))


def apply_patch(repo, patch_text):
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


def worst_function_delta(before, after):
    """Largest complexity increase among functions present both before and after."""
    worst, name = 0, None
    for path, arec in after.items():
        brec = before.get(path)
        if not brec:
            continue
        for fname, ainfo in arec["functions"].items():
            binfo = brec["functions"].get(fname)
            if not binfo:
                continue
            delta = ainfo["cc"] - binfo["cc"]
            if delta > worst:
                worst, name = delta, f"{path}::{fname}"
    return worst, name


def measured_patch_lines(patch_text, measured):
    """Changed lines within the files actually measured.

    Patch size is the natural denominator for "how much complexity did this add
    per line changed", but the raw diff also contains scratch files and test
    files that are never measured. Counting those inflates the denominator and
    makes a bloated patch look efficient.
    """
    current, count = None, 0
    for line in patch_text.splitlines():
        m = re.match(r"^\+\+\+ b/(.+)$", line)
        if m:
            current = m.group(1).strip()
            continue
        if line.startswith(("--- ", "diff --git", "index ", "@@")):
            continue
        if current in measured and line[:1] in ("+", "-"):
            count += 1
    return count


# -------------------------------------------------------------------- core

def evaluate(analyzer, repo_path, base_commit, sources, instance_id):
    reset_to(repo_path, base_commit)
    records = []

    for source_name, patch_text in sources:
        touched = patched_files(patch_text, analyzer.extensions)
        # Measure only files that already exist at base_commit. Files the model
        # creates have no "before" state, so including them in "after" would
        # inflate every delta. They are counted separately -- how much scratch
        # an agent leaves behind is interesting on its own. See README, trap 1.
        files = [f for f in touched
                 if os.path.isfile(os.path.join(repo_path, f))]
        new_files = [f for f in touched if f not in files]

        rec = {"instance_id": instance_id, "source": source_name,
               "n_files": len(files), "files": files,
               "n_new_files": len(new_files), "new_files": new_files}

        if not files:
            rec["status"] = "no_measurable_files"
            records.append(rec)
            continue

        try:
            before, before_errors = analyzer.run(repo_path, files)

            applied, err = apply_patch(repo_path, patch_text)
            if not applied:
                rec["status"] = "apply_failed"
                rec["error"] = err
                records.append(rec)
                reset_to(repo_path, base_commit)
                continue

            after, after_errors = analyzer.run(repo_path, files)

            broke = [f for f in after_errors if f not in before_errors]
            if broke:
                # The patch left a measured file unparseable. Its metrics would
                # read as a perfect score, so this is its own outcome rather
                # than a quality delta. See README, trap 2.
                rec["status"] = "broke_syntax"
                rec["broken_files"] = broke
                records.append(rec)
                reset_to(repo_path, base_commit)
                continue

        except (AnalyzerError, subprocess.SubprocessError, OSError) as exc:
            rec["status"] = "error"
            rec["error"] = str(exc)[:300]
            records.append(rec)
            reset_to(repo_path, base_commit)
            continue

        b, a = aggregate(before), aggregate(after)
        worst, worst_name = worst_function_delta(before, after)

        rec["status"] = "ok"
        rec["before"] = b
        rec["after"] = a
        rec["delta"] = {k: a[k] - b[k] for k in b}
        rec["worst_func_cc_increase"] = worst
        rec["worst_func"] = worst_name
        rec["patch_lines"] = measured_patch_lines(patch_text, set(files))
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
        resolved[name] = set(json.load(open(path)).get("resolved") or [])
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

    meta = json.load(open(os.path.join(args.data, "instances.json")))
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

    out = []
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
            out.append(r)

    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    with open(args.out, "w") as fh:
        for r in out:
            fh.write(json.dumps(r) + "\n")
    print(f"wrote {len(out)} records to {args.out}", file=sys.stderr)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Summarise harness output.

Three things make the numbers mean what they appear to mean:

Pairing. Each model is compared against the human patch on exactly the instances
where both produced a measurable result. Intersecting across all models instead
would shrink the sample to whatever the weakest submission managed.

The correctness gate (--resolved-only). Restricting to patches that actually
passed the tests is not a refinement, it changes the answer. A patch that breaks
or guts a file scores as a large quality improvement, and those patches are
overwhelmingly the ones that fail their tests. Without the gate, model strength
appears to predict patch quality; with it, that relationship disappears.

What counts as a regression. Per-function, never the aggregate. The maximum
complexity over every function in every measured file only moves when a patch
touches the single most complex function in the set, which in django is usually
some 60-branch method nobody in the sample went near. Measured that way the
benchmark saw roughly a quarter of the complexity increases it should have.

Effect sizes here are small and event counts are low, so the paired McNemar
column is the one to read before concluding anything.
"""

import argparse
import glob
import gzip
import json
import math
import os
import statistics as st
import sys
from collections import defaultdict

MIN_PAIRED_INSTANCES = 20


def open_records(path):
    """Result sets are committed gzipped; accept either form.

    Returns (meta, records). The first line of a result set written by
    harness.py is a metadata record, not a measurement.
    """
    opener = gzip.open if path.endswith(".gz") else open
    meta, records = {}, []
    with opener(path, "rt") as fh:
        for line in fh:
            if not line.strip():
                continue
            obj = json.loads(line)
            if obj.get("record_type") == "meta":
                meta = obj
            else:
                records.append(obj)
    return meta, records


def open_all(paths):
    """Several result sets as one pool.

    One repository is a single codebase with a single set of conventions, so the
    interesting comparison spans all twelve. Instance ids are globally unique
    across SWE-bench, so records from different repositories pool without
    colliding -- but only if every set was produced by the same analyzer build,
    which is checked here rather than discovered later in the numbers.
    """
    metas, records = [], []
    for path in paths:
        meta, recs = open_records(path)
        if not recs:
            sys.exit(f"{path} contains no measurement records")
        metas.append((path, meta))
        records += recs

    analyzers = {m.get("analyzer") for _, m in metas if m.get("analyzer")}
    if len(analyzers) > 1:
        sys.exit("result sets were produced by different analyzer builds, so "
                 "their metrics are not comparable:\n"
                 + "\n".join(f"  {p}: {m.get('analyzer', '?')}"
                             for p, m in metas))

    merged = dict(metas[0][1]) if metas else {}
    merged["repo"] = ", ".join(
        sorted({m.get("repo", "?") for _, m in metas if m.get("repo")}))
    merged["n_instances"] = sum(m.get("n_instances", 0) for _, m in metas)
    return merged, records


def regressed(rec):
    """Did this patch make some measured metric worse?

    `worst_func_cc_increase` compares each function against itself, which is the
    only way to see a function getting harder to read. The aggregate max_cc that
    this used to test is nearly blind: over the django run it caught 256 of the
    1053 patches that raised a function's complexity.
    """
    d = rec["delta"]
    return (rec.get("worst_func_cc_increase", 0) > 0
            or d["n_high_risk"] > 0
            or d["dead_critical"] > 0 or d["dead_warning"] > 0)


def mcnemar_exact(b, c):
    """Two-sided exact test on discordant pairs."""
    n = b + c
    if n == 0:
        return 1.0
    tail = sum(math.comb(n, k) for k in range(min(b, c) + 1))
    return min(1.0, 2 * tail / 2 ** n)


def load_config(path):
    if not os.path.exists(path):
        return {}, {}
    with open(path) as fh:
        cfg = json.load(fh)
    subs = cfg.get("submissions") or []
    labels = {s["id"]: s.get("label", s["id"]) for s in subs}
    scaffolds = {s["id"]: s.get("scaffold", "?") for s in subs}
    return labels, scaffolds


def load_resolved(records, data_dir):
    """Resolved sets and dollar costs, preferring the result set itself.

    The harness stamps each record with the model's resolved status, so a
    committed result set carries its own correctness gate. Reading it back out of
    `data/` was a trap: that directory is gitignored, so from a fresh clone
    --resolved-only silently gated every model down to zero instances and
    printed an empty table.
    """
    resolved = defaultdict(set)
    for rec in records:
        if rec.get("resolved"):
            resolved[rec["source"]].add(rec["instance_id"])

    totals, cost = {}, {}
    for path in glob.glob(os.path.join(data_dir, "results_*.json")):
        name = os.path.basename(path)[len("results_"):-len(".json")]
        with open(path) as fh:
            payload = json.load(fh)
        totals[name] = len(payload.get("resolved") or [])
        cost[name] = payload.get("cost_total")
    return resolved, totals, cost


STATUSES = (
    ("ok", "ok"),
    ("broke_syntax", "broke"),
    ("apply_failed", "no-apply"),
    ("all_files_skipped", "skipped"),
    ("no_measurable_files", "no-files"),
    ("not_a_diff", "not-diff"),
    ("error", "error"),
    ("instance_failed", "git-fail"),
)


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("jsonl", nargs="+",
                    help="one or more result sets; several are pooled")
    ap.add_argument("--data", default="data",
                    help="optional; only used for resolve rate and cost")
    ap.add_argument("--config", default="config/models.json")
    ap.add_argument("--resolved-only", action="store_true",
                    help="restrict each model to the instances it actually solved")
    args = ap.parse_args()

    labels, scaffolds = load_config(args.config)
    labels["gold"] = "human (gold patch)"

    meta, records = open_all(args.jsonl)
    resolved, resolve_totals, cost = load_resolved(records, args.data)

    if args.resolved_only and not resolved:
        sys.exit("--resolved-only was requested but no record carries a "
                 "`resolved` field.\nThis result set predates the correctness "
                 "gate; re-run harness.py to regenerate it.")

    by_source = defaultdict(dict)
    status_counts = defaultdict(lambda: defaultdict(int))
    for rec in records:
        status_counts[rec["source"]][rec["status"]] += 1
        if rec["status"] == "ok":
            by_source[rec["source"]][rec["instance_id"]] = rec

    if "gold" not in by_source:
        sys.exit("no gold records; nothing to compare against")
    gold = by_source["gold"]

    if meta:
        print(f"analyzer: {meta.get('analyzer', '?')}   "
              f"repo: {meta.get('repo', '?')}   "
              f"instances: {meta.get('n_instances', '?')}")

    # Every status is printed. Two of them used to be counted and then dropped
    # from the table, which hid 451 of 3407 records -- including the 22 instances
    # where the human baseline itself went unmeasured.
    print("\n=== outcomes ===")
    print(f"{'source':26} " + " ".join(f"{h:>9}" for _, h in STATUSES)
          + f" {'destruct':>9}")
    for src in sorted(status_counts, key=lambda s: labels.get(s, s)):
        counts = status_counts[src]
        destructive = sum(1 for r in by_source[src].values()
                          if r.get("destructive"))
        cells = " ".join(f"{counts.get(s, 0):>9}" for s, _ in STATUSES)
        print(f"{labels.get(src, src)[:26]:26} {cells} {destructive:>9}")

    unknown = ({s for c in status_counts.values() for s in c}
               - {s for s, _ in STATUSES})
    if unknown:
        print(f"  (statuses not shown above: {', '.join(sorted(unknown))})")

    scope = "resolved by that model" if args.resolved_only else "measurable for both"
    print(f"\n=== quality vs the human patch (instances {scope}) ===")

    groups = defaultdict(list)
    dropped = []
    for src in by_source:
        if src == "gold":
            continue
        ids = set(by_source[src]) & set(gold)
        if args.resolved_only:
            ids &= resolved.get(src, set())
        # A destructive patch guts the files it touches, so it scores as a large
        # improvement and would count as a *non*-regression for its model. It is
        # not evidence of clean code either way; report it, do not average it in.
        n_destructive = sum(1 for i in ids
                            if by_source[src][i].get("destructive")
                            or gold[i].get("destructive"))
        ids = {i for i in ids
               if not by_source[src][i].get("destructive")
               and not gold[i].get("destructive")}
        if len(ids) < MIN_PAIRED_INSTANCES:
            dropped.append((labels.get(src, src), len(ids)))
            continue

        rows = [by_source[src][i] for i in ids]
        model_bad = {i for i in ids if regressed(by_source[src][i])}
        human_bad = {i for i in ids if regressed(gold[i])}
        b = len(model_bad - human_bad)
        c = len(human_bad - model_bad)
        total_resolved = resolve_totals.get(src)

        groups[scaffolds.get(src, "?")].append({
            "label": labels.get(src, src),
            "n": len(ids),
            "excluded": n_destructive,
            "model_reg": len(model_bad),
            "human_reg": len(human_bad),
            "b": b, "c": c,
            "p": mcnemar_exact(b, c),
            "sum_cc": st.mean(r["delta"]["sum_cc"] for r in rows),
            "junk": st.mean(r.get("n_new_files", 0) for r in rows),
            "resolve": total_resolved / 500 if total_resolved else None,
            "cost": cost.get(src),
        })

    if not groups:
        print(f"\nno model reached {MIN_PAIRED_INSTANCES} paired instances; "
              "nothing to compare.")
        if dropped:
            for label, n in sorted(dropped):
                print(f"  {label:24} {n:4} paired instances")
        return 1

    header = (f"{'model':24} {'n':>4} {'resolve':>8} {'worse':>6} {'human':>6} "
              f"{'p':>7} {'Σcc':>6} {'junk':>6} {'$':>6}")
    for scaffold in sorted(groups):
        print(f"\n-- scaffold: {scaffold} --")
        print(header)
        for r in sorted(groups[scaffold], key=lambda x: x["p"]):
            mark = " *" if r["p"] < 0.05 else ""
            money = f"${r['cost']:.0f}" if r["cost"] else "-"
            rate = f"{r['resolve']*100:7.1f}%" if r["resolve"] else "       -"
            print(f"{r['label'][:24]:24} {r['n']:4} {rate} "
                  f"{r['model_reg']:6} {r['human_reg']:6} {r['p']:7.3f}{mark} "
                  f"{r['sum_cc']:6.2f} {r['junk']:6.2f} {money:>6}")

    rows = [r for v in groups.values() for r in v]
    total = len(rows)
    signif = sum(1 for r in rows if r["p"] < 0.05)
    bonferroni = 0.05 / total if total else 0
    survives = sum(1 for r in rows if r["p"] < bonferroni)

    # Each model's own test is underpowered: it asks ~100 paired instances to
    # resolve a few points of difference, so most land at p>0.05 whatever the
    # truth is. Whether every model lands on the *same side* of the human is a
    # separate question, and a much better powered one. Reporting only the
    # per-model tests reads their inconclusiveness as evidence of no effect.
    worse = sum(1 for r in rows if r["model_reg"] > r["human_reg"])
    better = sum(1 for r in rows if r["model_reg"] < r["human_reg"])
    print(f"\n{worse} of {total} models regress more often than the human "
          f"patch, {better} less often;")
    print(f"sign test across models p = {mcnemar_exact(worse, better):.2e}.")

    b_tot = sum(r["b"] for r in rows)
    c_tot = sum(r["c"] for r in rows)
    if c_tot:
        print(f"Pooled over every model, {b_tot + c_tot} instances split the "
              f"two sides: {b_tot} where only the\nmodel regressed against "
              f"{c_tot} where only the human did ({b_tot / c_tot:.2f}x). "
              "Descriptive only --\nevery model shares one human baseline, so "
              "these pairs are not independent.")

    print(f"\n{signif} of {total} models differ from the human patch at p<0.05; "
          f"{survives} survive Bonferroni correction (alpha={bonferroni:.4f}).")
    print("'worse' and 'human' count instances where that side's patch raised a "
          "function's\ncomplexity, added a high-risk function, or added dead "
          "code.\n'p' is a two-sided exact McNemar test over the discordant "
          "pairs.")

    excluded = sum(r["excluded"] for v in groups.values() for r in v)
    if excluded:
        print(f"{excluded} destructive pair(s) excluded from the comparison; "
              "see the outcomes table.")
    if dropped:
        print(f"{len(dropped)} model(s) below {MIN_PAIRED_INSTANCES} paired "
              f"instances and not shown: "
              + ", ".join(f"{lbl} ({n})" for lbl, n in sorted(dropped)))

    if not args.resolved_only:
        print("\nNOTE: run with --resolved-only for the headline number. "
              "Without the correctness\ngate, patches that break or delete code "
              "count as quality improvements.")
    return 0


if __name__ == "__main__":
    sys.exit(main())

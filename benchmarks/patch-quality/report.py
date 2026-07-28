#!/usr/bin/env python3
"""Summarise harness output.

Two things make the numbers mean what they appear to mean:

Pairing. Each model is compared against the human patch on exactly the instances
where both produced a measurable result. Intersecting across all models instead
would shrink the sample to whatever the weakest submission managed.

The correctness gate (--resolved-only). Restricting to patches that actually
passed the tests is not a refinement, it changes the answer. A patch that breaks
or guts a file scores as a large quality improvement, and those patches are
overwhelmingly the ones that fail their tests. Without the gate, model strength
appears to predict patch quality; with it, that relationship disappears.

Effect sizes here are small and event counts are low, so the paired McNemar
column is the one to read before concluding anything.
"""

import argparse
import glob
import json
import math
import os
import statistics as st
import sys
from collections import defaultdict


def regressed(rec):
    d = rec["delta"]
    return (d["max_cc"] > 0 or d["n_high_risk"] > 0
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
    cfg = json.load(open(path))
    labels = {s["id"]: s.get("label", s["id"]) for s in cfg["submissions"]}
    scaffolds = {s["id"]: s.get("scaffold", "?") for s in cfg["submissions"]}
    return labels, scaffolds


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("jsonl")
    ap.add_argument("--data", default="data")
    ap.add_argument("--config", default="config/models.json")
    ap.add_argument("--resolved-only", action="store_true",
                    help="restrict each model to the instances it actually solved")
    args = ap.parse_args()

    labels, scaffolds = load_config(args.config)
    labels["gold"] = "human (gold patch)"

    records = [json.loads(l) for l in open(args.jsonl) if l.strip()]

    resolved, cost = {}, {}
    for path in glob.glob(os.path.join(args.data, "results_*.json")):
        name = os.path.basename(path)[len("results_"):-len(".json")]
        payload = json.load(open(path))
        resolved[name] = set(payload.get("resolved") or [])
        cost[name] = payload.get("cost_total")

    by_source = defaultdict(dict)
    status_counts = defaultdict(lambda: defaultdict(int))
    for rec in records:
        status_counts[rec["source"]][rec["status"]] += 1
        if rec["status"] == "ok":
            by_source[rec["source"]][rec["instance_id"]] = rec

    if "gold" not in by_source:
        sys.exit("no gold records; nothing to compare against")
    gold = by_source["gold"]

    print("=== outcomes ===")
    print(f"{'source':28} {'ok':>5} {'broke_syntax':>13} "
          f"{'apply_failed':>13} {'destructive':>12}")
    for src in sorted(status_counts, key=lambda s: labels.get(s, s)):
        counts = status_counts[src]
        destructive = sum(1 for r in by_source[src].values()
                          if r.get("destructive"))
        print(f"{labels.get(src, src)[:28]:28} {counts['ok']:5} "
              f"{counts['broke_syntax']:13} {counts['apply_failed']:13} "
              f"{destructive:12}")

    scope = "resolved by that model" if args.resolved_only else "measurable for both"
    print(f"\n=== quality vs the human patch (instances {scope}) ===")

    groups = defaultdict(list)
    for src in by_source:
        if src == "gold":
            continue
        ids = set(by_source[src]) & set(gold)
        if args.resolved_only:
            ids &= resolved.get(src, set())
        if len(ids) < 20:
            continue

        rows = [by_source[src][i] for i in ids]
        model_bad = {i for i in ids if regressed(by_source[src][i])}
        human_bad = {i for i in ids if regressed(gold[i])}
        b = len(model_bad - human_bad)
        c = len(human_bad - model_bad)

        groups[scaffolds.get(src, "?")].append({
            "label": labels.get(src, src),
            "n": len(ids),
            "model_reg": len(model_bad),
            "human_reg": len(human_bad),
            "b": b, "c": c,
            "p": mcnemar_exact(b, c),
            "sum_cc": st.mean(r["delta"]["sum_cc"] for r in rows),
            "junk": st.mean(r.get("n_new_files", 0) for r in rows),
            "resolve": len(resolved.get(src, ())) / 500,
            "cost": cost.get(src),
        })

    header = (f"{'model':24} {'n':>4} {'resolve':>8} {'worse':>6} {'human':>6} "
              f"{'p':>7} {'Σcc':>6} {'junk':>6} {'$':>6}")
    for scaffold in sorted(groups):
        print(f"\n-- scaffold: {scaffold} --")
        print(header)
        for r in sorted(groups[scaffold], key=lambda x: x["p"]):
            mark = " *" if r["p"] < 0.05 else ""
            money = f"${r['cost']:.0f}" if r["cost"] else "-"
            print(f"{r['label'][:24]:24} {r['n']:4} {r['resolve']*100:7.1f}% "
                  f"{r['model_reg']:6} {r['human_reg']:6} {r['p']:7.3f}{mark} "
                  f"{r['sum_cc']:6.2f} {r['junk']:6.2f} {money:>6}")

    total = sum(len(v) for v in groups.values())
    signif = sum(1 for v in groups.values() for r in v if r["p"] < 0.05)
    bonferroni = 0.05 / total if total else 0
    survives = sum(1 for v in groups.values() for r in v
                   if r["p"] < bonferroni)
    print(f"\n{signif} of {total} models differ from the human patch at p<0.05; "
          f"{survives} survive Bonferroni correction (alpha={bonferroni:.4f}).")
    print("'worse' and 'human' count instances where that side's patch made "
          "some metric worse.\n'p' is a two-sided exact McNemar test over the "
          "discordant pairs.")

    if not args.resolved_only:
        print("\nNOTE: run with --resolved-only for the headline number. "
              "Without the correctness\ngate, patches that break or delete code "
              "count as quality improvements.")


if __name__ == "__main__":
    main()

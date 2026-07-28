#!/usr/bin/env python3
"""Download everything the benchmark needs. Stdlib only, no API keys.

Three sources, because no single one has all of it:

  instances + gold patches
      HuggingFace datasets-server, princeton-nlp/SWE-bench_Verified.
      The `patch` field is the real commit a maintainer wrote to fix the issue,
      which is what makes a human baseline possible.

  model patches
      The swe-bench-submissions S3 bucket. The GitHub repo swe-bench/experiments
      documents all_preds.jsonl in its README but no longer contains any: at
      HEAD there are zero, and even in history only four submissions ever had
      one. S3 has them for ~100 submissions and is readable without credentials
      as long as a prefix is supplied (a bare bucket listing is denied).

  resolved status
      The correctness gate, from GitHub. Two shapes depending on split:
      verified  -> results/results.json, already a list of resolved ids
      bash-only -> per_instance_details.json, a per-instance dict that also
                   carries cost and api_calls
"""

import argparse
import concurrent.futures
import json
import os
import sys
import urllib.error
import urllib.request

HF_ROWS = ("https://datasets-server.huggingface.co/rows"
           "?dataset=princeton-nlp/SWE-bench_Verified"
           "&config=default&split=test&offset={offset}&length={length}")
S3 = "https://swe-bench-submissions.s3.amazonaws.com/"
GH = "https://raw.githubusercontent.com/swe-bench/experiments/main/evaluation/"

INSTANCE_FIELDS = ("instance_id", "repo", "base_commit", "patch",
                   "test_patch", "version", "difficulty")


def get(url, timeout=300):
    with urllib.request.urlopen(url, timeout=timeout) as resp:
        return resp.read()


def fetch_instances(out_dir):
    dest = os.path.join(out_dir, "instances.json")
    if os.path.exists(dest):
        print(f"instances.json already present, skipping")
        return json.load(open(dest))

    rows = []
    for offset in range(0, 500, 100):
        payload = json.loads(get(HF_ROWS.format(offset=offset, length=100)))
        rows += [r["row"] for r in payload["rows"]]
        print(f"  instances {len(rows)}/500")

    meta = {r["instance_id"]: {k: r.get(k) for k in INSTANCE_FIELDS}
            for r in rows}
    with open(dest, "w") as fh:
        json.dump(meta, fh)
    return meta


def fetch_one(sub, out_dir):
    sid, split = sub["id"], sub["split"]
    preds_path = os.path.join(out_dir, f"preds_{sid}.jsonl")
    results_path = os.path.join(out_dir, f"results_{sid}.json")

    try:
        if not os.path.exists(preds_path):
            data = get(f"{S3}{split}/{sid}/all_preds.jsonl")
            with open(preds_path, "wb") as fh:
                fh.write(data)

        if not os.path.exists(results_path):
            if split == "bash-only":
                detail = json.loads(
                    get(f"{GH}{split}/{sid}/per_instance_details.json"))
                payload = {
                    "resolved": [k for k, v in detail.items()
                                 if v.get("resolved")],
                    "cost_total": sum(v.get("cost") or 0
                                      for v in detail.values()),
                    "api_calls_total": sum(v.get("api_calls") or 0
                                           for v in detail.values()),
                }
            else:
                res = json.loads(get(f"{GH}{split}/{sid}/results/results.json"))
                payload = {"resolved": res.get("resolved") or []}
            with open(results_path, "w") as fh:
                json.dump(payload, fh)
    except urllib.error.HTTPError as exc:
        return sid, f"HTTP {exc.code} on {exc.url}"
    except (urllib.error.URLError, OSError, ValueError) as exc:
        return sid, str(exc)[:160]

    n_preds = sum(1 for _ in open(preds_path))
    n_resolved = len(json.load(open(results_path))["resolved"])
    return sid, f"{n_preds} patches, {n_resolved} resolved"


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--out", default="data")
    ap.add_argument("--config", default="config/models.json")
    ap.add_argument("--jobs", type=int, default=6)
    args = ap.parse_args()

    os.makedirs(args.out, exist_ok=True)
    config = json.load(open(args.config))
    submissions = config["submissions"]

    meta = fetch_instances(args.out)
    print(f"{len(meta)} instances\n")

    failures = []
    with concurrent.futures.ThreadPoolExecutor(args.jobs) as pool:
        futures = [pool.submit(fetch_one, s, args.out) for s in submissions]
        for future in concurrent.futures.as_completed(futures):
            sid, status = future.result()
            print(f"{sid[:52]:52} {status}")
            if "HTTP" in status or "Errno" in status:
                failures.append(sid)

    if failures:
        print(f"\n{len(failures)} submission(s) failed: {', '.join(failures)}",
              file=sys.stderr)
        return 1
    print(f"\nwrote {args.out}/")
    return 0


if __name__ == "__main__":
    sys.exit(main())

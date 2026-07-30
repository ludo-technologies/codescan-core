#!/usr/bin/env python3
"""Clone the repositories SWE-bench Verified draws from.

Full history is required. Every instance pins a `base_commit` that can be
years old, and the harness checks it out directly, so a shallow clone is not
enough.

Where you put these matters. Analyzers discover configuration by walking up the
directory tree, so a repository nested underneath a polyscan or pyscn checkout
inherits that checkout's config and is measured under different thresholds than
you think. Keep the clone directory outside any analyzer source tree. The
default `repos/` under this benchmark directory is fine because it is gitignored
and carries no analyzer config, but if you relocate it, check the ancestors.
"""

import argparse
import concurrent.futures
import json
import os
import subprocess
import sys

CONFIG_NAMES = (".pyscn.toml", ".jscan.toml", "polyscan.toml")


def run(*args, cwd=None):
    return subprocess.run(args, cwd=cwd, capture_output=True, text=True)


def warn_on_inherited_config(path):
    """Walk up from the clone directory looking for analyzer config."""
    found = []
    current = os.path.abspath(path)
    while True:
        for name in CONFIG_NAMES:
            candidate = os.path.join(current, name)
            if os.path.isfile(candidate):
                found.append(candidate)
        parent = os.path.dirname(current)
        if parent == current:
            break
        current = parent
    return found


def clone_one(repo, dest_root):
    dest = os.path.join(dest_root, repo.replace("/", "__"))
    if os.path.isdir(os.path.join(dest, ".git")):
        proc = run("git", "-C", dest, "rev-parse", "--is-shallow-repository")
        if proc.stdout.strip() == "true":
            unshallow = run("git", "-C", dest, "fetch", "--unshallow")
            if unshallow.returncode != 0:
                return repo, f"present but shallow, unshallow failed: " \
                             f"{unshallow.stderr.strip()[:120]}"
            return repo, "unshallowed"
        return repo, "already present"

    proc = run("git", "clone", "--quiet",
               f"https://github.com/{repo}.git", dest)
    if proc.returncode != 0:
        return repo, f"clone failed: {proc.stderr.strip()[:160]}"

    # The harness checks out thousands of commits in here. Git's background
    # maintenance takes the index lock while it does, and a checkout that loses
    # that race costs a whole instance.
    run("git", "-C", dest, "config", "gc.auto", "0")
    run("git", "-C", dest, "config", "maintenance.auto", "false")

    count = run("git", "-C", dest, "rev-list", "--count", "HEAD").stdout.strip()
    return repo, f"cloned, {count} commits"


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--data", default="data")
    ap.add_argument("--out", default="repos")
    ap.add_argument("--repo", action="append",
                    help="clone only this repo (repeatable); default is all")
    ap.add_argument("--jobs", type=int, default=4)
    args = ap.parse_args()

    instances_path = os.path.join(args.data, "instances.json")
    if not os.path.exists(instances_path):
        sys.exit(f"{instances_path} not found; run fetch_data.py first")

    with open(instances_path) as fh:
        meta = json.load(fh)
    counts = {}
    for m in meta.values():
        counts[m["repo"]] = counts.get(m["repo"], 0) + 1

    repos = args.repo or sorted(counts, key=lambda r: -counts[r])
    os.makedirs(args.out, exist_ok=True)

    inherited = warn_on_inherited_config(args.out)
    if inherited:
        print("WARNING: analyzer config found above the clone directory.\n"
              "Cloned repositories will inherit it and be measured under its\n"
              "thresholds rather than the defaults:", file=sys.stderr)
        for path in inherited:
            print(f"  {path}", file=sys.stderr)
        print(file=sys.stderr)

    print(f"cloning {len(repos)} repositories into {args.out}/ "
          f"(full history, expect several GB)\n")

    failed = []
    with concurrent.futures.ThreadPoolExecutor(args.jobs) as pool:
        futures = [pool.submit(clone_one, r, args.out) for r in repos]
        for future in concurrent.futures.as_completed(futures):
            repo, status = future.result()
            print(f"{repo:32} {counts.get(repo, 0):4} instances  {status}")
            if "failed" in status:
                failed.append(repo)

    if failed:
        print(f"\nfailed: {', '.join(failed)}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

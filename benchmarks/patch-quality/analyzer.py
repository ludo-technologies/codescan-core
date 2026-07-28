"""Analyzer adapters.

The benchmark measures a patch by running a polyscan analyzer over the files the
patch touches, before and after applying it. Only per-function metrics that are
computed within a single file are used:

    complexity  - McCabe cyclomatic complexity, counted from the CFG of one
                  function in one file
    dead code   - unreachable blocks, likewise per function

Coupling (CBO), cohesion (LCOM), clone detection and dependency analysis are
deliberately excluded. They are only meaningful over a whole codebase; run on a
subset they undercount, because the files they would have referenced are not
there. Measuring them here would make the numbers depend on which files a patch
happened to touch.
"""

import glob
import json
import os
import re
import shutil
import subprocess
import tempfile


class AnalyzerError(RuntimeError):
    pass


def _resolve_binary(name, env_var):
    override = os.environ.get(env_var)
    if override:
        if not os.path.isfile(override):
            raise AnalyzerError(f"{env_var}={override} is not a file")
        return override
    found = shutil.which(name)
    if not found:
        raise AnalyzerError(
            f"{name} not found on PATH; set {env_var} to its location")
    return found


def _rel(path, repo_root):
    return os.path.relpath(os.path.realpath(path), os.path.realpath(repo_root))


EMPTY_FILE_METRICS = {
    "functions": {}, "max_cc": 0, "sum_cc": 0, "n_func": 0,
    "n_high_risk": 0, "dead_critical": 0, "dead_warning": 0, "dead_info": 0,
}


class PyscnAnalyzer:
    """Adapter for pyscn (Python)."""

    language = "python"
    extensions = (".py",)

    def __init__(self, binary=None):
        self.binary = binary or _resolve_binary("pyscn", "PYSCN_BIN")

    def run(self, repo_root, rel_files):
        """Analyze the given repo-relative files.

        Returns (per_file_metrics, files_that_failed_to_parse).
        """
        # The analyzer runs from a private working directory, so every path
        # handed to it has to be absolute regardless of what the caller passed.
        repo_root = os.path.abspath(repo_root)
        existing = [f for f in rel_files
                    if os.path.isfile(os.path.join(repo_root, f))]
        if not existing:
            return {}, []

        # pyscn writes its report under the working directory, so give each run
        # a private one and read back the single report it produces.
        tmp = tempfile.mkdtemp(prefix="pq_pyscn_")
        try:
            cmd = [self.binary, "analyze", "--json", "--no-open",
                   "--select", "complexity,deadcode",
                   "--min-severity", "info"]
            cmd += [os.path.join(repo_root, f) for f in existing]
            proc = subprocess.run(cmd, cwd=tmp, capture_output=True,
                                  text=True, timeout=600)
            reports = glob.glob(os.path.join(tmp, ".pyscn", "reports", "*.json"))
            if not reports:
                raise AnalyzerError(
                    f"pyscn produced no report (rc={proc.returncode}): "
                    f"{proc.stderr.strip()[:300]}")
            with open(sorted(reports)[-1]) as fh:
                data = json.load(fh)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

        return self._extract(data, repo_root, existing), \
            self._parse_errors(data, repo_root)

    @staticmethod
    def _parse_errors(data, repo_root):
        """Files pyscn could not parse.

        This matters more than it looks. A file that fails to parse yields zero
        functions, and zero functions means zero complexity and zero dead code,
        so a patch that breaks the syntax of a file registers as a large quality
        improvement unless it is caught here. See README, trap 2.
        """
        out = []
        for msg in ((data.get("complexity") or {}).get("Errors") or []):
            if "Parse error" in msg or "syntax error" in msg.lower():
                m = re.match(r"^\[([^\]]+)\]", msg)
                out.append(_rel(m.group(1), repo_root) if m else msg)
        return out

    @staticmethod
    def _extract(data, repo_root, expected):
        per_file = {f: dict(EMPTY_FILE_METRICS, functions={}) for f in expected}

        # complexity.Functions has no JSON struct tags on the Go side, so its
        # keys are PascalCase while the rest of the report is snake_case.
        comp = data.get("complexity") or {}
        for fn in (comp.get("Functions") or []):
            rel = _rel(fn["FilePath"], repo_root)
            rec = per_file.get(rel)
            if rec is None:
                continue
            metrics = fn.get("Metrics") or {}
            cc = metrics.get("Complexity", 0)
            risk = (fn.get("RiskLevel") or "").lower()
            rec["functions"][fn["Name"]] = {
                "cc": cc,
                "cognitive": metrics.get("CognitiveComplexity", 0),
                "nesting": metrics.get("NestingDepth", 0),
                "risk": risk,
            }
            rec["n_func"] += 1
            rec["sum_cc"] += cc
            rec["max_cc"] = max(rec["max_cc"], cc)
            if risk == "high":
                rec["n_high_risk"] += 1

        for entry in ((data.get("dead_code") or {}).get("files") or []):
            rel = _rel(entry["file_path"], repo_root)
            rec = per_file.get(rel)
            if rec is None:
                continue
            for func in (entry.get("functions") or []):
                rec["dead_critical"] += func.get("critical_count", 0)
                rec["dead_warning"] += func.get("warning_count", 0)
                rec["dead_info"] += func.get("info_count", 0)

        return per_file


ANALYZERS = {"python": PyscnAnalyzer}


def get_analyzer(language, binary=None):
    try:
        cls = ANALYZERS[language]
    except KeyError:
        raise AnalyzerError(
            f"no analyzer for {language!r}; available: {sorted(ANALYZERS)}")
    return cls(binary=binary)

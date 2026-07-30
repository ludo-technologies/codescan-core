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

An adapter's `run(repo_root, rel_files)` returns an `Analysis`:

    metrics       {rel_path: per-file metrics}
    parse_errors  [rel_path] the analyzer could not parse
    skipped       [rel_path] the analyzer silently declined to analyze

`skipped` exists because absence of metrics is ambiguous. A file with no
reported functions may have none, or may have been filtered out by the
analyzer's own default excludes -- pyscn drops everything under `tests/`, for
instance. Both look identical in the report, and a silently skipped file
contributes zeros to before and after while still counting toward the file and
line totals. The adapter is responsible for telling the two apart.
"""

import ast
import glob
import json
import os
import re
import shutil
import subprocess
import tempfile

# pyscn's --min-complexity defaults to 5 and filters the *reported* function
# list, not just a display threshold. Left at the default, django's
# sql/query.py reports 29 functions summing to 277 complexity when the file
# actually holds 105 summing to 425: 72% of functions invisible. Worse, the
# threshold makes the measurement non-monotonic. A function simplified from 6
# to 4 drops out of the report entirely and reads as a deleted function, while
# one that grows from 3 to 8 appears out of nowhere and so cannot be compared
# against its own "before" value. Every function has to be reported for a
# before/after diff to mean anything.
MIN_COMPLEXITY = 1

ANALYZER_TIMEOUT = 600


class AnalyzerError(RuntimeError):
    pass


class Analysis:
    """What one analyzer invocation observed."""

    __slots__ = ("metrics", "parse_errors", "skipped", "version")

    def __init__(self, metrics, parse_errors, skipped, version=None):
        self.metrics = metrics
        self.parse_errors = parse_errors
        self.skipped = skipped
        self.version = version


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


def _empty_metrics():
    return dict(EMPTY_FILE_METRICS, functions={})


class PyscnAnalyzer:
    """Adapter for pyscn (Python)."""

    language = "python"
    extensions = (".py",)

    def __init__(self, binary=None):
        self.binary = binary or _resolve_binary("pyscn", "PYSCN_BIN")
        self._version = None

    def version(self):
        if self._version is None:
            proc = subprocess.run([self.binary, "version"],
                                  capture_output=True, text=True)
            first = (proc.stdout or proc.stderr).strip().splitlines()
            self._version = first[0].strip() if first else "unknown"
        return self._version

    def run(self, repo_root, rel_files):
        """Analyze the given repo-relative files. Returns an Analysis."""
        # The analyzer runs from a private working directory, so every path
        # handed to it has to be absolute regardless of what the caller passed.
        repo_root = os.path.abspath(repo_root)
        existing = [f for f in rel_files
                    if os.path.isfile(os.path.join(repo_root, f))]
        if not existing:
            return Analysis({}, [], [], self.version())

        # Our own parse of each file, independent of pyscn. Serves two purposes:
        # it is a second opinion on syntax (trap 2) that does not depend on
        # pyscn's error message wording, and it tells us whether a file with no
        # reported functions genuinely has none.
        own = {f: _own_parse(os.path.join(repo_root, f)) for f in existing}

        data = self._invoke(repo_root, existing)
        metrics = self._extract(data, repo_root, existing)

        parse_errors = self._parse_errors(data, repo_root)
        parse_errors += [f for f, (ok, _) in own.items()
                         if not ok and f not in parse_errors]

        # A file our parse found functions in, but pyscn reported nothing for,
        # was filtered out rather than measured. Reporting it as zeros would
        # silently pad the file and line counts with unmeasured code.
        skipped = [f for f in existing
                   if own[f][0] and own[f][1] > 0
                   and metrics[f]["n_func"] == 0
                   and f not in parse_errors]

        return Analysis(metrics, sorted(set(parse_errors)), sorted(skipped),
                        self.version())

    def _invoke(self, repo_root, existing):
        # pyscn writes its report under the working directory, so give each run
        # a private one and read back the single report it produces.
        tmp = tempfile.mkdtemp(prefix="pq_pyscn_")
        try:
            cmd = [self.binary, "analyze", "--json", "--no-open",
                   "--select", "complexity,deadcode",
                   "--min-severity", "info",
                   "--min-complexity", str(MIN_COMPLEXITY)]
            cmd += [os.path.join(repo_root, f) for f in existing]
            proc = subprocess.run(cmd, cwd=tmp, capture_output=True,
                                  text=True, timeout=ANALYZER_TIMEOUT)
            reports = glob.glob(os.path.join(tmp, ".pyscn", "reports", "*.json"))
            if not reports:
                raise AnalyzerError(
                    f"pyscn produced no report (rc={proc.returncode}) for "
                    f"{len(existing)} file(s): {proc.stderr.strip()[:300]}")
            with open(sorted(reports)[-1]) as fh:
                return json.load(fh)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    @staticmethod
    def _parse_errors(data, repo_root):
        """Files pyscn reported as unparseable.

        This matters more than it looks. A file that fails to parse yields zero
        functions, and zero functions means zero complexity and zero dead code,
        so a patch that breaks the syntax of a file registers as a large quality
        improvement unless it is caught here. See README, trap 2.

        The match is on pyscn's message text, which is not a stable interface,
        so `run` also folds in an independent parse of every file. Neither
        source is trusted alone.
        """
        out = []
        for msg in ((data.get("complexity") or {}).get("Errors") or []):
            if "parse error" in msg.lower() or "syntax error" in msg.lower():
                m = re.match(r"^\[([^\]]+)\]", msg)
                out.append(_rel(m.group(1), repo_root) if m else msg)
        return out

    @staticmethod
    def _extract(data, repo_root, expected):
        per_file = {f: _empty_metrics() for f in expected}

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


def _own_parse(abs_path):
    """(parses_cleanly, number_of_functions) via the stdlib.

    A deliberately independent second opinion. `ast` and pyscn's tree-sitter
    grammar do not accept exactly the same language, so a disagreement is only
    ever used differentially -- a file that already failed to parse before the
    patch is not blamed on the patch.
    """
    try:
        with open(abs_path, "rb") as fh:
            tree = ast.parse(fh.read())
    except (SyntaxError, ValueError, UnicodeDecodeError, OSError):
        return False, 0
    n = sum(1 for node in ast.walk(tree)
            if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)))
    return True, n


ANALYZERS = {"python": PyscnAnalyzer}


def get_analyzer(language, binary=None):
    try:
        cls = ANALYZERS[language]
    except KeyError:
        raise AnalyzerError(
            f"no analyzer for {language!r}; available: {sorted(ANALYZERS)}")
    return cls(binary=binary)

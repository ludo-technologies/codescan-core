#!/usr/bin/env python3
"""Unit tests for the pure parts of the benchmark. Stdlib only.

    python3 -m unittest discover -s tests -v

Everything here is a function whose wrongness would silently change a published
number rather than raise, which is the reason it is tested at all.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import analyzer                                              # noqa: E402
import harness                                               # noqa: E402
import report                                                # noqa: E402

MODIFY = """\
diff --git a/pkg/mod.py b/pkg/mod.py
index 1111111..2222222 100644
--- a/pkg/mod.py
+++ b/pkg/mod.py
@@ -1,2 +1,4 @@
 def f(x):
-    return x
+    if x:
+        return x
+    return 0

"""

CREATE = """\
diff --git a/scratch/check_url.py b/scratch/check_url.py
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/scratch/check_url.py
@@ -0,0 +1,2 @@
+def probe():
+    pass
"""

DELETE = """\
diff --git a/pkg/gone.py b/pkg/gone.py
deleted file mode 100644
index 4444444..0000000
--- a/pkg/gone.py
+++ /dev/null
@@ -1,2 +0,0 @@
-def dead():
-    pass
"""

RENAME = """\
diff --git a/pkg/old.py b/pkg/new.py
similarity index 90%
rename from pkg/old.py
rename to pkg/new.py
index 5555555..6666666 100644
--- a/pkg/old.py
+++ b/pkg/new.py
@@ -1,2 +1,2 @@
-def a():
+def b():
     pass
"""

# A hunk body containing lines that look exactly like diff file headers.
ADVERSARIAL = """\
diff --git a/pkg/doc.py b/pkg/doc.py
--- a/pkg/doc.py
+++ b/pkg/doc.py
@@ -1,2 +1,5 @@
 SAMPLE = '''
+--- a/decoy.py
+++ b/decoy.py
+@@ -1,1 +1,1 @@
 '''

"""

NOT_A_DIFF = "fatal: not a git repository (or any of the parent directories)\n"


class TestParseDiff(unittest.TestCase):

    def test_modification_has_matching_old_and_new(self):
        self.assertEqual(harness.parse_diff(MODIFY),
                         [{"old": "pkg/mod.py", "new": "pkg/mod.py",
                           "changed": 4}])

    def test_creation_has_no_pre_image(self):
        entries = harness.parse_diff(CREATE)
        self.assertEqual(entries, [{"old": None, "new": "scratch/check_url.py",
                                    "changed": 2}])

    def test_deletion_has_no_post_image(self):
        entries = harness.parse_diff(DELETE)
        self.assertEqual(entries, [{"old": "pkg/gone.py", "new": None,
                                    "changed": 2}])

    def test_rename_keeps_both_paths(self):
        entries = harness.parse_diff(RENAME)
        self.assertEqual(entries[0]["old"], "pkg/old.py")
        self.assertEqual(entries[0]["new"], "pkg/new.py")

    def test_header_lines_inside_a_hunk_are_content(self):
        """The regression this guards: source that looks like a diff header."""
        entries = harness.parse_diff(ADVERSARIAL)
        self.assertEqual([e["new"] for e in entries], ["pkg/doc.py"])
        self.assertEqual(entries[0]["changed"], 3)

    def test_multi_file_patch(self):
        entries = harness.parse_diff(MODIFY + CREATE + DELETE)
        self.assertEqual([e["new"] for e in entries],
                         ["pkg/mod.py", "scratch/check_url.py", None])

    def test_non_diff_yields_nothing(self):
        self.assertEqual(harness.parse_diff(NOT_A_DIFF), [])
        self.assertEqual(harness.parse_diff(""), [])


class TestClassifyPaths(unittest.TestCase):

    def test_splits_by_kind(self):
        entries = harness.parse_diff(MODIFY + CREATE + DELETE)
        mod, new, gone = harness.classify_paths(entries, (".py",))
        self.assertEqual(mod, ["pkg/mod.py"])
        self.assertEqual(new, ["scratch/check_url.py"])
        self.assertEqual(gone, ["pkg/gone.py"])

    def test_deleted_file_is_never_measured(self):
        """Trap 1's mirror image: a deleted file measured before and absent
        after reads as a pure quality improvement."""
        mod, _, gone = harness.classify_paths(
            harness.parse_diff(DELETE), (".py",))
        self.assertEqual(mod, [])
        self.assertEqual(gone, ["pkg/gone.py"])

    def test_rename_is_a_delete_plus_a_create(self):
        mod, new, gone = harness.classify_paths(
            harness.parse_diff(RENAME), (".py",))
        self.assertEqual(mod, [])
        self.assertEqual(new, ["pkg/new.py"])
        self.assertEqual(gone, ["pkg/old.py"])

    def test_extension_filter(self):
        patch = MODIFY.replace("pkg/mod.py", "docs/readme.rst")
        mod, _, _ = harness.classify_paths(harness.parse_diff(patch), (".py",))
        self.assertEqual(mod, [])


class TestChangedLines(unittest.TestCase):

    def test_counts_only_measured_files(self):
        entries = harness.parse_diff(MODIFY + CREATE)
        self.assertEqual(harness.changed_lines(entries, {"pkg/mod.py"}), 4)
        self.assertEqual(harness.changed_lines(entries, set()), 0)

    def test_deletion_lines_attribute_to_the_old_path(self):
        """`+++ /dev/null` used to fall through and be counted as an added line
        against whichever file came before it."""
        entries = harness.parse_diff(MODIFY + DELETE)
        self.assertEqual(harness.changed_lines(entries, {"pkg/mod.py"}), 4)


class TestFunctionDeltas(unittest.TestCase):

    @staticmethod
    def _file(**funcs):
        return {"functions": {name: {"cc": cc} for name, cc in funcs.items()}}

    def test_detects_a_function_getting_more_complex(self):
        before = {"a.py": self._file(f=3, g=20)}
        after = {"a.py": self._file(f=9, g=20)}
        d = harness.function_deltas(before, after)
        self.assertEqual(d["worst_func_cc_increase"], 6)
        self.assertEqual(d["worst_func"], "a.py::f")

    def test_aggregate_max_would_have_missed_it(self):
        """The whole reason function_deltas exists: g dominates the file, so the
        maximum complexity is unchanged while f doubled."""
        before = {"a.py": self._file(f=3, g=20)}
        after = {"a.py": self._file(f=9, g=20)}
        self.assertEqual(harness.aggregate({"a.py": dict(
            self._file(f=9, g=20), max_cc=20, sum_cc=29, n_func=2,
            n_high_risk=0, dead_critical=0, dead_warning=0,
            dead_info=0)})["max_cc"], 20)
        self.assertGreater(
            harness.function_deltas(before, after)["worst_func_cc_increase"], 0)

    def test_new_and_removed_functions(self):
        d = harness.function_deltas({"a.py": self._file(f=3, old=4)},
                                    {"a.py": self._file(f=3, fresh=11)})
        self.assertEqual(d["new_func_max_cc"], 11)
        self.assertEqual(d["n_func_removed"], 1)
        self.assertEqual(d["worst_func_cc_increase"], 0)

    def test_file_missing_from_after(self):
        d = harness.function_deltas({"a.py": self._file(f=3)}, {})
        self.assertEqual(d["n_func_removed"], 1)


class TestAggregate(unittest.TestCase):

    def test_sums_counts_and_maxes_complexity(self):
        def rec(max_cc, sum_cc, n):
            return {"max_cc": max_cc, "sum_cc": sum_cc, "n_func": n,
                    "n_high_risk": 0, "dead_critical": 0, "dead_warning": 0,
                    "dead_info": 0, "functions": {}}
        agg = harness.aggregate({"a.py": rec(9, 12, 3), "b.py": rec(4, 4, 1)})
        self.assertEqual(agg["max_cc"], 9)
        self.assertEqual(agg["sum_cc"], 16)
        self.assertEqual(agg["n_func"], 4)


class TestMcnemar(unittest.TestCase):

    def test_no_discordant_pairs(self):
        self.assertEqual(report.mcnemar_exact(0, 0), 1.0)

    def test_known_values(self):
        # Two-sided sign test: 2 * P(X <= min(b,c)) with X ~ Binomial(b+c, 1/2).
        self.assertAlmostEqual(report.mcnemar_exact(5, 0), 2 / 32)
        self.assertAlmostEqual(report.mcnemar_exact(3, 1), 2 * 5 / 16)
        self.assertAlmostEqual(report.mcnemar_exact(10, 2),
                               2 * (1 + 12 + 66) / 4096)

    def test_symmetric_and_clamped(self):
        self.assertEqual(report.mcnemar_exact(4, 7), report.mcnemar_exact(7, 4))
        self.assertLessEqual(report.mcnemar_exact(8, 8), 1.0)

    def test_more_evidence_lowers_p(self):
        self.assertLess(report.mcnemar_exact(20, 2), report.mcnemar_exact(5, 2))


class TestRegressed(unittest.TestCase):

    @staticmethod
    def _rec(worst=0, **delta):
        d = {"max_cc": 0, "sum_cc": 0, "n_func": 0, "n_high_risk": 0,
             "dead_critical": 0, "dead_warning": 0, "dead_info": 0}
        d.update(delta)
        return {"delta": d, "worst_func_cc_increase": worst}

    def test_per_function_increase_counts(self):
        self.assertTrue(report.regressed(self._rec(worst=1)))

    def test_dead_code_counts(self):
        self.assertTrue(report.regressed(self._rec(dead_warning=1)))
        self.assertTrue(report.regressed(self._rec(dead_critical=1)))

    def test_high_risk_counts(self):
        self.assertTrue(report.regressed(self._rec(n_high_risk=1)))

    def test_clean_patch_does_not(self):
        self.assertFalse(report.regressed(self._rec()))
        self.assertFalse(report.regressed(self._rec(worst=0, max_cc=-3)))

    def test_informational_dead_code_does_not(self):
        self.assertFalse(report.regressed(self._rec(dead_info=4)))


class TestAnalyzerExtract(unittest.TestCase):
    """The pyscn report shape, which mixes PascalCase and snake_case."""

    REPORT = {
        "complexity": {
            "Functions": [
                {"FilePath": "/repo/a.py", "Name": "f",
                 "RiskLevel": "high",
                 "Metrics": {"Complexity": 12, "CognitiveComplexity": 8,
                             "NestingDepth": 3}},
                {"FilePath": "/repo/a.py", "Name": "g",
                 "RiskLevel": "low", "Metrics": {"Complexity": 2}},
                {"FilePath": "/repo/elsewhere.py", "Name": "h",
                 "Metrics": {"Complexity": 40}},
            ],
            "Errors": ["[/repo/b.py] Parse error: unexpected token"],
        },
        "dead_code": {
            "files": [{"file_path": "/repo/a.py",
                       "functions": [{"critical_count": 1,
                                      "warning_count": 2,
                                      "info_count": 3}]}],
        },
    }

    def test_extract_ignores_files_not_requested(self):
        out = analyzer.PyscnAnalyzer._extract(self.REPORT, "/repo",
                                             ["a.py", "b.py"])
        self.assertEqual(sorted(out), ["a.py", "b.py"])
        self.assertEqual(out["a.py"]["n_func"], 2)
        self.assertEqual(out["a.py"]["sum_cc"], 14)
        self.assertEqual(out["a.py"]["max_cc"], 12)
        self.assertEqual(out["a.py"]["n_high_risk"], 1)
        self.assertEqual(out["a.py"]["dead_warning"], 2)

    def test_requested_file_with_no_findings_is_zeroed(self):
        out = analyzer.PyscnAnalyzer._extract(self.REPORT, "/repo", ["b.py"])
        self.assertEqual(out["b.py"]["n_func"], 0)

    def test_empty_metrics_are_not_shared_between_files(self):
        out = analyzer.PyscnAnalyzer._extract({}, "/repo", ["a.py", "b.py"])
        out["a.py"]["functions"]["f"] = {"cc": 1}
        self.assertEqual(out["b.py"]["functions"], {})

    def test_parse_errors_extract_the_path(self):
        errs = analyzer.PyscnAnalyzer._parse_errors(self.REPORT, "/repo")
        self.assertEqual(errs, ["b.py"])

    def test_unrelated_errors_are_not_parse_failures(self):
        data = {"complexity": {"Errors": ["[/repo/b.py] timeout after 600s"]}}
        self.assertEqual(
            analyzer.PyscnAnalyzer._parse_errors(data, "/repo"), [])

    def test_missing_sections(self):
        self.assertEqual(analyzer.PyscnAnalyzer._parse_errors({}, "/repo"), [])
        self.assertEqual(
            analyzer.PyscnAnalyzer._extract({"complexity": None}, "/repo", []),
            {})


class TestOwnParse(unittest.TestCase):
    """The independent syntax check, which is what makes trap 2 not depend on
    pyscn's error message wording."""

    def _write(self, text):
        import tempfile
        fd, path = tempfile.mkstemp(suffix=".py")
        with os.fdopen(fd, "w") as fh:
            fh.write(text)
        self.addCleanup(os.unlink, path)
        return path

    def test_counts_functions(self):
        ok, n = analyzer._own_parse(self._write(
            "def a():\n    pass\n\nclass C:\n    def b(self):\n        pass\n"))
        self.assertTrue(ok)
        self.assertEqual(n, 2)

    def test_async_functions_count(self):
        ok, n = analyzer._own_parse(self._write("async def a():\n    pass\n"))
        self.assertTrue(ok)
        self.assertEqual(n, 1)

    def test_syntax_error_is_reported(self):
        """`return 0 objs` is the real corruption that scored as the largest
        quality improvement in the django run."""
        ok, n = analyzer._own_parse(self._write(
            "def a():\n    return 0 objs\n"))
        self.assertFalse(ok)
        self.assertEqual(n, 0)

    def test_file_with_no_functions_parses_cleanly(self):
        ok, n = analyzer._own_parse(self._write("A = 1\nB = 2\n"))
        self.assertTrue(ok)
        self.assertEqual(n, 0)

    def test_missing_file(self):
        self.assertEqual(analyzer._own_parse("/nonexistent/x.py"), (False, 0))


class TestOpenRecords(unittest.TestCase):

    def test_meta_line_is_not_a_measurement(self):
        import json
        import tempfile
        fd, path = tempfile.mkstemp(suffix=".jsonl")
        with os.fdopen(fd, "w") as fh:
            fh.write(json.dumps({"record_type": "meta",
                                 "analyzer": "pyscn v1.0.0"}) + "\n")
            fh.write(json.dumps({"instance_id": "x", "source": "gold",
                                 "status": "ok"}) + "\n")
        self.addCleanup(os.unlink, path)
        meta, records = report.open_records(path)
        self.assertEqual(meta["analyzer"], "pyscn v1.0.0")
        self.assertEqual(len(records), 1)

    def test_gzip_and_plain_agree(self):
        import gzip
        import json
        import tempfile
        line = json.dumps({"instance_id": "x", "source": "gold",
                           "status": "ok"}) + "\n"
        d = tempfile.mkdtemp()
        plain, packed = os.path.join(d, "a.jsonl"), os.path.join(d, "a.jsonl.gz")
        with open(plain, "w") as fh:
            fh.write(line)
        with gzip.open(packed, "wt") as fh:
            fh.write(line)
        self.assertEqual(report.open_records(plain),
                         report.open_records(packed))


class TestGitRetry(unittest.TestCase):
    """Lock contention costs an instance, so it has to be retried, not raised."""

    def test_lock_errors_are_retryable(self):
        for msg in ["fatal: Unable to create '.git/index.lock': File exists.",
                    "fatal: cannot lock ref 'HEAD'",
                    "Another git process seems to be running"]:
            self.assertTrue(harness._retryable(msg), msg)

    def test_real_failures_are_not_retried(self):
        for msg in ["fatal: reference is not a tree: deadbeef",
                    "error: pathspec 'nope' did not match any file"]:
            self.assertFalse(harness._retryable(msg), msg)

    def test_git_retries_then_succeeds(self):
        calls = []

        class Proc:
            def __init__(self, rc, err):
                self.returncode, self.stderr, self.stdout = rc, err, ""

        def fake_run(argv, **kw):
            calls.append(argv)
            if len(calls) < 3:
                return Proc(128, "fatal: Unable to create index.lock: File exists.")
            return Proc(0, "")

        orig_run, orig_sleep = harness.subprocess.run, harness.time.sleep
        harness.subprocess.run = fake_run
        harness.time.sleep = lambda _: None
        try:
            proc = harness.git("/repo", "checkout", "-f", "abc")
        finally:
            harness.subprocess.run, harness.time.sleep = orig_run, orig_sleep
        self.assertEqual(proc.returncode, 0)
        self.assertEqual(len(calls), 3)

    def test_non_retryable_failure_raises_immediately(self):
        calls = []

        class Proc:
            returncode, stdout = 128, ""
            stderr = "fatal: reference is not a tree: deadbeef"

        def fake_run(argv, **kw):
            calls.append(argv)
            return Proc()

        orig = harness.subprocess.run
        harness.subprocess.run = fake_run
        try:
            with self.assertRaises(RuntimeError):
                harness.git("/repo", "checkout", "-f", "deadbeef")
        finally:
            harness.subprocess.run = orig
        self.assertEqual(len(calls), 1)


class TestOpenAll(unittest.TestCase):
    """Pooling result sets across repositories."""

    def _write(self, meta, records):
        import json
        import tempfile
        fd, path = tempfile.mkstemp(suffix=".jsonl")
        with os.fdopen(fd, "w") as fh:
            fh.write(json.dumps(dict(meta, record_type="meta")) + "\n")
            for r in records:
                fh.write(json.dumps(r) + "\n")
        self.addCleanup(os.unlink, path)
        return path

    @staticmethod
    def _rec(iid):
        return {"instance_id": iid, "source": "gold", "status": "ok"}

    def test_records_pool_and_meta_merges(self):
        a = self._write({"analyzer": "pyscn v1.0.0", "repo": "django/django",
                         "n_instances": 231}, [self._rec("d-1")])
        b = self._write({"analyzer": "pyscn v1.0.0", "repo": "sympy/sympy",
                         "n_instances": 75}, [self._rec("s-1")])
        meta, records = report.open_all([a, b])
        self.assertEqual(len(records), 2)
        self.assertEqual(meta["n_instances"], 306)
        self.assertEqual(meta["repo"], "django/django, sympy/sympy")

    def test_mismatched_analyzer_builds_are_refused(self):
        """Pooling metrics from two analyzer versions would silently mix
        measurement regimes, which is what --min-complexity already taught us."""
        a = self._write({"analyzer": "pyscn v1.0.0"}, [self._rec("d-1")])
        b = self._write({"analyzer": "pyscn v1.1.0"}, [self._rec("s-1")])
        with self.assertRaises(SystemExit) as ctx:
            report.open_all([a, b])
        self.assertIn("different analyzer builds", str(ctx.exception))

    def test_single_path_still_works(self):
        a = self._write({"analyzer": "pyscn v1.0.0", "repo": "django/django"},
                        [self._rec("d-1")])
        meta, records = report.open_all([a])
        self.assertEqual(len(records), 1)
        self.assertEqual(meta["repo"], "django/django")

    def test_empty_result_set_is_refused(self):
        a = self._write({"analyzer": "pyscn v1.0.0"}, [])
        with self.assertRaises(SystemExit):
            report.open_all([a])


class TestLoadResolved(unittest.TestCase):

    def test_gate_comes_from_the_records(self):
        """From a fresh clone `data/` does not exist, so reading the gate out of
        it silently emptied every model's instance set."""
        records = [
            {"instance_id": "i1", "source": "m", "status": "ok",
             "resolved": True},
            {"instance_id": "i2", "source": "m", "status": "ok",
             "resolved": False},
            {"instance_id": "i1", "source": "gold", "status": "ok"},
        ]
        resolved, totals, cost = report.load_resolved(records, "/nonexistent")
        self.assertEqual(resolved["m"], {"i1"})
        self.assertEqual(totals, {})
        self.assertEqual(cost, {})


if __name__ == "__main__":
    unittest.main()

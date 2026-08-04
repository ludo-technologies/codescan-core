# Configuration Reference

Every key jscan accepts, with its type, its default, and whether it currently changes anything.

Keys marked :material-check-circle:{ title="Applied" } **Applied** affect the analysis. Keys marked :material-minus-circle:{ title="Not applied" } **Not applied** are parsed and validated, then ignored. The [configuration guide](index.md#which-keys-take-effect-today) explains why that distinction exists.

All examples use JSON. YAML and TOML files accept the same keys with the same names.

---

## `complexity`

Controls cyclomatic complexity analysis.

### `complexity.low_threshold`

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; integer &nbsp;&middot;&nbsp; default `9`

Upper bound of the low risk band, inclusive. A function at or below this value is reported as low risk.

Must be at least 1.

### `complexity.medium_threshold`

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; integer &nbsp;&middot;&nbsp; default `19`

Upper bound of the medium risk band, inclusive. Functions above it are high risk.

Must be greater than `low_threshold`.

### `complexity.max_complexity`

:material-check-circle: **Applied by `check` only** &nbsp;&middot;&nbsp; integer &nbsp;&middot;&nbsp; default `0`

Supplies the default for `jscan check --max-complexity`. It is used only when the flag is absent from the command line and this value is greater than 0. `jscan analyze` ignores it.

Must be either 0, meaning no limit, or greater than `medium_threshold`.

```json
{
  "complexity": {
    "low_threshold": 10,
    "medium_threshold": 20,
    "max_complexity": 25
  }
}
```

### `complexity.enabled`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; boolean &nbsp;&middot;&nbsp; default `true`

Intended to switch complexity analysis off. No command reads it. Use `--select` to choose analyses instead.

### `complexity.report_unchanged`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; boolean &nbsp;&middot;&nbsp; default `true`

Intended to hide functions whose complexity is exactly 1. No command reads it. Set `output.min_complexity` to 2 for the same effect.

---

## `output`

### `output.min_complexity`

:material-check-circle: **Applied by `analyze` only** &nbsp;&middot;&nbsp; integer &nbsp;&middot;&nbsp; default `1`

Functions below this complexity are excluded from the report. The default of 1 keeps every function, since no function scores lower than 1.

Must be at least 1.

Raising it is the most effective way to shorten the report on a large codebase:

```json
{
  "output": {
    "min_complexity": 5
  }
}
```

When this filter removes functions, the text and JSON output disclose both counts, reported as `12 reported / 340 parsed`, so a filtered report never looks like a complete one.

### `output.format`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; string &nbsp;&middot;&nbsp; default `"text"`

Validated against `text`, `json`, `yaml`, `csv`, and `html`, then ignored. The output format comes from the `--format` flag. An invalid value here still fails the whole run, so keep it to one of the five.

### `output.show_details`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; boolean &nbsp;&middot;&nbsp; default `false`

### `output.sort_by`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; string &nbsp;&middot;&nbsp; default `"complexity"`

Validated against `name`, `complexity`, and `risk`, then ignored. Reports are always sorted by complexity, descending.

### `output.directory`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; string &nbsp;&middot;&nbsp; default `""`

Use `jscan analyze --output <path>` to choose where the HTML report goes.

---

## `analysis`

### `analysis.exclude_patterns`

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; array of strings

Directory names and filename patterns to skip. A value here **replaces** the default list rather than extending it.

The default is:

```json
{
  "analysis": {
    "exclude_patterns": [
      "node_modules",
      "bower_components",
      "jspm_packages",
      "vendor",
      "assets",
      "overrides",
      "third_party",
      "third-party",
      "extern",
      "external",
      "dist",
      "build",
      "out",
      ".output",
      ".next",
      ".nuxt",
      ".vercel",
      ".cache",
      ".turbo",
      "coverage",
      ".git",
      "*.min.js",
      "*.min.mjs",
      "*.min.cjs",
      "*.bundle.js",
      "*.map"
    ]
  }
}
```

Matching happens in two places, with different rules.

A **directory** is skipped when its own name equals a pattern exactly, or matches it as a glob. This behaves the way you would expect: `dist` skips any directory named `dist` at any depth, and nothing else.

A **file** is skipped when its filename matches a pattern as a glob, **or when the pattern appears anywhere in the file's full path as a plain substring**. That second rule is far broader than it looks, and it is the cause of the problem described next.

!!! danger "Short patterns silently exclude files you meant to analyze"

    Because file matching falls back to a substring test on the whole path, a short pattern in the list removes every file whose path merely contains those characters. Two entries in the default list cause this in ordinary projects:

    - `out` matches `src/layout/Header.tsx`, `src/checkout/Cart.ts`, and `src/routes/api.ts`, because `layout`, `checkout`, and `routes` all contain the letters `out`.
    - `dist` matches `src/utils/distance.ts` and anything else containing `dist`.

    The files are dropped during collection, so nothing in the output mentions them. The only visible symptom is that the `Analyzing N files...` count is lower than you expect.

    Verify what jscan actually read before trusting a clean report:

    ```console
    $ jscan analyze --text src/ | head -1
    Analyzing 1 files...
    ```

    If that count looks wrong, override `exclude_patterns` with a list that leaves out the short entries:

    ```json
    {
      "analysis": {
        "exclude_patterns": [
          "node_modules",
          "bower_components",
          "jspm_packages",
          "vendor",
          "third_party",
          "coverage",
          ".git",
          ".next",
          ".nuxt",
          ".turbo",
          ".cache",
          "*.min.js",
          "*.min.mjs",
          "*.min.cjs",
          "*.bundle.js",
          "*.map"
        ]
      }
    }
    ```

    This list drops `out`, `dist`, `build`, `assets`, `extern`, `external`, and `overrides`, which are the entries most likely to over-match. Dropping them has a cost: a directory named `dist` or `build` inside the analyzed path will now be analyzed, because the same list drives both the directory rule and the file rule. Point jscan at your source directory rather than the project root to avoid that:

    ```bash
    jscan analyze src/
    ```

Because your list replaces the default, start from one of the lists above and append to it rather than writing a short one from scratch. Omitting `node_modules` in particular will make jscan analyze your entire dependency tree.

### `analysis.include_patterns`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; array of strings

The analyzed extensions are fixed at `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.mts`, and `.cts`. This key is validated only to the extent that it must not be an empty array, which means you cannot delete it from a file that already has one.

### `analysis.recursive`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; boolean &nbsp;&middot;&nbsp; default `true`

Directory walks are always recursive.

### `analysis.follow_symlinks`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; boolean &nbsp;&middot;&nbsp; default `false`

Symbolic links are never followed.

---

## `dead_code`

Every key in this group is :material-minus-circle: **not applied**. Dead code detection currently runs with a fixed configuration: it reports at info severity and above, sorts by severity, and enables all six unreachable-code checks.

The keys are still validated, so an invalid value fails the run.

| Key | Type | Default | Validation |
| --- | --- | --- | --- |
| `dead_code.enabled` | boolean | `true` | |
| `dead_code.min_severity` | string | `"warning"` | One of `critical`, `warning`, `info` |
| `dead_code.show_context` | boolean | `false` | |
| `dead_code.context_lines` | integer | `3` | From 0 to 20 |
| `dead_code.sort_by` | string | `"severity"` | One of `severity`, `line`, `file`, `function` |
| `dead_code.detect_after_return` | boolean | `true` | |
| `dead_code.detect_after_break` | boolean | `true` | |
| `dead_code.detect_after_continue` | boolean | `true` | |
| `dead_code.detect_after_throw` | boolean | `true` | |
| `dead_code.detect_unreachable_branches` | boolean | `true` | |
| `dead_code.ignore_patterns` | array of strings | `[]` | |

To exclude dead code from a gate, use `jscan check --allow-dead-code` rather than `dead_code.enabled`.

---

## `clones`

:material-minus-circle: **Not applied.** Clone detection runs with the built-in defaults, which are:

| Setting | Default |
| --- | --- |
| Minimum fragment size | 10 lines and 20 syntax tree nodes |
| Enabled clone types | Type 1, Type 2, and Type 4 |
| Type 1 similarity threshold | 0.85 |
| Type 2 similarity threshold | 0.75 |
| Type 3 similarity threshold | 0.70 |
| Type 4 similarity threshold | 0.65 |
| Grouping strategy | Connected components |
| Locality-sensitive hashing | Enabled automatically above 500 fragments |

Type 3 is excluded from the enabled set because near-miss matches produce too many false positives for everyday use.

---

## Reserved groups

These four groups exist in the schema for planned features. All of them default to disabled and none of them is read by any command today.

| Group | Intended purpose |
| --- | --- |
| `system_analysis` | Combining the individual analyses into one system-level view |
| `dependencies` | Filtering and reporting options for dependency analysis |
| `architecture` | Layer definitions and rules for validating architectural boundaries |
| `module_analysis` | Import resolution options, including path alias handling |

Values inside them are unmarshalled without validation. Writing them today is harmless and has no effect.

---

## Environment variables

| Variable | Effect |
| --- | --- |
| `JSCAN_CONFIG` | Path to a configuration file, consulted late in the search order |
| `PYSCN_CONFIG` | Same, kept for backward compatibility |
| `XDG_CONFIG_HOME` | Changes where jscan looks for a user-level `jscan/` config directory |

The two config variables are checked only after every directory in the search has been tried, so a `jscan.config.json` anywhere above your source will take priority over them. See [config discovery](index.md#how-jscan-finds-your-config-file) for the full order.

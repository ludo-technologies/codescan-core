# Configuration Reference

Every key jscan accepts, with its type, its default, and whether it currently changes anything.

Keys marked :material-check-circle:{ title="Applied" } **Applied** affect the analysis. Keys marked :material-minus-circle:{ title="Not applied" } **Not applied** are parsed and validated, then ignored, and jscan warns on stderr when your file sets one. The [configuration guide](index.md#which-keys-take-effect-today) explains why that distinction exists.

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

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; boolean &nbsp;&middot;&nbsp; default `true`

Set it to `false` to leave out functions whose complexity is exactly 1, which on most codebases is the majority of them.

This differs from setting `output.min_complexity` to 2 in what the counts say afterwards. A function dropped by `min_complexity` is still counted as parsed, so the report shows `12 reported / 340 parsed`; a function dropped by `report_unchanged` is not counted at all, because it was never meant to be part of the report.

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

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; string &nbsp;&middot;&nbsp; default `"complexity"`

Order of the functions in the complexity report. One of:

| Value | Order |
| --- | --- |
| `complexity` | Complexity, highest first |
| `name` | Function name, alphabetical |
| `risk` | Risk band, high risk first |

Functions the criterion cannot separate are ordered by source location, so the report is stable across runs. The text report names the criterion in its `Functions (sorted by ...)` heading.

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

A pattern is matched against whole names, never against part of one.

A pattern **without a slash** is compared to the file's own name and to each directory name above it. `dist` skips every directory named `dist` at any depth along with everything inside it, and it leaves `src/utils/distance.ts` alone, because no name in that path is exactly `dist`. Glob characters apply to a single name, so `*.min.js` matches file names and `__*__` matches a directory named `__tests__`.

A pattern **containing a slash** is compared to the path itself, where `**` stands for any number of directory levels. `src/generated` skips that directory and everything under it. `**/dist/**` skips every file below any directory named `dist`.

Patterns are matched relative to the path you pass to jscan, so the directories above it are never considered. A project stored at `/home/me/build/myapp` is analyzed normally even though `build` is on the default list. A file you name directly on the command line is matched on its own name alone, so `jscan analyze src/dist/bundle.js` analyzes that file.

!!! note "Behavior changed in the release after 0.9.0"

    Earlier versions also skipped a file when a pattern appeared anywhere in its path as a plain substring. The default entries `out` and `dist` therefore removed `src/routes/api.ts`, `src/layout/Header.tsx`, `src/checkout/Cart.ts`, and `src/utils/distance.ts` without reporting anything. If you worked around that by trimming the short entries out of your `exclude_patterns`, you can now go back to the default list.

Because your list replaces the default, start from the list above and append to it rather than writing a short one from scratch. Omitting `node_modules` in particular will make jscan analyze your entire dependency tree.

### `analysis.include_patterns`

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; array of strings

Which files to analyze, of those jscan can parse. A file is analyzed when it matches at least one pattern here and no pattern in `exclude_patterns`.

The default is every extension jscan understands:

```json
{
  "analysis": {
    "include_patterns": [
      "**/*.js",
      "**/*.ts",
      "**/*.jsx",
      "**/*.tsx",
      "**/*.mjs",
      "**/*.cjs",
      "**/*.mts",
      "**/*.cts"
    ]
  }
}
```

Patterns use the same matching rules as [`exclude_patterns`](#analysisexclude_patterns), including the part that catches people out: they are matched relative to the path you pass on the command line, so `src/**/*.ts` matches nothing when you run `jscan analyze src/`. Prefer a leading `**/` unless you mean to depend on where the command is run from.

This key cannot widen the analysis. The analyzed extensions are fixed at `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.mts`, and `.cts`, so adding `**/*.vue` changes nothing. It also must not be an empty array, which would select no files at all.

A file you name directly on the command line is analyzed whether or not it matches, on the grounds that naming it is a clearer statement of intent than the config file is.

### `analysis.recursive`

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; boolean &nbsp;&middot;&nbsp; default `true`

Set it to `false` to analyze only the files directly inside each directory you pass, without descending into subdirectories. Files named directly on the command line are unaffected.

### `analysis.follow_symlinks`

:material-minus-circle: **Not applied** &nbsp;&middot;&nbsp; boolean &nbsp;&middot;&nbsp; default `false`

Symbolic links are never followed.

---

## `dead_code`

Two keys in this group reach the analysis. The rest are validated and ignored: every unreachable-code check always runs, context lines are never shown, and nothing is ignored.

### `dead_code.min_severity`

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; string &nbsp;&middot;&nbsp; default `"info"`

Findings below this severity are dropped before anything is reported or counted. One of `critical`, `warning`, or `info`.

The default of `info` keeps every finding, which is what the health score is calibrated against. Raising it to `warning` also raises the score, so compare scores only between runs that used the same floor.

Raising it changes `jscan check` as well: a run whose only findings fall below the floor passes.

### `dead_code.sort_by`

:material-check-circle: **Applied** &nbsp;&middot;&nbsp; string &nbsp;&middot;&nbsp; default `"severity"`

Order of the files in the dead code report. One of `severity`, `line`, `file`, or `function`. Files that the criterion cannot separate are ordered by path, so the report is stable across runs.

### Not applied

| Key | Type | Default | Validation |
| --- | --- | --- | --- |
| `dead_code.enabled` | boolean | `true` | |
| `dead_code.show_context` | boolean | `false` | |
| `dead_code.context_lines` | integer | `3` | From 0 to 20 |
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

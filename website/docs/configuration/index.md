# Configuration

jscan runs without any configuration at all. A configuration file is for the cases where the defaults do not suit your project, most often because you want different complexity thresholds or a different set of skipped directories.

Create one with [`jscan init`](../cli/init.md), or write it by hand.

## How jscan finds your config file

When you pass `--config`, jscan loads exactly that file and fails if it cannot be read. When you do not, jscan searches, and the first file it finds wins.

The search runs in this order:

1. **Upward from the analyzed path.** Starting at the directory you asked jscan to analyze, it checks that directory, then its parent, and so on to the filesystem root. If you passed a file rather than a directory, the search starts from the file's directory.
2. **The current working directory.**
3. **`$XDG_CONFIG_HOME/jscan/`**, then `$XDG_CONFIG_HOME/pyscn/`, if that variable is set.
4. **`~/.config/jscan/`**, then `~/.config/pyscn/`.
5. **Your home directory.**
6. **The path in `$JSCAN_CONFIG`**, then the path in `$PYSCN_CONFIG`, if either is set and points at a file that exists.

Searching upward from the target rather than from the current directory means that analyzing `packages/api/src` from the repository root still picks up `packages/api/jscan.config.json`.

### Accepted filenames

Within each directory, jscan checks these names in order:

```text
jscan.config.json
.jscanrc.json
jscan.yaml
jscan.yml
.jscan.toml
.jscan.yml
jscan.json
.jscan.json
```

It then checks the equivalent `pyscn` names, which are accepted for backward compatibility from when jscan shared its configuration loader with pyscn. Prefer a `jscan` name in new projects.

### Supported file formats

The loader reads JSON, YAML, and TOML. The format is chosen from the file extension, so `.jscanrc.json` must contain JSON and `jscan.yaml` must contain YAML. JSON is the most common choice because it needs no extra tooling in a JavaScript project.

```bash
# Confirm which file was used
jscan analyze --config ./jscan.config.json src/
```

Passing `--config` to `analyze` makes it print `Using config: <path>` before the results, which is the quickest way to confirm that a file is being read at all. The `check` and `deps` commands accept `--config` but print no such line.

## Which keys take effect today

This is the part worth reading carefully. jscan validates the whole configuration schema, but the commands act on only part of it. Setting a key from the second table below is accepted, and validated, and then ignored.

You do not have to memorize the split. Any command that loads a file naming such a key prints a warning to stderr:

```console
$ jscan analyze src/
Warning: /work/app/jscan.config.json sets 2 keys that no command reads: dead_code.context_lines, output.format
  See https://jscan.codescan.dev/configuration/#which-keys-take-effect-today
```

The same warning catches misspelled keys, since a key jscan does not recognize is by definition one that no command reads. A file written by [`jscan init`](../cli/init.md) never triggers it: the generated file contains only keys that work.

### Keys that change behavior

| Key | Affects | What it does |
| --- | --- | --- |
| `complexity.low_threshold` | `analyze`, `check` | Upper bound of the low risk band |
| `complexity.medium_threshold` | `analyze`, `check` | Upper bound of the medium risk band |
| `complexity.max_complexity` | `check` | Default for `--max-complexity`, used only when the flag is absent and the value is above 0 |
| `complexity.report_unchanged` | `analyze`, `check` | Whether functions with complexity 1 are reported at all |
| `dead_code.min_severity` | `analyze`, `check` | Findings below this severity are dropped |
| `dead_code.sort_by` | `analyze` | Order of the files in the dead code report |
| `output.min_complexity` | `analyze` | Functions below this complexity are left out of the report |
| `output.sort_by` | `analyze` | Order of the functions in the complexity report |
| `analysis.include_patterns` | `analyze`, `check`, `deps` | Which files to analyze, of those jscan can parse |
| `analysis.exclude_patterns` | `analyze`, `check`, `deps` | Directories and filename patterns to skip |
| `analysis.recursive` | `analyze`, `check`, `deps` | Whether a directory is walked to its leaves or only at its top level |

### Keys that are parsed but not yet applied

| Key group | Status |
| --- | --- |
| `dead_code.show_context`, `dead_code.context_lines` | Context lines are never shown. |
| `dead_code.detect_*`, `dead_code.ignore_patterns` | All unreachable-code checks always run, and nothing is ignored. |
| `dead_code.enabled`, `complexity.enabled` | Use `--select` to choose which analyses run. |
| `clones.*` | Clone detection runs with the built-in defaults. |
| `output.format` | The format comes from the `--format` flag only. |
| `output.show_details`, `output.directory` | Not read by any command. |
| `analysis.follow_symlinks` | Symbolic links are never followed. |
| `system_analysis.*`, `dependencies.*`, `architecture.*`, `module_analysis.*` | Reserved for features that are not yet implemented. All default to disabled. |

This is documented rather than hidden because a configuration key that quietly does nothing is worse than one that does not exist. If a setting you need is in the second table, use the equivalent command line flag where one exists, and otherwise track the gap in the [issue tracker](https://github.com/ludo-technologies/polyscan/issues).

### Narrowing what gets analyzed

`analysis.include_patterns` selects from the files jscan can parse; it cannot add file types, because the analyzed extensions are fixed at `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.mts`, and `.cts`. Adding `**/*.vue` to the list changes nothing.

Patterns are matched the same way `exclude_patterns` are, and relative to the path you pass on the command line. Analyzing `src/` with an include pattern of `src/**/*.ts` therefore matches nothing, because the paths being matched start below `src`:

```json
{
  "analysis": {
    "include_patterns": ["**/*.ts", "**/*.tsx"]
  }
}
```

A file you name directly is analyzed whether or not it matches, so `jscan analyze src/legacy.js` still works under a TypeScript-only include list.

## jscan also reads your `.gitignore`

Before applying `exclude_patterns`, jscan looks for a `.gitignore` file in the directory you asked it to analyze, and skips anything that file ignores. This is usually what you want, since build output and local artifacts are normally ignored by git as well.

Two details are worth knowing:

- Only the `.gitignore` at the root of the analyzed path is read. Running `jscan analyze src/` uses `src/.gitignore` and does **not** read the repository's top-level `.gitignore`. Running `jscan analyze .` from the repository root does read it.
- Global and nested gitignore files are not consulted, and neither is `.git/info/exclude`.

If a file you expected in the report is missing, check both your `.gitignore` and the `exclude_patterns` behavior described in the [reference](reference.md#analysisexclude_patterns).

## A minimal useful file

Most projects need only this much:

```json
{
  "complexity": {
    "low_threshold": 10,
    "medium_threshold": 20
  },
  "analysis": {
    "exclude_patterns": [
      "node_modules",
      "dist",
      "build",
      ".next",
      "coverage",
      "*.min.js",
      "**/*.generated.ts"
    ]
  }
}
```

!!! warning "Writing `exclude_patterns` replaces the default list"

    The value you provide is not merged with the built-in defaults. It replaces them. The default list is long and covers dependency directories, build outputs, framework caches, and minified files, so a short custom list will make jscan analyze things you probably did not intend to analyze, such as `dist`. Copy the [full default list](reference.md#analysisexclude_patterns) as your starting point and add to it.

## Validation

The configuration is validated on load, and an invalid file stops the command with a message naming the offending key:

```console
$ jscan analyze --config bad.json src/
Error: failed to load configuration: invalid configuration: complexity.medium_threshold (5) must be > low_threshold (10)
```

The rules enforced are listed with each key in the [reference](reference.md).

## Next

- [Configuration reference](reference.md) documents every key, its type, and its default.
- [Configuration examples](examples.md) has complete files for several kinds of project.

# Configuration

polyscan runs without any configuration at all. A configuration file tunes the **JavaScript/TypeScript analysis** — most often because you want different complexity thresholds or a different set of skipped directories. There is no configuration file for the Go, Rust and C++ analysis yet; those files are collected by extension and analyzed with the built-in defaults.

The file format is the one carried over from jscan, the JavaScript/TypeScript analyzer that merged into polyscan, which is why the conventional filename is `jscan.config.json`. An existing file keeps working unchanged. Write one by hand, or copy a [complete example](examples.md).

## How polyscan finds your config file

polyscan searches for a configuration file automatically, and the first file it finds wins. There is no `--config` flag.

The search runs in this order:

1. **Upward from the analyzed path.** Starting at the first directory you asked polyscan to analyze, it checks that directory, then its parent, and so on to the filesystem root. If you passed a file rather than a directory, the search starts from the file's directory.
2. **The current working directory.**
3. **`$XDG_CONFIG_HOME/jscan/`**, then `$XDG_CONFIG_HOME/pyscn/`, if that variable is set.
4. **`~/.config/jscan/`**, then `~/.config/pyscn/`.
5. **Your home directory.**
6. **The path in `$JSCAN_CONFIG`**, then the path in `$PYSCN_CONFIG`, if either is set and points at a file that exists.

Searching upward from the target rather than from the current directory means that analyzing `packages/api/src` from the repository root still picks up `packages/api/jscan.config.json`.

### Accepted filenames

Within each directory, polyscan checks these names in order:

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

## Which keys take effect today

This is the part worth reading carefully. polyscan validates the whole configuration schema, but acts on only part of it. Setting a key from the second table below is accepted, and validated, and then ignored.

You do not have to memorize the split. A run that loads a file naming such a key prints a warning to stderr:

```console
$ polyscan analyze src/
Warning: /work/app/jscan.config.json sets 2 keys that no command reads: dead_code.context_lines, output.format
  See https://polyscan.codescan.dev/configuration/#which-keys-take-effect-today
```

The same warning catches misspelled keys, since a key polyscan does not recognize is by definition one that no command reads.

### Keys that change behavior

All of these affect the JavaScript/TypeScript analysis inside `polyscan analyze`.

| Key | What it does |
| --- | --- |
| `complexity.low_threshold` | Upper bound of the low risk band |
| `complexity.medium_threshold` | Upper bound of the medium risk band |
| `complexity.report_unchanged` | Whether functions with complexity 1 are reported at all |
| `dead_code.min_severity` | Findings below this severity are dropped |
| `dead_code.sort_by` | Order of the files in the dead code report |
| `output.min_complexity` | Functions below this complexity are left out of the report |
| `output.sort_by` | Order of the functions in the complexity report |
| `analysis.include_patterns` | Which files to analyze, of those the JavaScript/TypeScript analysis can parse |
| `analysis.exclude_patterns` | Directories and filename patterns to skip |
| `analysis.recursive` | Whether a directory is walked to its leaves or only at its top level |

### Keys that are parsed but not yet applied

| Key group | Status |
| --- | --- |
| `complexity.max_complexity` | Was read by jscan's retired `check` command. Accepted without a warning, but nothing reads it now |
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

`analysis.include_patterns` selects from the files the JavaScript/TypeScript analysis can parse; it cannot add file types, because the analyzed extensions are fixed at `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts`, `.tsx`, `.mts`, and `.cts`. Adding `**/*.vue` to the list changes nothing. Go, Rust and C++ files are collected independently of these patterns.

Patterns are matched the same way `exclude_patterns` are, and relative to the path you pass on the command line. Analyzing `src/` with an include pattern of `src/**/*.ts` therefore matches nothing, because the paths being matched start below `src`:

```json
{
  "analysis": {
    "include_patterns": ["**/*.ts", "**/*.tsx"]
  }
}
```

A file you name directly is analyzed whether or not it matches, so `polyscan analyze src/legacy.js` still works under a TypeScript-only include list.

## polyscan also reads your `.gitignore`

Before applying `exclude_patterns` to the JavaScript/TypeScript files, polyscan looks for a `.gitignore` file in the directory you asked it to analyze, and skips anything that file ignores. This is usually what you want, since build output and local artifacts are normally ignored by git as well.

Two details are worth knowing:

- Only the `.gitignore` at the root of the analyzed path is read. Running `polyscan analyze src/` uses `src/.gitignore` and does **not** read the repository's top-level `.gitignore`. Running `polyscan analyze .` from the repository root does read it.
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

    The value you provide is not merged with the built-in defaults. It replaces them. The default list is long and covers dependency directories, build outputs, framework caches, and minified files, so a short custom list will make polyscan analyze things you probably did not intend to analyze, such as `dist`. Copy the [full default list](reference.md#analysisexclude_patterns) as your starting point and add to it.

## Validation

The configuration is validated on load, and an invalid file stops the command with a message naming the offending key:

```console
$ polyscan analyze src/
Error: failed to load the JavaScript configuration: invalid configuration: complexity.medium_threshold (5) must be > low_threshold (10)
```

The rules enforced are listed with each key in the [reference](reference.md).

## Next

- [Configuration reference](reference.md) documents every key, its type, and its default.
- [Configuration examples](examples.md) has complete files for several kinds of project.

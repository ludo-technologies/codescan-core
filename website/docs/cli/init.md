# jscan init

Writes a configuration file so you do not have to repeat flags on every run.

```bash
jscan init
```

By default this creates `jscan.config.json` in the current directory. The command refuses to overwrite an existing file unless you pass `--force`.

## Synopsis

```bash
jscan init                        # Write jscan.config.json here
jscan init --config custom.json   # Write to another path
jscan init --force                # Overwrite an existing file
jscan init --minimal              # Write only the essential keys
jscan init --interactive          # Answer a few questions first
```

## Flags

| Flag | Short | Default | Description |
| --- | --- | --- | --- |
| `--config` | `-c` | `jscan.config.json` | Path of the file to write |
| `--force` | `-f` | `false` | Overwrite the file if it already exists |
| `--minimal` | | `false` | Write a shorter file with essential keys only |
| `--interactive` | `-i` | `false` | Ask about the project type and strictness first |

The parent directory must already exist. `jscan init --config config/jscan.json` fails if there is no `config` directory.

## Interactive setup

`--interactive` asks two questions and then writes a file tuned to the answers.

The first question is the project type, which decides the include and exclude patterns:

| Choice | Adds to the excluded paths | Notable include patterns |
| --- | --- | --- |
| Generic JavaScript/TypeScript | `node_modules`, `dist`, `build` | `.js`, `.ts`, `.jsx`, `.tsx` |
| React or Next.js | also `.next` and `coverage` | same as generic |
| Vue or Nuxt | also `.nuxt` and `coverage` | same as generic, plus `.vue` |
| Node.js backend | also `test`, `tests`, `__tests__` | `.js`, `.ts`, `.mjs`, `.cjs` |

The second question is how strict the complexity thresholds should be:

| Choice | `low_threshold` | `medium_threshold` | `max_complexity` |
| --- | --- | --- | --- |
| Relaxed | 15 | 30 | 0, meaning no limit |
| Standard | 10 | 20 | 0, meaning no limit |
| Strict | 5 | 10 | 15 |

Finally it asks where to write the file, offering the current directory.

!!! note "The Vue preset lists a pattern jscan cannot use yet"

    Choosing Vue or Nuxt adds `**/*.vue` to `analysis.include_patterns`. jscan does not parse single-file components today, and it does not read `include_patterns` at all, so the entry has no effect. Vue support is on the roadmap. The `.js` and `.ts` files in a Vue project are analyzed normally.

## The generated file

Without `--interactive`, `jscan init` writes the generic project preset at standard strictness:

```json
{
  "complexity": {
    "enabled": true,
    "low_threshold": 10,
    "medium_threshold": 20,
    "max_complexity": 0,
    "report_unchanged": false
  },
  "dead_code": {
    "enabled": true,
    "min_severity": "warning",
    "show_context": false,
    "context_lines": 3,
    "sort_by": "severity",
    "detect_after_return": true,
    "detect_after_break": true,
    "detect_after_continue": true,
    "detect_after_throw": true,
    "detect_unreachable_branches": true,
    "ignore_patterns": []
  },
  "output": {
    "format": "text",
    "show_details": true,
    "sort_by": "complexity",
    "min_complexity": 1
  },
  "analysis": {
    "include_patterns": [
      "**/*.js",
      "**/*.ts",
      "**/*.jsx",
      "**/*.tsx"
    ],
    "exclude_patterns": [
      "node_modules",
      "dist",
      "build",
      "*.min.js",
      "*.bundle.js"
    ],
    "recursive": true,
    "follow_symlinks": false
  }
}
```

`--minimal` writes a much shorter file with the complexity thresholds, the dead code severity floor, and the file patterns.

!!! warning "The generated file is wider than what jscan reads"

    Both templates include keys that jscan parses and validates but does not yet act on, such as everything under `dead_code`. The [configuration guide](../configuration/index.md#which-keys-take-effect-today) lists exactly which keys change behavior. Nothing in the generated file is invalid, but do not assume that editing `dead_code.min_severity` changes the output.

    In particular, note that the generated `exclude_patterns` is **shorter** than the built-in default. Writing a configuration file therefore narrows what jscan skips. If you do not need custom patterns, delete the key and let the default apply.

## Which filenames jscan looks for

You do not have to use `jscan.config.json`. jscan searches for several names, and `.jscanrc.json` is the common alternative. The [configuration guide](../configuration/index.md#how-jscan-finds-your-config-file) lists the full set and the directories searched.

## See also

- [Configuration reference](../configuration/reference.md) for every key
- [Configuration examples](../configuration/examples.md) for files tuned to particular project shapes

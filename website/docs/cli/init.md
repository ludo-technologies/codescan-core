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

    Choosing Vue or Nuxt adds `**/*.vue` to `analysis.include_patterns`. Include patterns select from the file types jscan can parse rather than adding new ones, and jscan does not parse single-file components today, so the entry has no effect. Vue support is on the roadmap. The `.js` and `.ts` files in a Vue project are analyzed normally.

## The generated file

Without `--interactive`, `jscan init` writes the generic project preset at standard strictness:

```json
{
  "complexity": {
    "low_threshold": 10,
    "medium_threshold": 20,
    "max_complexity": 0,
    "report_unchanged": false
  },
  "dead_code": {
    "min_severity": "info",
    "sort_by": "severity"
  },
  "output": {
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
    "recursive": true
  }
}
```

`--minimal` writes a much shorter file with the complexity thresholds, the dead code severity floor, and the file patterns.

Every key here changes behavior. The templates deliberately leave out the keys jscan parses but does not act on, so that nothing in a generated file is a promise it cannot keep; the [configuration guide](../configuration/index.md#which-keys-take-effect-today) lists those keys, and jscan warns if your file sets one.

!!! warning "The generated `exclude_patterns` is shorter than the default"

    Writing a configuration file therefore narrows what jscan skips, since your list replaces the built-in one rather than extending it. If you do not need custom patterns, delete the key and let the default apply.

## Which filenames jscan looks for

You do not have to use `jscan.config.json`. jscan searches for several names, and `.jscanrc.json` is the common alternative. The [configuration guide](../configuration/index.md#how-jscan-finds-your-config-file) lists the full set and the directories searched.

## See also

- [Configuration reference](../configuration/reference.md) for every key
- [Configuration examples](../configuration/examples.md) for files tuned to particular project shapes

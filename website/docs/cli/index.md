# CLI Reference

jscan exposes five commands. Every command accepts one or more paths, and every path may be a file or a directory.

| Command | Purpose | Fails the build? |
| --- | --- | --- |
| [`analyze`](analyze.md) | Run the full analysis and produce a report | No |
| [`check`](check.md) | Enforce quality thresholds for continuous integration | Yes, exit code 1 or 2 |
| [`deps`](deps.md) | Inspect and export the module dependency graph | No |
| [`init`](init.md) | Write a configuration file | No |
| [`version`](version.md) | Print version information | No |

```console
$ jscan --help
jscan is a high-performance static analyzer for JavaScript and TypeScript code.
It provides complexity analysis, dead code detection, and more.

Usage:
  jscan [command]

Available Commands:
  analyze     Analyze JavaScript/TypeScript files
  check       Fast quality check for CI/CD pipelines
  completion  Generate the autocompletion script for the specified shell
  deps        Analyze and visualize module dependencies
  help        Help about any command
  init        Generate a jscan configuration file
  version     Print version information

Flags:
  -h, --help      help for jscan
  -v, --version   version for jscan

Use "jscan [command] --help" for more information about a command.
```

## Which files jscan reads

Every command that takes a path walks the directory tree and collects files with these extensions:

`.js` &nbsp; `.jsx` &nbsp; `.mjs` &nbsp; `.cjs` &nbsp; `.ts` &nbsp; `.tsx` &nbsp; `.mts` &nbsp; `.cts`

The walk always recurses into subdirectories and never follows symbolic links. Directories and filename patterns listed under `analysis.exclude_patterns` in the configuration file are skipped. That list defaults to dependency directories such as `node_modules`, build outputs such as `dist` and `.next`, cache directories, and minified or bundled files. The full default list appears in the [configuration reference](../configuration/reference.md#analysisexclude_patterns).

If no matching file is found, the command stops with the error `no JavaScript/TypeScript files found`.

## Shell completion

Cobra, the command line framework jscan is built on, generates completion scripts:

```bash
# bash
jscan completion bash > /etc/bash_completion.d/jscan

# zsh
jscan completion zsh > "${fpath[1]}/_jscan"

# fish
jscan completion fish > ~/.config/fish/completions/jscan.fish
```

Run `jscan completion --help` for the PowerShell instructions and for notes on loading the script for the current session only.

## Configuration file discovery

Commands other than `version` look for a configuration file before they run. When you do not pass `--config`, jscan searches upward from the analyzed path toward the filesystem root, then falls back to several well-known locations. The [configuration guide](../configuration/index.md#how-jscan-finds-your-config-file) documents the search order and the accepted filenames.

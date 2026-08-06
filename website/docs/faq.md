# FAQ

## Usage

### Why did jscan analyze far fewer files than my project has?

Check your exclude patterns first. A pattern in `analysis.exclude_patterns` matches a whole file or directory name, so `dist` skips a directory named `dist` and leaves `src/utils/distance.ts` alone. A pattern that names a directory you did not mean to exclude removes every file under it, and nothing in the output names the missing files. Compare the `Analyzing N files...` line against a `find` count to confirm. The [configuration reference](configuration/reference.md#analysisexclude_patterns) describes the matching rules.

Versions up to 0.9.0 also matched a pattern against any part of a file's path, so the default entries `out` and `dist` silently dropped `src/routes/`, `src/layout/`, `src/checkout/`, and `src/utils/distance.ts`. Upgrade if you see that.

A second possibility is your `.gitignore`. jscan reads the one at the root of the analyzed directory and skips whatever it ignores.

### Why is every one of my exports reported as unused?

The unused-export check compares imports against exports across the files in the current run only. It has no way to see importers outside that set.

This happens in two situations. Analyzing a subdirectory, such as `src/components/`, means the importers in the rest of `src/` were never read. Pass the whole source root instead. Analyzing a library means the importers are in other repositories entirely, and there is nothing to do about it except run `jscan check --allow-dead-code`.

### Why does `jscan check` fail on a codebase that looks fine?

The default gate fails on any dead code finding, at any severity. The `unused_exported_function` warning described above is usually the cause. Start with:

```bash
jscan check --allow-dead-code --max-complexity 20 src/
```

### Does jscan need my project to compile?

No. It parses the source with tree-sitter and never invokes `tsc`. It works on code that currently fails type checking, and it does not read `tsconfig.json`.

### Does jscan read `tsconfig.json` path aliases?

No. An import written `@/lib/util` becomes a module node of its own rather than resolving to `src/lib/util.ts`. This inflates module counts in `jscan deps`, drops real edges, and hides cycles that pass through an alias. Relative imports resolve correctly. See [the TypeScript guide](guides/typescript-projects.md#path-aliases-are-not-resolved).

### Does jscan support Vue single-file components?

Not yet. `.vue` files are not collected, so the `<script>` block inside them is never analyzed. The `.ts` and `.js` files in a Vue or Nuxt project are analyzed normally.

### Can I run jscan on a single file?

Yes. Every command accepts files as well as directories:

```bash
jscan check --select complexity src/server/router.ts
```

Bear in mind that cross-file analyses cannot say anything useful about one file. Dead code detection will report its exports as unused, and the dependency graph will have one node.

### Why did my score go up when I changed nothing?

Check whether the `--select` value changed. A category that did not run contributes no penalty, so a narrower selection produces a higher score for identical code. Always use the same selection when tracking the score over time. See [the health score page](output/health-score.md#selecting-fewer-analyses-raises-the-score).

## Configuration

### I set a value in my config file and nothing changed. Why?

jscan validates the whole configuration schema but acts on only part of it. These keys change behavior today:

- `complexity.low_threshold` and `complexity.medium_threshold`
- `complexity.max_complexity`, used by `jscan check` only
- `output.min_complexity`, used by `jscan analyze` only
- `analysis.exclude_patterns`

Everything else, including all of `dead_code`, is parsed and then ignored. The [configuration guide](configuration/index.md#which-keys-take-effect-today) has the full list of both kinds.

### Why does adding `exclude_patterns` make jscan analyze *more* files?

Your value replaces the default list rather than extending it. The default is 26 entries long and covers dependency directories, build outputs, framework caches, and minified files. A short custom list therefore removes most of that protection. Copy the [full default list](configuration/reference.md#analysisexclude_patterns) and add to it.

### Can I use YAML or TOML instead of JSON?

Yes. The format is chosen from the file extension. `jscan.yaml`, `jscan.yml`, and `.jscan.toml` all work, with the same keys.

### Which config file does jscan use?

It searches upward from the path you asked it to analyze, then falls back to the current directory, the XDG config directory, `~/.config/jscan/`, your home directory, and finally the `JSCAN_CONFIG` environment variable. The first file found wins. Pass `--config` to remove the guesswork, and note that `jscan analyze --config` prints the path it used.

## Results

### What counts toward cyclomatic complexity?

Each branch adds one to a baseline of 1: `if`, `else if`, each loop, each `catch`, each `case` label, each ternary, and each `&&`, `||`, or `??`. A `default` clause adds nothing, the same as `else`, and neither does optional chaining with `?.`.

A function with no branching scores 1.

Because every `case` label counts, a `switch` scores the same as the equivalent chain of `if` statements:

```console
$ jscan analyze --json --select complexity src/d.ts \
  | jq -r '.complexity.functions[] | "\(.name): \(.metrics.complexity)"'
ifChain: 5
sw4: 5
```

The `switch_cases` field in the JSON output reports how many case labels the function contains, `default` excluded.

Versions before this behavior landed scored the whole `switch` as a single branch, so switch-heavy code such as reducers and state machines will score higher after upgrading. The code did not get worse; it was under-measured.

### The CBO section says "classes" but my code has none. Why?

In jscan the coupling analysis produces one entry per file, named after the module. The label "classes" comes from the shared metric definition and applies literally in pyscn, which analyzes Python. Read every count in that section as a per-module count.

### What is a good health score?

Grade A starts at 90 and grade B at 75. In practice, treat the category scores as more actionable than the total, because each category saturates at a different point and the total hides which one is costing you.

The complexity category saturates fastest. Its penalty maxes out once the weighted ratio of problematic functions reaches 5 percent, so a handful of high complexity functions in a small codebase can max it out alone.

### Why is `architecture_score` always 0?

Architecture validation is not implemented in jscan. The field exists because the scoring code is shared with pyscn. Ignore both `architecture_score` and `arch_enabled` in JSON output.

### Why does jscan flag code my linter does not?

They answer different questions. A linter checks style and known bug patterns statement by statement. jscan builds a control flow graph and an import graph, which lets it find unreachable code, duplication across files, and circular imports. Neither replaces the other, and running both is normal.

## Performance

### Which analysis is slowest?

Clone detection, followed by dead code detection. The five analyses run in parallel, so total time is set by the slowest rather than by their sum. Dropping clone detection is the single most effective speedup:

```bash
jscan analyze --select complexity,deadcode,cbo,deps src/
```

`jscan check` is fast because it never runs clone or coupling analysis at all.

### The progress bar sits near the end for a long time. Is it stuck?

No. The bar is driven by an elapsed-time estimate rather than by real progress. When a run takes longer than estimated, the bar advances slowly toward 99 percent rather than stopping, so that it does not look frozen. Large repositories often reach that state.

## Project

### How does jscan relate to pyscn?

They are the same analyses for different languages: pyscn for Python, jscan for JavaScript and TypeScript. The language-independent algorithms live in a shared `core` module in the polyscan repository, which both depend on. That is why grade thresholds and the health score formula are identical across the two.

### Where does jscan's source live?

In the [polyscan monorepo](https://github.com/ludo-technologies/polyscan), under `jscan/`. The standalone `jscan` repository was retired and moved into the monorepo.

### Is the JSON output stable?

Not yet. Fields are more likely to be added than removed, but treat the structure as subject to change between releases. Pin the jscan version in anything that parses it.

### How do I report a bug?

Open an issue in the [polyscan repository](https://github.com/ludo-technologies/polyscan/issues). Including the jscan version from `jscan version`, the command you ran, and a small reproduction makes it much faster to act on.

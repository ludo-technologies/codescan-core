# Analyze a TypeScript Project

jscan reads TypeScript directly. It does not run `tsc`, does not read `tsconfig.json`, and does not need your project to compile. Everything it reports comes from parsing the source with a TypeScript grammar, which is why it works on a project that is mid-refactor and currently full of type errors.

That design has consequences worth understanding before you trust the output.

## What jscan understands

These TypeScript constructs are parsed and analyzed normally:

| Construct | Handled |
| --- | --- |
| Type annotations and generics | Yes. Generic functions and classes are analyzed like any other |
| `interface` and `type` declarations | Yes, and they count toward coupling |
| `enum` | Yes |
| Class methods, getters, setters | Yes, each measured as its own function |
| `import type` and `export type` | Yes. Included as dependency graph edges by default |
| `.ts`, `.tsx`, `.mts`, `.cts` | Yes, all four extensions are collected |
| Decorators | Parsed. The decorated class and its methods are analyzed normally |
| Nullish coalescing (`??`) | Yes, and it adds 1 to cyclomatic complexity |
| Optional chaining (`?.`) | Parsed, but it does **not** add to cyclomatic complexity |

Type-only imports are treated as real edges in the dependency graph because they express a design dependency, even though they vanish at compile time. Pass `--include-types=false` to `jscan deps` when you want the runtime graph instead.

## What jscan does not do

Because there is no type checker involved, jscan cannot reason about types across files. It will not tell you that a function returns the wrong type or that an interface member is unused. Use `tsc --noEmit` for that. jscan answers a different question, which is whether the code is structured in a way you can keep working with.

### Path aliases are not resolved

This is the limitation most likely to affect you. If your `tsconfig.json` maps `@/*` to `src/*`, jscan does not know that.

```ts title="src/app/main.ts"
import { label } from "@/lib/util";
import type { Role } from "@/lib/types";
```

jscan records `@/lib/util` as a module in its own right rather than resolving it to `src/lib/util.ts`. In a three-file project with two aliased imports, the dependency graph reports five modules instead of three:

```console
$ jscan deps --format json src/ | jq -r '.graph.nodes | keys[]'
src/app/main.ts
src/lib/types.ts
src/lib/util.ts
@/lib/types
@/lib/util
```

The two extra entries are phantoms. They have no file behind them, and the real edge from `main.ts` to `util.ts` is missing.

The effects are:

- **Module counts are inflated** in the `deps` output and in the health score's dependency category.
- **Real edges are missing**, so cycles that pass through an aliased import are not detected.
- **Maximum depth is understated**, because chains are broken at every alias.

Relative imports such as `./util` and `../lib/util` resolve correctly, so a project that uses relative imports throughout gets an accurate graph.

There is an `module_analysis.alias_patterns` key in the configuration schema, defaulting to `["@/", "~/"]`, but no command reads it yet. See the [configuration reference](../configuration/reference.md#reserved-groups).

!!! tip "Getting an accurate graph today"

    If the dependency graph matters to you more than your import style does, relative imports give correct results now. Otherwise, read the `deps` numbers as a lower bound on your real coupling, and rely on the complexity, dead code, and clone analyses, none of which depend on import resolution.

### Vue single-file components are not parsed

`.vue` files are not collected, so the `<script>` block inside them is never analyzed. The `.ts` and `.js` files in a Vue or Nuxt project are analyzed normally. Support is on the roadmap.

Note that `jscan init --interactive` adds `**/*.vue` to `analysis.include_patterns` when you choose the Vue preset. That key is not read, so the entry has no effect either way.

## Recommended setup

### 1. Fix the exclude patterns first

The default exclude list contains short entries that match as substrings of a file's full path. In a TypeScript application this silently removes real source directories:

- `src/routes/` is skipped, because `routes` contains `out`.
- `src/layout/` and `app/**/layout.tsx` are skipped, for the same reason.
- `src/checkout/` is skipped.
- `src/utils/distance.ts` is skipped, because `distance` contains `dist`.

Nothing in the output mentions the missing files. The only clue is a low count on the `Analyzing N files...` line.

Write an explicit list without the short entries:

```json title="jscan.config.json"
{
  "complexity": {
    "low_threshold": 12,
    "medium_threshold": 24
  },
  "analysis": {
    "exclude_patterns": [
      "node_modules",
      "coverage",
      ".git",
      ".next",
      ".nuxt",
      ".turbo",
      "*.min.js",
      "*.map"
    ]
  }
}
```

Then confirm the file count matches reality:

```console
$ jscan analyze --text src/ | head -1
Analyzing 214 files...

$ find src -name '*.ts' -o -name '*.tsx' | wc -l
214
```

Those two numbers should agree, apart from anything your `.gitignore` excludes. If they do not, a pattern is over-matching.

### 2. Point jscan at the source root

Analyze `src/` rather than the project root. This keeps build output, configuration files, and scripts out of the analysis without needing patterns for them, which is what lets you drop the risky short patterns in step one.

```bash
jscan analyze src/
```

Pass the whole source root rather than a subdirectory. The unused-export analysis compares imports against exports across the analyzed files only, so running it on `src/components/` alone reports every component as unused, because the importers in the rest of `src/` were never read.

### 3. Raise the complexity thresholds

Component code accumulates conditional rendering, and each `&&`, `||`, `??`, and `?:` counts toward cyclomatic complexity. A React component that is entirely readable can score 12 to 15 on that measure. Thresholds of 12 and 24 rather than the default 9 and 19 give a truer picture.

```json
{
  "complexity": {
    "low_threshold": 12,
    "medium_threshold": 24
  }
}
```

### 4. Allow dead code in the gate at first

An exported symbol that no analyzed file imports produces a warning, and `jscan check` fails on warnings by default. In a library, or in any project analyzed one directory at a time, that describes most of your public API.

```bash
jscan check --allow-dead-code --max-complexity 20 src/
```

Tighten both once the first round of fixes has landed.

## Monorepos

There is no workspace-aware mode. Run jscan once per package:

```bash
for pkg in packages/*/; do
  jscan check --allow-dead-code "$pkg/src" || exit 1
done
```

Because config discovery walks upward from the analyzed path, each package can carry its own `jscan.config.json` and fall back to a root file when it has none. The [monorepo example](../configuration/examples.md#monorepo) shows the layout.

Cross-package imports usually go through a workspace alias such as `@myorg/core`, which jscan does not resolve, so a per-package run cannot see them. Run `jscan analyze packages/` for the combined view and per-package runs for the gates.

## Frameworks

### Next.js

jscan knows the App Router conventions. Inside a file under a path containing `/app/` and named `page`, `layout`, `template`, `loading`, `error`, `not-found`, `default`, or `route`, the default export is exempt from the unused-export warning, as are the framework's reserved names such as `metadata`, `generateStaticParams`, and `revalidate`. In `route` files the HTTP verb exports are exempt too.

That exemption only helps if the file reaches the analyzer. A file named `layout.tsx` is dropped by the default exclude list before the exemption is consulted, which is one more reason to fix the patterns first.

Add `.next`, `.vercel`, and `.turbo` to your exclude list.

### Nuxt and Vue

Add `.nuxt` and `.output` to your exclude list. Only the `.ts` and `.js` files are analyzed, since `.vue` files are not parsed.

Note that `.output` is safe to include as a pattern, since it is long enough not to over-match, unlike the bare `out` entry in the default list.

### Node backends

Express and Fastify projects usually have a `routes` directory, which the default exclude list removes. Fix the patterns and the rest works normally. Relative imports are common in backend code, so the dependency graph tends to be accurate here.

## See also

- [Configuration examples](../configuration/examples.md) for complete files
- [Reading the dependency graph](dependency-graph.md) for what the coupling metrics mean
- [CI/CD integration](../integrations/ci-cd.md) for pipeline setups

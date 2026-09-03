# Analyze a TypeScript Project

polyscan reads TypeScript directly. It does not run `tsc`, does not read `tsconfig.json`, and does not need your project to compile. Everything it reports comes from parsing the source with a TypeScript grammar, which is why it works on a project that is mid-refactor and currently full of type errors.

That design has consequences worth understanding before you trust the output.

## What polyscan understands

These TypeScript constructs are parsed and analyzed normally:

| Construct | Handled |
| --- | --- |
| Type annotations and generics | Yes. Generic functions and classes are analyzed like any other |
| `interface` and `type` declarations | Yes, and they count toward coupling |
| `enum` | Yes |
| Class methods, getters, setters | Yes, each measured as its own function |
| `import type` and `export type` | Yes. Included as dependency graph edges |
| `.ts`, `.tsx`, `.mts`, `.cts` | Yes, all four extensions are collected |
| Decorators | Parsed. The decorated class and its methods are analyzed normally |
| Nullish coalescing (`??`) | Yes, and it adds 1 to cyclomatic complexity |
| Optional chaining (`?.`) | Parsed, but it does **not** add to cyclomatic complexity |

Type-only imports are treated as real edges in the dependency graph because they express a design dependency, even though they vanish at compile time.

## What polyscan does not do

Because there is no type checker involved, polyscan cannot reason about types across files. It will not tell you that a function returns the wrong type or that an interface member is unused. Use `tsc --noEmit` for that. polyscan answers a different question, which is whether the code is structured in a way you can keep working with.

### Path aliases are not resolved

This is the limitation most likely to affect you. If your `tsconfig.json` maps `@/*` to `src/*`, polyscan does not know that.

```ts title="src/app/main.ts"
import { label } from "@/lib/util";
import type { Role } from "@/lib/types";
```

polyscan records `@/lib/util` as a module in its own right rather than resolving it to `src/lib/util.ts`. In a three-file project with two aliased imports, the dependency graph reports five modules instead of three:

```console
$ polyscan analyze --format json --select deps src/ 2>/dev/null \
    | jq -r '.deps.graph.nodes | keys[]'
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

There is a `module_analysis.alias_patterns` key in the configuration schema, defaulting to `["@/", "~/"]`, but no command reads it yet. See the [configuration reference](../configuration/reference.md#reserved-groups).

!!! tip "Getting an accurate graph today"

    If the dependency graph matters to you more than your import style does, relative imports give correct results now. Otherwise, read the `deps` numbers as a lower bound on your real coupling, and rely on the complexity, dead code, and clone analyses, none of which depend on import resolution.

### Vue single-file components are not parsed

`.vue` files are not collected, so the `<script>` block inside them is never analyzed. The `.ts` and `.js` files in a Vue or Nuxt project are analyzed normally. Support is on the roadmap.

## Recommended setup

### 1. Check the exclude patterns first

An exclude pattern matches a whole file or directory name, so `dist` skips a directory named `dist` and leaves `src/utils/distance.ts` alone. Any pattern that names a directory you did not mean to exclude removes every file under it, and nothing in the output mentions the missing files. The only clue is a low count on the `Analyzing N files...` line.

jscan versions up to 0.9.0 matched a pattern against any part of a path, so the default entries `out` and `dist` silently dropped `src/routes/`, `src/layout/`, `app/**/layout.tsx`, `src/checkout/`, and `src/utils/distance.ts`. polyscan matches whole names only.

A short explicit list is still worth writing, because your own list replaces the default rather than extending it:

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
$ polyscan analyze --format text src/ | head -1
Analyzing 214 files...

$ find src -name '*.ts' -o -name '*.tsx' | wc -l
214
```

Those two numbers should agree, apart from anything your `.gitignore` excludes. If they do not, a pattern is over-matching.

### 2. Point polyscan at the source root

Analyze `src/` rather than the project root. This keeps build output, configuration files, and scripts out of the analysis without needing patterns for them, which is what lets you keep the list in step one short.

```bash
polyscan analyze src/
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

### 4. Gate on critical findings first

An exported symbol that no analyzed file imports produces a warning-level finding. In a library, or in any project analyzed one directory at a time, that describes most of your public API, so a CI gate should start by counting only the critical findings:

```bash
polyscan analyze --format json src/ 2>/dev/null \
  | jq -e '.summary.critical_dead_code == 0'
```

Tighten the gate once the first round of fixes has landed. The [CI/CD page](../integrations/ci-cd.md) has complete jobs.

## Monorepos

There is no workspace-aware mode. Run polyscan once per package:

```bash
for pkg in packages/*/; do
  polyscan analyze --format json "$pkg/src" 2>/dev/null \
    | jq -e '.summary.critical_dead_code == 0' || exit 1
done
```

Because config discovery walks upward from the analyzed path, each package can carry its own `jscan.config.json` and fall back to a root file when it has none. The [monorepo example](../configuration/examples.md#monorepo) shows the layout.

Cross-package imports usually go through a workspace alias such as `@myorg/core`, which polyscan does not resolve, so a per-package run cannot see them. Run `polyscan analyze packages/` for the combined view and per-package runs for the gates.

## Frameworks

### Next.js

polyscan knows the App Router conventions. Inside a file under a path containing `/app/` and named `page`, `layout`, `template`, `loading`, `error`, `not-found`, `default`, or `route`, the default export is exempt from the unused-export warning, as are the framework's reserved names such as `metadata`, `generateStaticParams`, and `revalidate`. In `route` files the HTTP verb exports are exempt too.

Add `.next`, `.vercel`, and `.turbo` to your exclude list.

### Nuxt and Vue

Add `.nuxt` and `.output` to your exclude list. Only the `.ts` and `.js` files are analyzed, since `.vue` files are not parsed.

### Node backends

Express and Fastify projects usually have a `routes` directory, which jscan versions up to 0.9.0 removed from the analysis; polyscan reads it normally. Relative imports are common in backend code, so the dependency graph tends to be accurate here.

## See also

- [Configuration examples](../configuration/examples.md) for complete files
- [Reading the dependency graph](dependency-graph.md) for what the coupling metrics mean
- [CI/CD integration](../integrations/ci-cd.md) for pipeline setups

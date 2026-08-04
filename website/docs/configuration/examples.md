# Configuration Examples

Complete configuration files for common project shapes. Each one is valid as written, and each notes which parts actually change jscan's behavior today.

Remember two rules while reading these:

- `analysis.exclude_patterns` **replaces** the default list rather than adding to it.
- Each entry matches a whole file or directory name, so `dist` skips a directory named `dist` and leaves `src/utils/distance.ts` alone. The [reference](reference.md#analysisexclude_patterns) explains the matching rules in full.

## Starting point for any project

The smallest file worth writing. It sets complexity thresholds a little more forgiving than the built-in defaults, and it fixes the exclude list so that no source directory is dropped by accident.

```json title="jscan.config.json"
{
  "complexity": {
    "low_threshold": 10,
    "medium_threshold": 20
  },
  "analysis": {
    "exclude_patterns": [
      "node_modules",
      "coverage",
      ".git",
      "*.min.js",
      "*.bundle.js",
      "*.map"
    ]
  }
}
```

Run it against your source directory rather than the repository root, so that build output stays out of the analysis without needing a pattern for it:

```bash
jscan analyze src/
```

## React or Next.js application

Next.js projects keep generated output in `.next` and often have a `src/app` or `src/pages` tree full of route files. The route directories are exactly the ones the default exclude list damages, so the custom list matters here.

```json title="jscan.config.json"
{
  "complexity": {
    "low_threshold": 12,
    "medium_threshold": 24
  },
  "output": {
    "min_complexity": 3
  },
  "analysis": {
    "exclude_patterns": [
      "node_modules",
      ".next",
      ".vercel",
      ".turbo",
      "coverage",
      ".git",
      "*.min.js",
      "*.bundle.js",
      "*.map"
    ]
  }
}
```

The thresholds are raised because component code accumulates conditional rendering, which counts toward cyclomatic complexity without being genuinely hard to read. `min_complexity` of 3 hides the trivial components so that the report is about the parts worth looking at.

Next.js reserves several export names that nothing in your code imports. jscan recognizes them and does not report them as unused, but only inside App Router convention files, meaning a file under a path containing `/app/` and named `page`, `layout`, `template`, `loading`, `error`, `not-found`, `default`, or `route`. In those files the default export is exempt, along with `metadata`, `generateMetadata`, `viewport`, `generateViewport`, `generateStaticParams`, `dynamic`, `dynamicParams`, `revalidate`, `fetchCache`, `runtime`, `preferredRegion`, and `maxDuration`. In `route` files the HTTP verb exports such as `GET` and `POST` are exempt as well.

!!! note "This exemption needs the file to reach the analyzer"

    Up to version 0.9.0 a file named `layout.tsx` was dropped by the default `exclude_patterns`, because the pattern `out` matched any part of a path. The exemption was never consulted for those files. Later versions match whole names only, so `layout.tsx` is analyzed.

## Node.js backend service

Backend code is a better fit for stricter thresholds, and the express-style `routes` directory needs the same care as above.

```json title="jscan.config.json"
{
  "complexity": {
    "low_threshold": 8,
    "medium_threshold": 15,
    "max_complexity": 20
  },
  "analysis": {
    "exclude_patterns": [
      "node_modules",
      "coverage",
      ".git",
      "*.min.js",
      "*.map"
    ]
  }
}
```

`max_complexity` is read only by `jscan check`, where it supplies the default for `--max-complexity`. With this file in place, the gate becomes:

```bash
jscan check src/          # Fails above complexity 20
```

## Library or published package

A library's public exports are consumed by other repositories, so jscan will always report them as unused. The gate has to allow dead code, which makes the configuration file itself fairly plain.

```json title="jscan.config.json"
{
  "complexity": {
    "low_threshold": 8,
    "medium_threshold": 16,
    "max_complexity": 20
  },
  "analysis": {
    "exclude_patterns": [
      "node_modules",
      "coverage",
      ".git",
      "*.map"
    ]
  }
}
```

```bash
# The unused-export warnings are expected here
jscan check --allow-dead-code src/
```

You still get value from the dead code analysis in `jscan analyze`, where the critical findings, which are genuinely unreachable statements, are worth acting on even though the warnings are not.

## Monorepo

There is no workspace-aware mode. Run jscan once per package, and give each package its own file so that thresholds can differ between a strict core library and a looser internal tool.

```text
repo/
├── jscan.config.json          ← fallback for packages without their own
└── packages/
    ├── core/
    │   ├── jscan.config.json  ← stricter
    │   └── src/
    └── web/
        ├── jscan.config.json  ← looser
        └── src/
```

Because discovery walks upward from the analyzed path, `jscan analyze packages/core/src` finds `packages/core/jscan.config.json` first and falls back to the repository root file only when the package has none.

```bash
# Analyze each package separately
for pkg in packages/*/; do
  echo "== $pkg"
  jscan check "$pkg/src" || exit 1
done
```

Analyzing packages separately has one consequence worth understanding. The unused-export check can only see the files in the current run, so anything `packages/web` imports from `packages/core` is reported as an unused export while `core` is analyzed alone. Run `jscan analyze packages/` to see the whole picture, and the per-package runs to gate each package.

## Legacy codebase you are improving gradually

When the current state is far from where you want it, set thresholds you can actually pass today and tighten them over time.

```json title="jscan.config.json"
{
  "complexity": {
    "low_threshold": 20,
    "medium_threshold": 40,
    "max_complexity": 60
  },
  "output": {
    "min_complexity": 15
  },
  "analysis": {
    "exclude_patterns": [
      "node_modules",
      "coverage",
      ".git",
      "legacy/generated",
      "*.min.js",
      "*.map"
    ]
  }
}
```

The high `min_complexity` keeps the report focused on the worst functions rather than producing thousands of lines nobody reads. Lower `max_complexity` by five every time the build passes comfortably, and the gate will ratchet the codebase in the right direction without ever blocking work.

Note that `legacy/generated` contains a slash, so it is matched against the path rather than against a single name. It skips that directory and everything under it, and it matches nothing else.

## YAML instead of JSON

The loader accepts YAML when the filename ends in `.yaml` or `.yml`. The keys are identical.

```yaml title="jscan.yaml"
complexity:
  low_threshold: 10
  medium_threshold: 20
  max_complexity: 25

output:
  min_complexity: 2

analysis:
  exclude_patterns:
    - node_modules
    - coverage
    - .git
    - "*.min.js"
    - "*.map"
```

## See also

- [Configuration reference](reference.md) for every key and its validation rules
- [CI/CD integration](../integrations/ci-cd.md) for using these files in a pipeline

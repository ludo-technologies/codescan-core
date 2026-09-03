<div align="center">

# jscan

**jscan has merged into [polyscan](../polyscan/README.md).**

[![npm](https://img.shields.io/npm/v/polyscan?style=flat-square&logo=npm&label=polyscan)](https://www.npmjs.com/package/polyscan)
[![License](https://img.shields.io/github/license/ludo-technologies/polyscan?style=flat-square)](LICENSE)

</div>

polyscan runs the same JavaScript/TypeScript analysis jscan did — complexity,
dead code, clone detection, coupling, dependencies, and the health score — and
also analyzes Go, Rust and C++, in one report:

```bash
npx polyscan analyze .
```

📖 Documentation: [polyscan.codescan.dev](https://polyscan.codescan.dev/)

## What remains in this directory

- [`npm/`](npm/) — the deprecated [`jscan`](https://www.npmjs.com/package/jscan) npm package, now a thin wrapper that runs the polyscan CLI so existing `npx jscan` invocations keep working while users migrate
- [`CHANGELOG.md`](CHANGELOG.md) — jscan's release history up to the merge
- [`SYNC.md`](SYNC.md) and [`docs/`](docs/) — notes from jscan's development

The analysis code itself lives in the [`polyscan`](../polyscan/) module (its
JavaScript/TypeScript backend), and the standalone `jscan` CLI is no longer
released.

## Migrating

| Before | After |
| --- | --- |
| `npx jscan analyze src/` | `npx polyscan analyze src/` |
| `jscan analyze --json src/` | `polyscan analyze --format json src/` |
| `jscan check src/` | Retired — gate on `polyscan analyze --format json` output |
| `jscan deps src/` | `polyscan analyze --select deps src/` |
| `jscan init` | Retired — polyscan still reads `jscan.config.json` when present |

jscan's former standalone repository is
[ludo-technologies/jscan](https://github.com/ludo-technologies/jscan); releases
up to v0.9.0 live there.

## License

MIT License — see [LICENSE](LICENSE)

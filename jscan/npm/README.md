# jscan (deprecated)

**jscan has merged into [polyscan](https://www.npmjs.com/package/polyscan).**

polyscan runs the same JavaScript/TypeScript analysis jscan did — complexity,
dead code, clone detection, coupling, dependencies, and the health score — and
also analyzes Go, Rust and C++, in one report.

```bash
npx polyscan analyze .
```

This package is now a thin wrapper that runs the polyscan CLI, so existing
`npx jscan` invocations keep working while you migrate. It prints a
deprecation notice and forwards every command to polyscan, translating the
retired `--json`, `--text` and `--html` shorthands to `--format`. `--config`
exits with a hint instead, because polyscan discovers the file itself.

## Migrating

| Before | After |
| --- | --- |
| `npx jscan analyze src/` | `npx polyscan analyze src/` |
| `jscan analyze --json src/` | `polyscan analyze --format json src/` |
| `jscan check src/` | Retired — gate on `polyscan analyze --format json` output |
| `jscan deps src/` | `polyscan analyze --select deps src/` |
| `jscan init` | Retired — polyscan still reads `jscan.config.json` when present |

## Documentation

For full documentation, visit [GitHub](https://github.com/ludo-technologies/polyscan).

## License

MIT

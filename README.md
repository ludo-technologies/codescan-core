<div align="center">

# polyscan

**Code quality analyzers for vibe coders, in every language.**

Building with Cursor, Claude, or ChatGPT? The polyscan family performs structural analysis to keep your codebase maintainable — one shared algorithmic core, one analyzer per language.

[![CI](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml/badge.svg)](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml)
[![npm](https://img.shields.io/npm/v/jscan?style=flat-square&logo=npm&label=jscan)](https://www.npmjs.com/package/jscan)
[![PyPI](https://img.shields.io/pypi/v/pyscn?style=flat-square&logo=pypi&label=pyscn)](https://pypi.org/project/pyscn/)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/ludo-technologies/polyscan?style=flat-square)](LICENSE)

</div>

## Analyzers

| Language | Analyzer | Install | Where |
|----------|----------|---------|-------|
| JavaScript / TypeScript | [**jscan**](jscan/) | `npx jscan analyze src/` | this monorepo — [`jscan/`](jscan/) |
| Python | [**pyscn**](https://github.com/ludo-technologies/pyscn) | `uvx pyscn@latest analyze .` | independent repository |
| Go | **goscan** | — | planned |

Every analyzer works the same way: one command scores your whole codebase (0-100 with an A-F grade) and generates an HTML report that shows what to fix first.

- 🧹 **Dead code** - unreachable code you can safely delete
- 📋 **Duplicate code** - copy-pasted and structurally similar code worth merging (Type 1-4 clone detection)
- 🌀 **Complexity** - functions that are hard to read and test
- 🏗️ **Dependencies** - circular imports and unstable module dependencies
- 🧩 **Class design** - classes that do too much or depend on too much (CBO coupling, LCOM cohesion)

**Built with Go + tree-sitter**

## Shared Core

All analyzers are built on [`core/`](core/), a standalone, language-agnostic Go module: APTED tree edit distance, LSH/MinHash clone indexing, CFG analysis, dead code detection, and coupling/cohesion metrics. Language-specific behavior is injected via interfaces, so a new analyzer only implements parsing and classification.

```bash
go get github.com/ludo-technologies/polyscan/core
```

See [`core/README.md`](core/README.md) for the package catalog and extension points.

## AI Agent Integration

The analyzers ship Agent Skills that teach AI coding agents when and how to run each analysis: health checks, refactoring, architecture review, and CI-friendly reports.

```bash
# jscan Skills (from this monorepo)
npx skills add ludo-technologies/polyscan

# pyscn Skills
npx skills add ludo-technologies/pyscn
```

They work with Claude Code, Cursor, Codex, Gemini CLI, and [70+ other agents](https://github.com/vercel-labs/skills) (add `--agent cursor` etc. to target one, `--global` for all projects).

**Claude Code plugin marketplace:**

```bash
claude plugin marketplace add ludo-technologies/polyscan
claude plugin install jscan@polyscan-marketplace
```

## Repository Layout

| Directory | Description |
|-----------|-------------|
| [`core/`](core/) | Language-agnostic analysis algorithms as a standalone Go module |
| [`jscan/`](jscan/) | JavaScript/TypeScript code quality analyzer and standalone Go module |

jscan moved here from its former standalone repository, [ludo-technologies/jscan](https://github.com/ludo-technologies/jscan); releases up to v0.9.0 live there, and newer releases ship from this monorepo under the same npm package name [`jscan`](https://www.npmjs.com/package/jscan).

### Versioning

Each module is tagged with a directory prefix, e.g. `core/v0.2.1`, `jscan/v0.9.1`.

---

## Documentation

📖 **[pyscn documentation site](https://ludo-technologies.github.io/pyscn/)** • **[jscan README](jscan/README.md)** • **[core README](core/README.md)**

For contributors: **[Contributing](CONTRIBUTING.md)** • **[Code of Conduct](CODE_OF_CONDUCT.md)** • **[Security](SECURITY.md)**

## Enterprise Support

For commercial support, custom integrations, or consulting services, contact us at contact@ludo-tech.org

## License

MIT License — see [LICENSE](LICENSE)

---

*Built with ❤️ using Go and tree-sitter*

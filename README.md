<div align="center">

# polyscan

**Code quality analyzers for vibe coders — in every language.**

Building with Cursor, Claude, or ChatGPT? polyscan performs structural analysis to keep your codebase maintainable: one command scores your whole codebase and shows what to fix first.

[![CI](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml/badge.svg)](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml)
[![npm](https://img.shields.io/npm/v/jscan?style=flat-square&logo=npm&label=jscan)](https://www.npmjs.com/package/jscan)
[![PyPI](https://img.shields.io/pypi/v/pyscn?style=flat-square&logo=pypi&label=pyscn)](https://pypi.org/project/pyscn/)
[![License](https://img.shields.io/github/license/ludo-technologies/polyscan?style=flat-square)](LICENSE)

</div>

## Pick Your Language

| Language | Analyzer | Quick start |
|----------|----------|-------------|
| JavaScript / TypeScript | [**jscan**](jscan/) | `npx jscan analyze src/` |
| Python | [**pyscn**](https://github.com/ludo-technologies/pyscn) | `uvx pyscn@latest analyze .` |
| Go | **goscan** | planned |

No installation needed — the quick-start commands above run the full analysis directly.

## What You Get

Every analyzer scores your codebase (0-100 with an A-F grade) and generates an HTML report that shows what to fix first, looking at your code from five angles:

- 🧹 **Dead code** - unreachable code you can safely delete
- 📋 **Duplicate code** - copy-pasted and structurally similar code worth merging (Type 1-4 clone detection)
- 🌀 **Complexity** - functions that are hard to read and test
- 🏗️ **Dependencies** - circular imports and unstable module dependencies
- 🧩 **Class design** - classes that do too much or depend on too much (CBO coupling, LCOM cohesion)

**Built with Go + tree-sitter** — fast enough to run on every commit.

## AI Agent Integration

The analyzers ship Agent Skills that teach AI coding agents when and how to run each analysis: health checks, refactoring, architecture review, and CI-friendly reports.

```bash
# jscan Skills
npx skills add ludo-technologies/polyscan

# pyscn Skills
npx skills add ludo-technologies/pyscn
```

They work with Claude Code, Cursor, Codex, Gemini CLI, and [70+ other agents](https://github.com/vercel-labs/skills) (add `--agent cursor` etc. to target one, `--global` for all projects).

Then just ask your agent:

1. "Analyze the code quality of the src/ directory"

2. "Find duplicate code and help me refactor it"

3. "Show me complex code and help me simplify it"

**Claude Code plugin marketplace:**

```bash
claude plugin marketplace add ludo-technologies/polyscan
claude plugin install jscan@polyscan-marketplace
```

---

## How It's Built

All analyzers share [`core/`](core/), a standalone, language-agnostic Go module: APTED tree edit distance, LSH/MinHash clone indexing, CFG analysis, dead code detection, and coupling/cohesion metrics. Language-specific behavior is injected via interfaces, so a new analyzer only implements parsing and classification.

```bash
go get github.com/ludo-technologies/polyscan/core
```

| Directory | Description |
|-----------|-------------|
| [`core/`](core/) | Language-agnostic analysis algorithms as a standalone Go module |
| [`jscan/`](jscan/) | JavaScript/TypeScript code quality analyzer and standalone Go module |

jscan moved here from its former standalone repository, [ludo-technologies/jscan](https://github.com/ludo-technologies/jscan); releases up to v0.9.0 live there, and newer releases ship from this monorepo under the same npm package name [`jscan`](https://www.npmjs.com/package/jscan). [pyscn](https://github.com/ludo-technologies/pyscn) remains an independent repository and consumes `core/` as a Go module dependency.

Each module is tagged with a directory prefix, e.g. `core/v0.2.1`, `jscan/v0.9.1`.

## Documentation

📖 **[pyscn documentation site](https://ludo-technologies.github.io/pyscn/)** • **[jscan README](jscan/README.md)** • **[core README](core/README.md)**

For contributors: **[Contributing](CONTRIBUTING.md)** • **[Code of Conduct](CODE_OF_CONDUCT.md)** • **[Security](SECURITY.md)**

## Enterprise Support

For commercial support, custom integrations, or consulting services, contact us at contact@ludo-tech.org

## License

MIT License — see [LICENSE](LICENSE)

---

*Built with ❤️ using Go and tree-sitter*

<div align="center">

# polyscan

**Code quality analyzers for AI agents**

Building with Cursor, Claude, or ChatGPT? polyscan performs structural analysis to keep your codebase maintainable: one command scores your whole codebase and shows what to fix first.

[![CI](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml/badge.svg)](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml)
[![npm](https://img.shields.io/npm/v/jscan?style=flat-square&logo=npm&label=jscan)](https://www.npmjs.com/package/jscan)
[![npm](https://img.shields.io/npm/v/polyscan?style=flat-square&logo=npm&label=polyscan)](https://www.npmjs.com/package/polyscan)
[![PyPI](https://img.shields.io/pypi/v/pyscn?style=flat-square&logo=pypi&label=pyscn)](https://pypi.org/project/pyscn/)
[![License](https://img.shields.io/github/license/ludo-technologies/polyscan?style=flat-square)](LICENSE)

</div>

## Quick Start
No installation needed — the quick-start commands below run the full analysis directly.

### JavaScript / TypeScript
```
npx jscan analyze src/
```

See: [**jscan**](jscan/)

### Python
```
uvx pyscn@latest analyze .
```

See: [**pyscn**](https://github.com/ludo-technologies/pyscn)

### Go / Rust / C++
```
npx polyscan analyze .
```

See: [**polyscan**](polyscan/)

### Others
We are also planning to support more languages.

## What You Get

Every analyzer scores your codebase (0-100 with an A-F grade) and generates an HTML report that shows what to fix first, looking at your code from five angles:

- 🧹 **Dead code** - unreachable code you can safely delete
- 📋 **Duplicate code** - copy-pasted and structurally similar code worth merging (Type 1-4 clone detection)
- 🌀 **Complexity** - functions that are hard to read and test
- 🏗️ **Dependencies** - circular imports and unstable module dependencies
- 🧩 **Class design** - classes that do too much or depend on too much (CBO coupling, LCOM cohesion)

**Built with Go + tree-sitter** — fast enough to run on every commit.

## Polyscan for GitHub

Want this running continuously? The Polyscan App runs the same analyzers on your repositories automatically:

- 📅 **Weekly code audit** — scores the entire codebase once a week and files the report as a GitHub Issue (free while in beta)
- 🔍 **PR code review** — on every pull request, analyzes the changed files with the static analyzers and posts AI-generated improvement suggestions (Pro)

Unlike diff-only review bots, reports are grounded in the quantitative metrics above — complexity, clones, dead code, dependencies. Configured with a single YAML file: report language (en / ja / zh / ko / es / fr / de / pt), target directories, and audit interval.

**[Get started at codescan.dev →](https://codescan.dev/pyscn-bot)**

## AI Agent Integration

The analyzers ship Agent Skills that teach AI coding agents when and how to run each analysis: health checks, refactoring, architecture review, and CI-friendly reports.

```bash
# jscan Skills
npx skills add ludo-technologies/polyscan

# pyscn Skills
uvx add-skills ludo-technologies/pyscn
```

They work with Claude Code, Cursor, Codex, Gemini CLI, and other agents.

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
| [`website/`](website/) | Source of the jscan documentation site at [jscan.codescan.dev](https://jscan.codescan.dev/) |

jscan moved here from its former standalone repository, [ludo-technologies/jscan](https://github.com/ludo-technologies/jscan); releases up to v0.9.0 live there, and newer releases ship from this monorepo under the same npm package name [`jscan`](https://www.npmjs.com/package/jscan). [pyscn](https://github.com/ludo-technologies/pyscn) remains an independent repository and consumes `core/` as a Go module dependency.

Each module is tagged with a directory prefix, e.g. `core/v0.2.1`, `jscan/v0.9.1`.

## Documentation

📖 **[jscan documentation site](https://jscan.codescan.dev/)** • **[pyscn documentation site](https://docs.codescan.dev/)**

**[jscan README](jscan/README.md)** • **[core README](core/README.md)** • **[Performance](docs/performance.md)**

The jscan site is built with MkDocs Material from [`website/`](website/) and deploys to GitHub Pages on every push to `main`.

For contributors: **[Contributing](CONTRIBUTING.md)** • **[Code of Conduct](CODE_OF_CONDUCT.md)** • **[Security](SECURITY.md)**

## Enterprise Support

For commercial support, custom integrations, or consulting services, contact us at contact@ludo-tech.org

## License

MIT License — see [LICENSE](LICENSE)

---

*Built with ❤️ using Go and tree-sitter*

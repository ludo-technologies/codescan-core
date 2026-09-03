<div align="center">

# polyscan

**Code quality analyzers for AI agents**

Building with Cursor, Claude, or ChatGPT? polyscan performs structural analysis to keep your codebase maintainable: one command scores your whole codebase and shows what to fix first.

[![CI](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml/badge.svg)](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml)
[![npm](https://img.shields.io/npm/v/polyscan?style=flat-square&logo=npm&label=polyscan)](https://www.npmjs.com/package/polyscan)
[![PyPI](https://img.shields.io/pypi/v/pyscn?style=flat-square&logo=pypi&label=pyscn)](https://pypi.org/project/pyscn/)
[![License](https://img.shields.io/github/license/ludo-technologies/polyscan?style=flat-square)](LICENSE)

</div>

## Quick Start

No installation needed — one command runs the full analysis for JavaScript, TypeScript, Go, Rust and C++:

```
npx polyscan analyze .
```

The language of each file is detected from its extension, and everything lands in one report with one health score.

### Python

Python has its own analyzer, [pyscn](https://github.com/ludo-technologies/pyscn), built on the same core:

```
uvx pyscn@latest analyze .
```

### Others

We are also planning to support more languages.

> Migrating from jscan? The [`jscan`](https://www.npmjs.com/package/jscan) npm package is deprecated and now runs polyscan for you — switch your scripts to `npx polyscan analyze .`.

## What You Get

Every analyzer scores your codebase (0-100 with an A-F grade) and generates an HTML report that shows what to fix first, looking at your code from five angles:

- 🧹 **Dead code** - unreachable code you can safely delete
- 📋 **Duplicate code** - copy-pasted and structurally similar code worth merging (Type 1-4 clone detection)
- 🌀 **Complexity** - functions that are hard to read and test
- 🏗️ **Dependencies** - circular imports and unstable module dependencies
- 🧩 **Class design** - classes that do too much or depend on too much (CBO coupling, LCOM cohesion)

Complexity and duplicate code cover every language; dead code, dependencies and class design run for JavaScript/TypeScript (and Python via pyscn) today. Dimensions a language does not have are left out of its score, not counted as clean.

The HTML report opens on an overview of the score, the biggest sources of debt and the files to fix first. This is `polyscan analyze src` on [Day.js](https://github.com/iamkun/dayjs):

![polyscan HTML report for Day.js](docs/images/report.png)

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
# polyscan Skills
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
claude plugin install polyscan@polyscan-marketplace
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
| [`polyscan/`](polyscan/) | The polyscan CLI: multi-language analysis with the JavaScript/TypeScript backend that began as jscan |
| [`jscan/`](jscan/) | The deprecated `jscan` npm wrapper and jscan's historical docs |
| [`website/`](website/) | Source of the polyscan documentation site |

jscan moved here from its former standalone repository, [ludo-technologies/jscan](https://github.com/ludo-technologies/jscan), and has since merged into the polyscan CLI; the [`jscan`](https://www.npmjs.com/package/jscan) npm package remains published as a deprecated wrapper that runs polyscan. [pyscn](https://github.com/ludo-technologies/pyscn) remains an independent repository and consumes `core/` as a Go module dependency.

Each module is tagged with a directory prefix, e.g. `core/v0.2.1`, `polyscan/v0.1.0`.

## Documentation

📖 **[polyscan documentation site](https://polyscan.codescan.dev/)** • **[pyscn documentation site](https://docs.codescan.dev/)**

**[polyscan README](polyscan/README.md)** • **[core README](core/README.md)** • **[Performance](docs/performance.md)**

The polyscan site is built with MkDocs Material from [`website/`](website/) and deploys to GitHub Pages on every push to `main`.

For contributors: **[Contributing](CONTRIBUTING.md)** • **[Code of Conduct](CODE_OF_CONDUCT.md)** • **[Security](SECURITY.md)**

## Enterprise Support

For commercial support, custom integrations, or consulting services, contact us at contact@ludo-tech.org

## License

MIT License — see [LICENSE](LICENSE)

---

*Built with ❤️ using Go and tree-sitter*

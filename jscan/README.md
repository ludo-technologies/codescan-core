<div align="center">

# jscan

**A code quality analyzer for JavaScript/TypeScript vibe coders.**

Building with Cursor, Claude, or ChatGPT? jscan performs structural analysis to keep your codebase maintainable.

[![CI](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml/badge.svg)](https://github.com/ludo-technologies/polyscan/actions/workflows/ci.yml)
[![npm](https://img.shields.io/npm/v/jscan?style=flat-square&logo=npm)](https://www.npmjs.com/package/jscan)
[![Downloads](https://img.shields.io/npm/dm/jscan?style=flat-square&logo=npm&label=downloads)](https://www.npmjs.com/package/jscan)
[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/ludo-technologies/polyscan?style=flat-square)](LICENSE)

*Working with Python? Check out [pyscn](https://github.com/ludo-technologies/pyscn)*

</div>

## Quick Start

```bash
# Run analysis without installation
npx jscan analyze src/
```

## Demo

https://github.com/user-attachments/assets/6c491b52-99d3-4fa4-b628-e09c0b61451d

## Features

One command scores your whole codebase (0-100 with an A-F grade) and generates an HTML report that shows what to fix first.

jscan looks at your code from five angles:

- 🧹 **Dead code** - unreachable code, unused imports/exports, and orphan files you can safely delete
- 📋 **Duplicate code** - copy-pasted and structurally similar code worth merging (Type 1-4 clone detection)
- 🌀 **Complexity** - functions that are hard to read and test (cyclomatic complexity)
- 🏗️ **Dependencies** - circular imports and unstable module dependencies (Martin metrics, DOT graph export)
- 🧩 **Class design** - classes that depend on too much (CBO coupling)

**Parallel execution** • Built with Go + tree-sitter

## AI Agent Integration

jscan ships Agent Skills that teach AI coding agents when and how to run each analysis: health checks, refactoring, architecture review, and CI-friendly reports.

### Agent Skills (Recommended)

```bash
npx skills add ludo-technologies/polyscan
```

This installs the Skills into your project. They work with Claude Code, Cursor, Codex, Gemini CLI, and [70+ other agents](https://github.com/vercel-labs/skills) (add `--agent cursor` etc. to target one, `--global` for all projects).

Then just ask your agent:

1. "Analyze the code quality of the src/ directory"

2. "Find duplicate code and help me refactor it"

3. "Show me complex code and help me simplify it"

### Claude Code Plugin (Optional)

```bash
claude plugin marketplace add ludo-technologies/polyscan
claude plugin install jscan@polyscan-marketplace
```

The plugin installs the same Agent Skills through Claude Code's plugin system.

## Installation

```bash
# Install globally with npm (recommended)
npm install -g jscan
```

<details>
<summary>Alternative installation methods</summary>

### Build from source

```bash
git clone https://github.com/ludo-technologies/polyscan.git
cd polyscan/jscan
go build -o jscan ./cmd/jscan
```

### Go install

```bash
go install github.com/ludo-technologies/polyscan/jscan/cmd/jscan@latest
```

</details>

## Common Commands

### `jscan analyze`

Run comprehensive analysis with HTML report

```bash
jscan analyze src/                              # All analyses with HTML report
jscan analyze --format json src/                # Generate JSON report
jscan analyze --select complexity src/          # Only complexity analysis
jscan analyze --select deadcode src/            # Only dead code analysis
jscan analyze --select complexity,deadcode,clone src/  # Multiple analyses
```

### `jscan check`

Fast CI-friendly quality gate

```bash
jscan check src/                         # Quick pass/fail check
```

### `jscan init`

Create configuration file

```bash
jscan init                               # Generate jscan.config.json
```

### `jscan deps`

Dependency visualization

```bash
jscan deps src/ --format dot | dot -Tsvg -o deps.svg
```

> 💡 Run `jscan --help` or `jscan <command> --help` for complete options

## Configuration

Create a `jscan.config.json` or `.jscanrc.json` in your project root:

```json
{
  "complexity": {
    "low_threshold": 10,
    "medium_threshold": 20
  },
  "dead_code": {
    "min_severity": "info"
  },
  "analysis": {
    "include_patterns": ["**/*.ts", "**/*.tsx"],
    "exclude_patterns": ["node_modules", "dist"]
  }
}
```

> ⚙️ Run `jscan init` to generate a configuration file with core options

Part of the schema is still validated but not yet acted on. jscan names those keys in a warning when your file sets one; the [configuration guide](https://jscan.codescan.dev/configuration/#which-keys-take-effect-today) lists them.

## Roadmap

- TypeScript-specific analysis features (type-aware dead code, generic complexity)
- Vue / JSX single-file component support
- IDE / editor integrations
- Watch mode for continuous analysis

---

## Documentation

📚 **[jscan.codescan.dev](https://jscan.codescan.dev/)** — installation, CLI reference, configuration, output formats, and CI/CD integration.

Start with the [quick start](https://jscan.codescan.dev/getting-started/quick-start/), then the [CLI reference](https://jscan.codescan.dev/cli/) and the [configuration guide](https://jscan.codescan.dev/configuration/).

For contributors: **[Development Guide](docs/DEVELOPMENT.md)** • **[Architecture](docs/ARCHITECTURE.md)** • **[Testing](docs/TESTING.md)** • **[Performance](../docs/performance.md)** • **[Contributing](CONTRIBUTING.md)**

## Enterprise Support

For commercial support, custom integrations, or consulting services, contact us at contact@ludo-tech.org

## License

MIT License — see [LICENSE](LICENSE)

---

*Built with ❤️ using Go and tree-sitter*

---
name: refactoring
description: Find refactoring targets in JavaScript/TypeScript, Go, Rust, or C++ code using polyscan - duplicate code (clones), overly complex functions, and dead code. Use when user asks about refactoring, code duplication, complexity hotspots, unreachable code, or cleaning up a codebase.
---

# Refactoring Analysis with polyscan

Run the polyscan CLI to locate concrete refactoring targets in JavaScript, TypeScript, Go, Rust and C++ code. No install needed: `npx polyscan@latest analyze <path>`.

## Commands

| User Request | Command |
|-------------|---------|
| "Find complex functions" | `npx polyscan@latest analyze --format text --select complexity <path>` |
| "Find duplicate code" | `npx polyscan@latest analyze --format text --select clone <path>` |
| "Find dead code" (JS/TS only) | `npx polyscan@latest analyze --format text --select deadcode <path>` |
| "What should I refactor first?" | `npx polyscan@latest analyze --format text --select complexity,deadcode,clone <path>` |

Use `--format json` instead of `--format text` for line-level findings in machine-readable form (written to stdout). Without either, polyscan writes an HTML report and opens a browser.

Complexity and clone detection cover every supported language; dead code detection exists for JavaScript/TypeScript only.

## Interpreting Results

- Complexity risk levels (defaults, configurable for JS/TS in `jscan.config.json`): Low (≤9), Medium (10-19), High (20+). For JavaScript/TypeScript, complexity counts branches plus logical operators (`&&`, `||`, `??`) and ternaries.
- Dead code severity (JS/TS): critical means code after return/break/continue/throw that can never execute; warning covers unreachable branches, unused imports, and orphan files; info covers unused exports.
- Clone types: Type-1 (identical), Type-2 (renamed identifiers/literals), Type-3 (modified, disabled by default), Type-4 (functionally similar), each with a similarity score.

## Prioritizing Findings

1. Critical dead code: safe deletions, do these first.
2. High-complexity functions (20+): extract functions, flatten conditionals.
3. Clone groups spanning multiple files: extract shared helpers; clones within one file are usually quicker wins.

When suggesting a refactor, cite the specific function names, files, and line ranges from the results, and re-run the same command afterward to confirm the improvement.

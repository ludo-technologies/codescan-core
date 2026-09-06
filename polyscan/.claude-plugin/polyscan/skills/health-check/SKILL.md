---
name: health-check
description: Get an overall code quality health score for JavaScript/TypeScript, Go, Rust, or C++ using polyscan. Use when user asks how healthy or good the code is, wants a quality overview, a grade, a summary of technical debt, or a before/after quality comparison.
---

# Code Health Check with polyscan

Run the polyscan CLI to give a quick, quantified picture of code quality across JavaScript, TypeScript, Go, Rust and C++. No install needed:

```bash
npx polyscan@latest analyze --format text <path>
```

The output ends with a Health Score section: a score (0-100), a letter grade (A-F), and per-category scores.

## Commands

| User Request | Command |
|-------------|---------|
| "How healthy is this code?" | `npx polyscan@latest analyze --format text <path>` |
| "Give me a quality overview" | Same command; walk through the category breakdown |
| "Did my refactoring improve quality?" | Run before and after, compare scores |

For machine-readable detail use `--format json`: full results go to stdout and the health-score summary is printed to stderr. **Without `--format text` or `--format json`, polyscan writes an HTML report (`polyscan-report.html`) and opens a browser** — prefer text or JSON when running as an agent.

## Interpreting Results

- Score 0-100 with letter grade; category scores cover complexity, dead code, code duplication, coupling (CBO), and dependencies.
- Complexity and duplication cover every language; dependencies run for Go and JavaScript/TypeScript; dead code and coupling for JavaScript/TypeScript only. A dimension a language does not have is left out of its score, not counted as clean.
- Lead with the grade and the weakest categories, then name the top offenders (files/functions) driving them.
- For deeper follow-up, hand off to the focused skills: refactoring targets → `refactoring`, module structure → `architecture-review`.

Always explain the score in plain terms and suggest the highest-impact next step.

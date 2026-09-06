---
name: architecture-review
description: Analyze JavaScript/TypeScript module architecture using polyscan - class coupling (CBO), instability metrics, and circular dependency detection. Use when user asks about architecture, module structure, coupling, or circular dependencies.
---

# Architecture Review with polyscan

Run the polyscan CLI to understand module structure and coupling. Dependency analysis exists for Go (package graph) and JavaScript/TypeScript (module graph); coupling (CBO) for JavaScript/TypeScript; Rust and C++ get complexity and clone detection only. No install needed: `npx polyscan@latest analyze <path>`.

## Commands

| User Request | Command |
|-------------|---------|
| "Check class coupling" | `npx polyscan@latest analyze --format text --select cbo <path>` |
| "Find circular dependencies" | `npx polyscan@latest analyze --format text --select deps <path>` |
| "Which modules are risky to change?" | `npx polyscan@latest analyze --format json --select deps,cbo <path>` |

Text output shows per-class CBO and cycle membership, but only **aggregate** coupling stats for modules. Per-module Martin metrics (Ca/Ce, instability, abstractness, distance, risk level) and the explicit Zone of Pain / main-sequence module lists live in the `deps` section of the `--format json` output, which goes to stdout with the health-score summary on stderr.

## Interpreting Coupling Results

- High CBO classes depend on many others; changes ripple widely. Suggest interface extraction or dependency inversion.
- Martin metrics per module (from the JSON output above): instability I = Ce / (Ca + Ce) and distance from the main sequence. Modules in the Zone of Pain (stable but concrete) are risky to change; name them explicitly.
- Dependency cycles are the highest-priority architectural issue; name the modules in each cycle and the weakest edge to break.

Always tie findings back to concrete modules and suggest a specific structural change.

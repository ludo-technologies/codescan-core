# Agent Skills

jscan ships four Agent Skills. A skill is a short instruction file that tells an AI coding agent when a tool is relevant, which command to run, and how to interpret the output. Installing them means you can ask for the analysis in plain language instead of remembering the flags.

## Install

```bash
npx skills add ludo-technologies/polyscan
```

This installs the skills into the current project for whichever agents it detects. Two flags are worth knowing:

```bash
npx skills add ludo-technologies/polyscan --agent cursor   # One agent only
npx skills add ludo-technologies/polyscan --global         # All your projects
```

The skills work with Claude Code, Cursor, Codex, Gemini CLI, and [many other agents](https://github.com/vercel-labs/skills).

## Install as a Claude Code plugin

Claude Code can install the same skills through its plugin system:

```bash
claude plugin marketplace add ludo-technologies/polyscan
claude plugin install jscan@polyscan-marketplace
```

The contents are identical. Choose whichever fits how you already manage tooling. There is no benefit to installing both.

## The four skills

| Skill | Triggers on | Runs |
| --- | --- | --- |
| `health-check` | Questions about overall quality, a grade, or a before and after comparison | `jscan analyze` and reads the health score |
| `refactoring` | Questions about duplication, complexity hotspots, or dead code | `jscan analyze` with the relevant `--select` |
| `architecture-review` | Questions about module structure, coupling, or circular imports | `jscan deps` and the coupling analysis |
| `cli-analysis` | Requests for a report file, a CI gate, or project configuration | `jscan check`, `jscan init`, and report generation |

Each skill runs jscan through `npx jscan@latest`, so no separate install is needed. The agent will download it on first use.

## What to ask

Once installed, ordinary requests reach the right analysis:

- "How healthy is the code in `src/`?"
- "Find duplicate code and help me refactor it."
- "Which functions are too complex?"
- "Are there any circular imports?"
- "Set up a CI quality gate for this project."
- "Generate an HTML quality report."

## Reading the results critically

An agent will report what jscan says, and jscan has limitations that are easy to miss when the output is summarized for you. Two are worth telling your agent about.

**Unused exports depend on what was analyzed.** If the agent runs jscan on one directory, every export in it is reported as unused, because the importers elsewhere were never read. Ask the agent to analyze the whole source root before acting on those findings.

**The default exclude patterns drop real directories.** Short entries such as `out` and `dist` match as substrings of the full path, so `src/routes/`, `src/layout/`, and `src/checkout/` are silently skipped. If an agent reports a clean codebase, ask it to confirm the file count on the `Analyzing N files...` line against the number of source files you actually have. The [configuration reference](../configuration/reference.md#analysisexclude_patterns) explains the fix.

Committing a `jscan.config.json` with a corrected exclude list solves this for every future agent run, since the agent picks the file up automatically.

## Working with Python too?

pyscn provides the equivalent skills for Python, but they live in the [pyscn repository](https://github.com/ludo-technologies/pyscn) rather than this one. Installing from `ludo-technologies/polyscan` gives you the four jscan skills only. Add the pyscn skills separately for a mixed codebase.

## See also

- [CLI reference](../cli/index.md) for what the agent is actually running
- [Configuration](../configuration/index.md) for making agent runs accurate by default

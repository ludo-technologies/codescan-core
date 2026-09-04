---
name: polyscan-repo-discovery
description: Discover candidate JavaScript/TypeScript, Go, Rust and C++ repositories for polyscan false-positive auditing via WebSearch. Rotates languages and themes, mixes popular and obscure projects. Appends new candidates to `.polyscan/audit/queue.md`. Designed for periodic invocation.
---

# polyscan Repo Discovery Skill

Discover repositories worth running polyscan against, and append them to a local audit queue at `.polyscan/audit/queue.md` (relative to the monorepo root).

## Goal

Avoid sample bias by varying both the **language** and the **theme** on every run. Always include some **obscure** repos (★50–500) — popular projects show only well-trodden patterns; smaller projects expose the long tail where polyscan is more likely to misfire.

## Input

No argument, or `<n>` (target count, default 8).

## Steps

### 1. Read the queue

Read `.polyscan/audit/queue.md`. If missing, create it from the template at the bottom of this file.
**Never re-add a URL that already appears in the queue** (pending or audited).

### 2. Pick languages and themes

Pick **3 (language, theme) pairs**. Languages rotate across `go`, `rust`, `cpp`, `js`, `ts`; check the `<!-- last-languages: ... -->` and `<!-- last-themes: ... -->` markers in `queue.md` and avoid overlap with the previous run. Over several runs every language should be covered evenly.

Themes:

- CLI / TUI tools
- Web frameworks and middleware (HTTP routers, ORMs, auth libraries)
- Data / numerical / ML runtimes
- DevOps / infrastructure / IaC / Kubernetes operators
- Static analysis / linters / compilers / parsers
- Networking / protocols / proxies
- Games / graphics / audio
- Embedded / systems / OS components
- Cryptography / security
- Async / concurrent runtimes
- Databases / storage engines / caches
- Educational / beginner-friendly projects (good source of low-star repos)

### 3. Discover via sub-agents

Spawn one **`general-purpose` Agent per (language, theme) pair in parallel**. Brief each agent like this:

> "Find open-source repositories on GitHub written primarily in **`<language>`** matching theme: `<theme>`.
>
> Constraints:
> - ★50 – ★10000, weighted toward smaller (at least one candidate must be under ★500)
> - At least one commit within the last 12 months (skip archived repos)
> - Roughly 1k–50k LOC (≈10–500 source files)
> - Exclude well-known majors: kubernetes, docker, terraform, prometheus, cobra, gin, hugo, tokio, serde, ripgrep, rustc, clippy, llvm, tensorflow, opencv, electron, react, vue, typescript, vite, next.js, express
> - Exclude these URLs (already in queue): `<paste URL list from queue.md>`
> - Exclude personal forks of major projects
> - For C++: prefer repos where most code is `.cpp`/`.hpp`/`.h`, not C
> - For JS/TS: prefer source repos, not bundled distributions
>
> Use WebSearch with queries like `site:github.com <language> <keyword>`, awesome-<language> lists, recent HN/Lobsters/Reddit posts.
>
> Return 3–5 candidates as JSON:
> ```json
> [
>   {"url": "https://github.com/owner/repo", "stars": 1234, "language": "<language>", "loc_estimate": "small|medium|large", "theme": "<theme>", "why": "one-line reason this is interesting for polyscan auditing"}
> ]
> ```"

Three pairs in parallel → 9–15 candidates. Deduplicate by URL.

### 4. Append to the queue

Add each new entry under the `## Pending` section in this format:

```markdown
- [ ] https://github.com/owner/repo — <language> — ★1234 — <theme> — discovered 2026-09-05 — <why>
```

Update the trailing markers:

```markdown
<!-- last-languages: go, rust, ts -->
<!-- last-themes: cli, networking, security -->
<!-- last-run: 2026-09-05 -->
```

### 5. Report

Tell the user **only the count, languages and themes** in one line. Don't dump the full list — they can read `queue.md`.

## queue.md template (create if missing)

```markdown
# polyscan FP Audit Queue

JavaScript/TypeScript, Go, Rust and C++ repositories discovered as candidates for polyscan false-positive auditing.
Consumed by `/polyscan-fp-audit`.

## Pending

<!-- new `- [ ] ...` entries appended here -->

## Audited

<!-- when audited, flip `[ ]` to `[x]`, append "audited YYYY-MM-DD", and move here -->

<!-- last-languages: -->
<!-- last-themes: -->
<!-- last-run: -->
```

## Notes

- Skip archived repositories — WebSearch results can be stale
- License doesn't matter (local audit only, no redistribution)
- Skip personal forks of upstream projects (owner is an end user, not a maintainer)
- Python repos belong to `pyscn-repo-discovery` in the pyscn checkout, not here

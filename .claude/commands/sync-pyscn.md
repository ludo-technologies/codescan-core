---
description: pyscn → jscan sync run driven by jscan/SYNC.md
---

Run a pyscn → jscan sync. The single source of truth for policy, file mapping, and history is `jscan/SYNC.md` — read it first and follow it; if this command and SYNC.md disagree, SYNC.md wins.

## Procedure

1. **Read `jscan/SYNC.md`** — note the current baseline SHA, the file-mapping tables with classifications, "Pending changes", and "pyscn-specific, unported features".

2. **Update the pyscn clone**: in `../pyscn`, run `git fetch origin`. All subsequent git commands run against `origin/main` (do not check anything out).

3. **Enumerate candidate commits**: `git log --oneline <baseline>..origin/main` restricted to the paths that appear in SYNC.md's file-mapping tables (`internal/analyzer/`, `domain/`, `.claude-plugin/`). Then filter out noise:
   - Commits that are part of pyscn's own core adoption (deleting local copies, switching imports to `polyscan/core`) — already accounted for; nothing to port.
   - Commits touching only unmapped/pyscn-specific files (tests of unported features, config-loader plumbing, MCP, docs).

4. **Classify each remaining commit** against SYNC.md:
   - Touches a **core-shared algorithm** (anything living in `core/`): nothing to port here — but flag it, because the change belongs in `core/` itself. If pyscn changed behavior that core should own, propose a `core/` change instead of a jscan port.
   - **case-by-case** row: read the diff, decide port / skip. Respect the "do not overwrite" notes (e.g. `calculateComplexityPenalty`, `calculateDeadCodePenalty` are intentionally divergent).
   - **reference-only** row: never port code. Summarize significant design-direction changes for the human.
   - Depends on a feature listed under "pyscn-specific, unported features": skip, and say which feature it's blocked on.

5. **Present the triage before porting**: list each candidate commit with verdict (port / skip / core-issue / report-only) and a one-line reason. Get user confirmation before writing code unless the user already said to proceed.

6. **Port the approved changes** to jscan, adapting to JS/TS semantics and jscan's architecture (jscan builds `domain.Clone` directly, has its own config-loader layer, etc.). After porting: `cd jscan && go build ./... && go test ./...` must pass. If a port changes shared-algorithm behavior, add/extend a parity test against core where one exists.

7. **Update `jscan/SYNC.md`**:
   - Record skipped-this-run changes under "Pending changes" with commit SHAs and reasons.
   - Add a "Sync history" row: date, new pyscn SHA, summary of what was ported and skipped.
   - Update the baseline SHA (and its date) to the `origin/main` HEAD you synced against.

8. **Commit** the jscan changes and the SYNC.md update on a branch (e.g. `sync/pyscn-<shortsha>`), one commit per logical port plus one for SYNC.md, following the repo's commit-message style. Do not push or open a PR unless asked.

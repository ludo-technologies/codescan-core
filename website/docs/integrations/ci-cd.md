# CI/CD Integration

`jscan check` exists for this. It runs a fast subset of the analyses, prints a short verdict, and sets an exit code your pipeline can act on.

| Exit code | Meaning | What the pipeline should do |
| --- | --- | --- |
| 0 | Every check passed | Continue |
| 1 | A threshold was violated | Fail the build |
| 2 | jscan could not run | Fail the build and investigate the job, not the code |

Distinguishing 1 from 2 is worth doing. Exit code 2 usually means a wrong path or a missing checkout rather than a genuine quality problem.

## Before you write the job

Two things will otherwise waste your time.

**The default gate fails on any dead code finding**, including the warning that an exported function is not imported by another analyzed file. In a library, that describes your entire public API. Start with `--allow-dead-code` and tighten later.

**A gate is only as good as the file set it runs on.** Check the `Analyzing N files...` count against reality once, so that you know the gate covers your source tree. Versions up to 0.9.0 matched the exclude patterns `out` and `dist` against any part of a path and skipped `src/routes/`, `src/layout/`, and `src/checkout/`, which made a passing gate meaningless. Pin a version later than 0.9.0 in CI, and see the [configuration reference](../configuration/reference.md#analysisexclude_patterns) for the current matching rules.

## GitHub Actions

### A quality gate

```yaml title=".github/workflows/quality.yml"
name: Code Quality

on:
  pull_request:
  push:
    branches: [main]

jobs:
  jscan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Quality gate
        run: npx jscan@0.4.1 check --allow-dead-code --max-complexity 20 src/
```

Pinning the version keeps a new release from turning a green pipeline red without a change to your code.

### Publishing the HTML report

The report is a single self-contained file, which makes it a good artifact. Note that `analyze` never fails, so this job reports without gating.

```yaml title=".github/workflows/quality.yml"
      - name: Full analysis
        run: npx jscan@0.4.1 analyze --no-open --output jscan-report.html src/

      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: jscan-report
          path: jscan-report.html
          retention-days: 30
```

`--no-open` matters here. Without it jscan tries to launch a browser, which does nothing useful on a runner.

### Posting the score on a pull request

```yaml
      - name: Analyze
        id: jscan
        run: |
          npx jscan@0.4.1 analyze --json src/ 2>/dev/null > report.json
          {
            echo "score=$(jq '.summary.health_score' report.json)"
            echo "grade=$(jq -r '.summary.grade' report.json)"
          } >> "$GITHUB_OUTPUT"

      - name: Comment
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            await github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `**jscan**: ${{ steps.jscan.outputs.score }}/100 (grade ${{ steps.jscan.outputs.grade }})`
            })
```

The `2>/dev/null` keeps the score summary out of the JSON file.

### Failing below a grade

`jscan check` gates on individual thresholds rather than on the score. To gate on the score itself, read it from the JSON:

```yaml
      - name: Require grade B or better
        run: |
          score=$(npx jscan@0.4.1 analyze --json src/ 2>/dev/null | jq '.summary.health_score')
          echo "Health score: $score"
          if [ "$score" -lt 75 ]; then
            echo "::error::Health score $score is below the grade B threshold of 75"
            exit 1
          fi
```

Always run the same `--select` when you gate on the score. A category that did not run contributes no penalty, so a narrower selection scores higher for the same code. See [the health score page](../output/health-score.md#selecting-fewer-analyses-raises-the-score).

### Analyzing a monorepo

```yaml
jobs:
  jscan:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        package: [core, web, api]
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: npx jscan@0.4.1 check --allow-dead-code packages/${{ matrix.package }}/src
```

`fail-fast: false` means one failing package does not cancel the others, so a single run tells you about all of them.

## GitLab CI

```yaml title=".gitlab-ci.yml"
quality:
  stage: test
  image: node:20
  script:
    - npx jscan@0.4.1 check --allow-dead-code --max-complexity 20 src/
    - npx jscan@0.4.1 analyze --no-open --output jscan-report.html src/
  artifacts:
    when: always
    paths:
      - jscan-report.html
    expire_in: 30 days
```

`when: always` publishes the report even when the gate fails, which is exactly when you want to read it.

Note the ordering. `check` runs first and stops the job on failure, so put `analyze` after it and rely on `when: always` to still collect the artifact.

## Pre-commit hook

A gate on the whole project is too slow for a commit hook. Check only the staged files:

```bash title=".git/hooks/pre-commit"
#!/usr/bin/env bash
set -euo pipefail

files=$(git diff --cached --name-only --diff-filter=ACM \
        | grep -E '\.(js|jsx|mjs|cjs|ts|tsx|mts|cts)$' || true)

[ -z "$files" ] && exit 0

# shellcheck disable=SC2086
npx jscan check --select complexity --max-complexity 20 $files
```

Make it executable with `chmod +x .git/hooks/pre-commit`.

Restricting `--select` to `complexity` is deliberate. Dead code detection on a handful of staged files reports almost every export as unused, because the importers are not in the file list.

### With the pre-commit framework

```yaml title=".pre-commit-config.yaml"
repos:
  - repo: local
    hooks:
      - id: jscan
        name: jscan complexity check
        entry: npx jscan check --select complexity --max-complexity 20
        language: system
        files: \.(js|jsx|mjs|cjs|ts|tsx|mts|cts)$
        pass_filenames: true
```

## Docker

There is no official image. Install jscan into whichever image you already use:

```dockerfile
FROM node:20-alpine
RUN npm install -g jscan@0.4.1
WORKDIR /src
ENTRYPOINT ["jscan"]
```

```bash
docker build -t jscan-ci .
docker run --rm -v "$PWD:/src" jscan-ci check --allow-dead-code src/
```

## Tracking the score over time

Appending one line per commit gives you a trend without any extra infrastructure:

```bash
#!/usr/bin/env bash
set -euo pipefail

json=$(jscan analyze --json src/ 2>/dev/null)
printf '%s\t%s\t%s\t%s\n' \
  "$(git rev-parse --short HEAD)" \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  "$(jq '.summary.health_score' <<<"$json")" \
  "$(jq -r '.summary.grade' <<<"$json")" \
  >> quality-history.tsv
```

Two rules keep the series meaningful. Pin the jscan version, since scoring changes between releases would show up as a change in your code quality. Keep the same `--select` and the same configuration file, for the same reason.

## Recommended progression

Adopting the strictest settings on day one means a permanently red pipeline that people learn to ignore. A workable sequence is:

1. **Report only.** Run `jscan analyze` and publish the artifact. Nothing fails. Let the team look at it for a couple of weeks.
2. **Gate on complexity alone**, at a threshold your codebase already passes. `jscan check --select complexity --max-complexity 30 src/`.
3. **Lower the threshold** by five whenever the pipeline has been comfortably green for a while.
4. **Add the dependency check** once the cycles are fixed. `--select complexity,deps`.
5. **Remove `--allow-dead-code`** last, and only if your project is an application rather than a library. In a library the unused-export warnings never go away.

## See also

- [`jscan check` reference](../cli/check.md) for every threshold flag
- [Configuration examples](../configuration/examples.md) for files to commit alongside these jobs

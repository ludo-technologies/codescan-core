# Integrations

jscan is a single binary that reads files and writes results, so it fits anywhere you can run a command.

<div class="grid cards" markdown>

-   :material-source-branch: **CI/CD**

    ---

    Quality gates for GitHub Actions, GitLab CI, and pre-commit hooks, including how to publish the HTML report as a build artifact.

    [Read more](ci-cd.md)

-   :material-robot: **Agent Skills**

    ---

    Teach an AI coding agent when to run each analysis and how to read the output, for Claude Code, Cursor, Codex, and others.

    [Read more](agent-skills.md)

</div>

## Editor integration

There is no editor extension yet. The closest thing available today is a task or watch script that runs jscan on save. It is not fast enough to run on every keystroke, since a full analysis of a large project takes seconds rather than milliseconds, but a fast subset works well on save:

```json title=".vscode/tasks.json"
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "jscan: complexity",
      "type": "shell",
      "command": "jscan analyze --select complexity --text src/",
      "problemMatcher": []
    }
  ]
}
```

## Using jscan from a script

Every command works in a pipeline. `--json` gives machine-readable output, and `jscan check` gives an exit code.

```bash
#!/usr/bin/env bash
set -euo pipefail

score=$(jscan analyze --json src/ 2>/dev/null | jq '.summary.health_score')
echo "Health score: $score"

if [ "$score" -lt 75 ]; then
  echo "Score is below the grade B threshold" >&2
  exit 1
fi
```

Note the `2>/dev/null`. `jscan analyze --json` writes its score summary to standard error, so discarding standard error keeps the parsed output clean. See [output formats](../output/index.md#standard-output-and-standard-error).

## Pinning the version

The JSON output shape is not covered by a stability guarantee yet. Anything that parses it should pin a version rather than track the latest release:

```json title="package.json"
{
  "devDependencies": {
    "jscan": "0.4.1"
  }
}
```

In a pipeline that uses `npx`, name the version explicitly:

```bash
npx jscan@0.4.1 check src/
```

# Output Formats

Which formats are available depends on the command.

| Command | Formats | Default | Destination |
| --- | --- | --- | --- |
| `analyze` | `html`, `json`, `text` | `html` | HTML to a file, the others to standard output |
| `check` | text, `--json` | text | Standard output |
| `deps` | `text`, `json`, `dot` | `text` | Standard output, or `--output` |

## Choosing a format

**HTML** is the format to use when a person is going to read the results. It ranks findings, groups them by category, and is the only format that shows the full clone comparison side by side. See [the HTML report](html-report.md).

**Text** is for a quick look in the terminal and for pasting into an issue. It contains everything the HTML report does, in reading order rather than ranked order.

**JSON** is for anything programmatic: a dashboard, a script that tracks the score over time, or a bot that comments on pull requests. See [the JSON schema](json-schema.md).

**DOT** applies to `jscan deps` only, and produces a Graphviz document for rendering the dependency graph as an image.

## Standard output and standard error

This matters when you redirect output in a script.

For `jscan analyze --json`, the JSON document goes to standard output and the human-readable score summary goes to standard error. That separation is deliberate, and it means the following works:

```bash
jscan analyze --json src/ > report.json     # report.json holds valid JSON
jscan analyze --json src/ 2> /dev/null      # summary suppressed, JSON kept
```

For `jscan analyze --text`, everything goes to standard output, because the text report already ends with its own score section and printing it twice would be noise.

For `jscan analyze` in HTML mode, the report goes to the file and the summary goes to standard output.

Progress bars are written only when the output is an interactive terminal, so redirecting to a file never captures escape sequences.

## Exit codes

Only `jscan check` varies its exit code by result. Every other command exits 0 on success and 1 on an internal error.

| Command | 0 | 1 | 2 |
| --- | --- | --- | --- |
| `analyze` | Analysis completed | Could not run | not used |
| `check` | All checks passed | A threshold was violated | Could not run |
| `deps` | Analysis completed | Could not run | not used |
| `init` | File written | Could not write | not used |

An individual analysis failing inside `analyze` does not fail the command. jscan prints the error to standard error and reports the categories that did succeed, so a parse failure in one file never costs you the whole run.

## The health score

Every format that carries a summary includes a health score from 0 to 100 and a letter grade. It is computed identically in all of them. See [the health score page](health-score.md) for the formula.

## Report file location

The HTML report is written to `jscan-report.html` in the current working directory. Change it with `--output`:

```bash
jscan analyze --output reports/quality.html src/
```

The directory must already exist. jscan does not create intermediate directories, and the command fails if the path cannot be opened for writing.

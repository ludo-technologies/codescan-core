# Output Formats

`polyscan analyze` produces one of three formats, chosen with `--format`.

| Format | Default | Destination |
| --- | --- | --- |
| `html` | Yes | A self-contained report file |
| `json` | | Standard output |
| `text` | | Standard output |

## Choosing a format

**HTML** is the format to use when a person is going to read the results. It ranks findings, groups them by category, and is the only format that shows the full clone comparison side by side. See [the HTML report](html-report.md).

**Text** is for a quick look in the terminal and for pasting into an issue. It contains everything the HTML report does, in reading order rather than ranked order.

**JSON** is for anything programmatic: a dashboard, a script that tracks the score over time, a CI gate, or a bot that comments on pull requests. See [the JSON schema](json-schema.md).

## Standard output and standard error

This matters when you redirect output in a script.

For `polyscan analyze --format json`, the JSON document goes to standard output and the human-readable score summary goes to standard error. That separation is deliberate, and it means the following works:

```bash
polyscan analyze --format json src/ > report.json     # report.json holds valid JSON
polyscan analyze --format json src/ 2> /dev/null      # summary suppressed, JSON kept
```

For `polyscan analyze --format text`, everything goes to standard output, because the text report already ends with its own score section and printing it twice would be noise.

For `polyscan analyze` in HTML mode, the report goes to the file and the summary goes to standard output.

Progress bars are written only when the output is an interactive terminal, so redirecting to a file never captures escape sequences.

## Exit codes

`polyscan analyze` exits 0 when the analysis completed, no matter how poor the results, and 1 when it could not run at all. To fail a pipeline on the results, gate on the JSON output; the [CI/CD page](../integrations/ci-cd.md) shows how.

An individual analysis failing inside `analyze` does not fail the command. polyscan prints the error to standard error and reports the categories that did succeed, so a parse failure in one file never costs you the whole run.

## The health score

Every format includes a health score from 0 to 100 and a letter grade, computed identically in all of them over the dimensions that ran. See [the health score page](health-score.md) for the formula.

## Report file location

The HTML report is written to `polyscan-report.html` in the current working directory. Change it with `--output`:

```bash
polyscan analyze --output reports/quality.html src/
```

The directory must already exist. polyscan does not create intermediate directories, and the command fails if the path cannot be opened for writing.

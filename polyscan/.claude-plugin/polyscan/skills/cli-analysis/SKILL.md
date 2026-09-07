---
name: cli-analysis
description: Run the polyscan command-line tool for code quality analysis of JavaScript/TypeScript, Go, Rust, and C++ - HTML/JSON/text reports, CI-friendly runs, and project configuration. Use when user wants a shareable report file, machine-readable results, or to configure polyscan for a project.
---

# Code Quality Analysis with the polyscan CLI

Use the `polyscan` command-line tool when the task needs report files, CI integration, or machine-readable results. It analyzes JavaScript, TypeScript, Go, Rust and C++ in one run; the language of each file is detected from its extension.

No install needed: `npx polyscan@latest analyze <path>` (or `npm install -g polyscan` for a permanent install). The examples below use a bare `polyscan` for readability — prefix with `npx polyscan@latest` when polyscan isn't installed.

## analyze — the one command

```bash
polyscan analyze .                            # HTML report (polyscan-report.html) + opens browser
polyscan analyze --no-open .                  # HTML report without opening a browser
polyscan analyze --format json src/           # JSON results to stdout
polyscan analyze --format text src/           # human-readable text to stdout
polyscan analyze --select complexity,clone .  # only specific analyses
polyscan analyze --no-open -o report.html .   # custom report path
polyscan analyze --min-complexity 10 .        # list only functions at or above 10
```

Key flags:

- `--format html|json|text` (`-f`): output format; `json` and `text` write to stdout instead of an HTML report
- `--select` (`-s`): `complexity,deadcode,clone,cbo,lcom,deps` (all run by default); `deps` applies to Go and JavaScript/TypeScript; `cbo` to Go, Rust and JavaScript/TypeScript; `lcom` to Go and Rust; `deadcode` to JavaScript/TypeScript only
- `--no-open`: don't auto-open the HTML report in a browser (use in scripts)
- `-o <path>`: HTML report path (default: `polyscan-report.html` in the current directory)
- `--min-complexity <n>`: hide functions below this complexity from the listing (scores still cover everything)

With `--format json`, the machine-readable results go to stdout and the health-score summary goes to stderr, so stdout stays parseable.

## Language coverage

Complexity and clone detection cover every supported language. Dependency analysis covers Go and JavaScript/TypeScript. Class coupling (CBO) covers Go, Rust and JavaScript/TypeScript, and class cohesion (LCOM4) Go and Rust. Dead code exists for JavaScript/TypeScript only. The health score is computed over the dimensions that ran: a dimension a language does not have is left out, not scored as clean.

## CI usage

There is no separate gate command; gate on the JSON output:

```bash
polyscan analyze --format json src/ > report.json
jq -e '.summary.health_score >= 80' report.json
```

## Configuration

JavaScript/TypeScript analysis reads `jscan.config.json` (or `.jscanrc.json`) from the analyzed project when present — complexity thresholds and exclude patterns carried over from jscan, which merged into polyscan. There is no config file for the other languages yet.

## Reporting Results

Summarize the health score and grade, list the specific functions/files behind each failing category, and suggest fixes. For CI setup, recommend a JSON gate as above plus periodic `polyscan analyze` HTML reports for deep reviews.

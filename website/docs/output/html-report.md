# HTML Report

The HTML report is what `polyscan analyze` produces by default. It is a single self-contained file with no external assets, so you can email it, attach it to a pull request, or publish it as a continuous integration artifact and it will render anywhere.

```bash
polyscan analyze src/
```

The file is written to `polyscan-report.html` in the current directory and opened in your browser. Change the path with `--output`, and suppress the browser with `--no-open`.

```bash
polyscan analyze --output reports/quality.html --no-open src/
```

## Layout

The report opens on an Overview and puts the per-analysis detail behind four more tabs. Tabs appear only for the analyses that ran, so a report produced with `--select complexity` has two tabs rather than five, and a project with no JavaScript/TypeScript files has no Classes or Architecture tab. A tab carries a badge with the number of problems it holds, colored red when any of them is high risk.

| Tab | Contents |
| --- | --- |
| Overview | The verdict, the score breakdown, hotspot files, the complexity distribution, and one summary card per remaining analysis |
| Functions | Complexity and dead code, plus the sortable per-file and per-directory rollups |
| Duplication | Clone statistics and the clone groups, or the individual pairs when no group formed |
| Classes | Coupling statistics and the most coupled classes (JavaScript/TypeScript) |
| Architecture | Module dependencies, main sequence zones, circular imports, and the longest dependency chains (JavaScript/TypeScript) |

Above the tabs, the header names the project and reports how many files were analyzed, how many were skipped, and how long the run took.

### The Overview

The Overview answers "what is wrong and where" without opening another tab.

The **verdict** pairs the health score ring with a sentence naming which dimensions are clean and which hold most of the debt, each with the numbers behind the judgement. When files failed to parse, the verdict says so first, because every score below excludes them. The line underneath reports how large the analyzed repository is; see [Project scale](health-score.md#project-scale) for the size labels.

The **score breakdown** gives each dimension its own card with the score, a bar, and the two figures that produced it. Clicking a card jumps to the tab that explains it. Dimensions that did not run — because `--select` excluded them, or because no analyzed language has them — are absent rather than shown as clean.

The **complexity distribution** buckets every analyzed function by cyclomatic complexity. The bucket edges follow the thresholds the run actually used, so the colored buckets hold only functions that really are medium or high risk, and the dashed line marks the complexity where risk begins. Change the JavaScript/TypeScript thresholds with [`complexity.low_threshold`](../configuration/reference.md#complexitylow_threshold) and [`complexity.medium_threshold`](../configuration/reference.md#complexitymedium_threshold).

**Hotspot files** ranks the eight files with the most high-risk functions, then by maximum complexity, joining complexity, dead code, and clone counts per file across every language. The Functions tab has the complete list.

## Row limits

Tables are truncated so that the file stays a reasonable size on a large codebase. Every truncated table says how many rows it is hiding.

| Table | Limit |
| --- | --- |
| Hotspot files | 8 |
| Most complex functions | 20 |
| Dead code findings | 20 |
| Most coupled classes | 20 |
| Clone groups | 10 groups, 10 fragments each |
| Clone pairs | 20 |
| Circular dependencies | 20 |
| Longest dependency chains | 10 |
| All modules | no limit |
| Directory complexity | no limit |

The dead code limit applies to the table as a whole rather than per function. Use `--format json` or `--format text` when you need the complete list.

## Sorting

Each table is sorted by the metric it is about, worst first. Functions are ordered by descending complexity, classes by descending coupling, and clone groups by descending similarity. Since the tables are truncated, this means you always see the worst offenders rather than an arbitrary sample.

The two full-length tables on the Functions tab, **All modules** and **Directory complexity**, are sortable in the browser: click a column heading to sort by it and click again to reverse.

The most-complex-functions table is the one exception to worst-first: it follows [`output.sort_by`](../configuration/reference.md#outputsort_by). Setting that key to `name` therefore leaves you with the first 20 functions alphabetically rather than the 20 worst, which is rarely what you want in the HTML report. The other tables ignore it.

## Dark mode

The report follows the theme of whatever is displaying it. Nothing needs to be configured, and the file is the same either way.

## Linking to a tab

The address bar tracks the open tab, so `polyscan-report.html#architecture` opens straight to Architecture. An unrecognized fragment is ignored and the Overview stays open.

## Sharing and archiving

The report embeds its own styles and scripts and loads nothing over the network, so it works offline and inside a restricted environment. That also makes it a good continuous integration artifact:

```yaml
- name: Analyze
  run: npx polyscan analyze --no-open --output polyscan-report.html src/

- uses: actions/upload-artifact@v4
  with:
    name: polyscan-report
    path: polyscan-report.html
```

The [CI/CD page](../integrations/ci-cd.md) has complete pipeline configurations.

## When the browser does not open

polyscan skips opening a browser when it detects an SSH session, since there is usually no display to open it on. It also prints a warning rather than failing when the browser cannot be launched for any other reason, such as inside a container with no desktop environment.

The report file is written either way. The absolute path is printed on the line beginning `HTML report written to`, so you can open it yourself.

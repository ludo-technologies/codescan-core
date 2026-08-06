# HTML Report

The HTML report is what `jscan analyze` produces by default. It is a single self-contained file with no external assets, so you can email it, attach it to a pull request, or publish it as a continuous integration artifact and it will render anywhere.

```bash
jscan analyze src/
```

The file is written to `jscan-report.html` in the current directory and opened in your browser. Change the path with `--output`, and suppress the browser with `--no-open`.

```bash
jscan analyze --output reports/quality.html --no-open src/
```

## Layout

Above the tabs, the report header carries the health score badge, and directly below it the project scale line reports how large the analyzed repository is. See [Project scale](health-score.md#project-scale) for the size labels.

The report opens on a summary and has one tab per analysis, plus a Modules tab that joins them. Tabs appear only for the analyses that ran, so a report produced with `--select complexity` has three tabs rather than seven.

| Tab | Contents |
| --- | --- |
| Summary | The overall score and grade, the six category scores, and file statistics |
| Complexity | Function count, average and maximum complexity, a table of directory rollups, and a table of functions |
| Dead Code | Finding counts by severity and a table of every issue |
| Clones | Clone pair and group counts and a table of the most similar pairs |
| Coupling | Module count, average CBO, and a table ranked by coupling |
| Dependencies | Module count, entry points, maximum depth, and any circular imports |
| Modules | Per-file quality hotspots joined across the analyses that ran |

Each analysis tab header carries that category's score out of 100, colored by quality band, so you can see where the problem is without opening every tab. The Modules tab has no score of its own: it reports where the other categories' problems are concentrated rather than a category.

## Row limits

Tables are truncated so that the file stays a reasonable size on a large codebase.

| Table | Limit | Notes |
| --- | --- | --- |
| Functions | 20 | A line below the table reports the true total |
| Clone pairs | 20 | A line below the table reports the true total |
| Modules by coupling | 20 | A line below the table reports the true total |
| Circular dependencies | 10 | A line below the table reports the true total |
| Modules | 20 | A line below the table reports the true total |
| Directory complexity | no limit | One row per directory that had a reported function |
| Dead code, per function | 20 | **No line is printed**, so the truncation is silent |
| Dead code, file level | no limit | Every file level finding is shown |

The dead code limit applies per function rather than to the table as a whole. A function with more than 20 findings shows the first 20 and nothing indicates that others exist. Use `--json` or `--text` when you need the complete list.

## Reading the summary tab

The headline is the health score and its grade. Below it sit the six category scores, and below those the file statistics.

Two details in this tab are easy to misread.

The **Total Functions** card changes meaning when `output.min_complexity` filtered anything out. In that case it shows two numbers and relabels itself **Reported / Parsed**, for example `12 / 340`. The first is what the report contains and the second is what jscan actually parsed. When the label says **Total Functions** with a single number, nothing was filtered.

The **architecture score** is not shown, because architecture validation is not implemented in jscan. Only five category scores carry meaning. See [the health score page](health-score.md) for what each one measures.

## Sorting

Every table is sorted by the metric it is about, worst first. Functions are ordered by descending complexity, modules by descending coupling, and clone pairs by descending similarity. Since the tables are truncated at 20 rows, this means you always see the worst offenders rather than an arbitrary sample.

The functions table is the one exception: it follows [`output.sort_by`](../configuration/reference.md#outputsort_by). Setting that key to `name` therefore leaves you with the first 20 functions alphabetically rather than the 20 worst, which is rarely what you want in the HTML report. The other tables ignore it.

## Sharing and archiving

The report embeds its own styles and scripts and loads nothing over the network, so it works offline and inside a restricted environment. That also makes it a good continuous integration artifact:

```yaml
- name: Analyze
  run: jscan analyze --no-open --output jscan-report.html src/

- uses: actions/upload-artifact@v4
  with:
    name: jscan-report
    path: jscan-report.html
```

The [CI/CD page](../integrations/ci-cd.md) has complete pipeline configurations.

## When the browser does not open

jscan skips opening a browser when it detects an SSH session, since there is usually no display to open it on. It also prints a warning rather than failing when the browser cannot be launched for any other reason, such as inside a container with no desktop environment.

The report file is written either way. The absolute path is printed on the line beginning `📊 Unified HTML report generated`, so you can open it yourself.

# Health Score

The health score is a single number from 0 to 100 summarizing everything polyscan measured, with a letter grade attached. It appears at the end of the terminal summary, at the top of the HTML report, and in the `summary` object of the JSON output.

```text
Health Score: 91/100 (Grade: A)
```

This page documents exactly how that number is produced, so that you can predict how a change to your code will move it.

## The formula

Each analyzed dimension produces an integer penalty with a fixed maximum:

| Dimension | Maximum penalty |
| --- | --- |
| Complexity | 20 |
| Dead code | 20 |
| Duplication | 20 |
| Coupling | 20 |
| Dependencies | 16 |

The score charges the run against the dimensions that actually ran:

```text
score = 100 - round(100 × total penalty ÷ maximum possible penalty)
```

where both sums cover only the enabled dimensions. A dimension that did not run — because `--select` excluded it, or because no analyzed language has it — is left out of both sums entirely, rather than scored as clean. A pure Go project is therefore judged on complexity and duplication alone, out of a budget of 40, while a JavaScript project is judged on all five, out of 96. In both cases a clean codebase scores 100 and a fully saturated one scores near 0.

Files that could not be parsed at all are charged separately, after the projection: the parse-error penalty grows with the skipped-file ratio, from a floor of 11 points — enough to cost grade A — up to 56 points when everything was skipped. It exists because every dimension above silently excludes unparsable files, so without it a broken tree would look healthy.

The result is clamped to the range 0 to 100.

!!! note "Changed when jscan merged into polyscan"

    jscan subtracted the raw penalties from 100 directly, so five clean dimensions were a precondition for a high score. polyscan projects the penalties onto the dimensions that ran, which keeps scores meaningful for languages that only have complexity and clone analysis. On a JavaScript project the two formulas agree closely but not exactly: the projection divides by 96 rather than 100.

## Grades

| Grade | Score |
| --- | --- |
| A | 90 to 100 |
| B | 75 to 89 |
| C | 60 to 74 |
| D | 45 to 59 |
| F | 0 to 44 |

These thresholds are shared with pyscn so that a grade means the same thing regardless of the language being analyzed.

## Project scale

Directly below the health score, polyscan prints the size of the repository it just analyzed:

```text
Project Scale: Medium (123 files, 456 functions, 7890 LOC)
```

The label comes from the number of analyzed files alone:

| Label | Analyzed files |
| --- | --- |
| Micro | 0 to 9 |
| Small | 10 to 49 |
| Medium | 50 to 199 |
| Large | 200 to 999 |
| Enterprise | 1000 or more |

The scale is reported for context only. It does not change the health score or the grade in any way, so two repositories of different sizes with identical metrics receive identical scores.

The line count comes from the clone analysis, which is the stage that reads whole files. When clone analysis is turned off, the line count is omitted and the line reads `Project Scale: Medium (123 files, 456 functions)`.

## How each penalty is computed

Four of the dimensions use the same shape. A ratio is computed, and the penalty grows linearly from 0 until the ratio reaches a saturation point, beyond which the penalty stays at its maximum.

### Complexity

```text
ratio   = (high risk functions + 0.5 × medium risk functions) ÷ total functions
penalty = 20 × ratio ÷ 0.05, capped at 20
```

The penalty reaches its maximum once the weighted ratio hits 5 percent. That is a demanding target: a codebase where one function in twenty is high risk already receives the full 20 point penalty. Medium risk functions count half as much as high risk ones. The population covers every analyzed function in every language.

Which functions count as high or medium risk depends on the thresholds, so for JavaScript/TypeScript raising `complexity.medium_threshold` raises the score without changing any code.

### Dead code

```text
weighted = 1.0 × critical + 0.5 × warning + 0.2 × info
rate     = weighted ÷ total files
penalty  = 20 × rate ÷ 3.0, capped at 20
```

This is a per-file rate rather than a raw count, so a large codebase is not penalized simply for being large. The penalty maxes out at three weighted findings per file. The divisor is the file count the dead code analysis covered, which is the JavaScript/TypeScript files.

### Duplication

```text
penalty = 20 × duplication percentage ÷ 30, capped at 20
```

The duplication percentage is the proportion of code fragments, across every language, that participate in at least one clone pair or group. It reaches the maximum penalty at 30 percent.

### Coupling

```text
ratio   = (high coupling modules + 0.3 × medium coupling modules) ÷ total modules
penalty = 20 × ratio ÷ 0.40, capped at 20
```

Medium risk modules are weighted at 0.3 rather than 0.5, which is more forgiving than the complexity weighting. The penalty saturates when the weighted ratio reaches 40 percent.

### Dependencies

This penalty is the sum of three independent parts.

**Cycles, up to 10 points.** Two candidate values are computed and the larger wins:

```text
proportional = 10 × modules in cycles ÷ total modules
floor        = log₂(modules in cycles + 1)
cycle points = min(10, max(proportional, floor))
```

The logarithmic floor exists so that circular imports always cost something. Without it, three modules in a cycle inside a thousand-module repository would round away to nothing.

**Depth, up to 3 points.** polyscan compares your longest import chain against the depth a healthy graph of that size would have:

```text
expected     = max(3, ⌈log₂(total modules + 1)⌉ + 1)
depth points = min(3, max(0, max depth - expected))
```

**Main sequence deviation, up to 3 points.** The Martin distance from the main sequence, a value between 0 and 1, multiplied by 3 and rounded. The [dependency graph guide](../guides/dependency-graph.md) explains what that metric means.

## Category scores

Alongside the overall number, polyscan reports a 0 to 100 score per dimension. These are presentation values and do not feed back into the total.

For complexity, dead code, duplication, and coupling the conversion is direct:

```text
category score = 100 - (penalty × 5)
```

A penalty of 4 out of 20 therefore displays as 80 out of 100.

The dependency score is normalized first, because its maximum penalty is 16 rather than 20:

```text
normalized     = round(penalty ÷ 16 × 20)
category score = 100 - (normalized × 5)
```

The `architecture_score` field in the JSON output is always 0, because architecture validation is not implemented in polyscan; the field exists because the scoring code is shared with pyscn. Ignore it.

## Selecting fewer analyses changes the budget

A dimension that did not run is left out of both the penalty and the budget, so scores from different `--select` values are computed over different denominators and are not comparable:

```console
$ polyscan analyze --format json --select complexity src/ 2>/dev/null | jq .summary.health_score
100

$ polyscan analyze --format json src/ 2>/dev/null | jq .summary.health_score
91
```

The code is identical in both runs. Only the set of dimensions being scored changed. The same applies across languages: a Go project's 90 and a TypeScript project's 90 are both grade A, but they are earned over different dimension sets.

If you track the score over time, always run the same selection on the same tree. The default, which runs everything a language supports, is the right choice for a tracked metric.

## Worked example

Take a TypeScript run summarized as 91 out of 100 with grade A. All five dimensions ran, so the budget is 20 + 20 + 20 + 20 + 16 = 96:

| Dimension | Measurement | Penalty | Category score |
| --- | --- | --- | --- |
| Complexity | 0 high risk and 0 medium risk of 4 functions | 0 | 100 |
| Dead code | 1 critical and 2 warnings across 2 files, a rate of 1.0 | 7 | 65 |
| Duplication | 0 percent | 0 | 100 |
| Coupling | 0 of 2 modules at risk | 0 | 100 |
| Dependencies | 0 cycles, depth 1, deviation 0.6 | 2 | 85 |

The total penalty is 9 out of a possible 96, so the score is `100 - round(100 × 9 ÷ 96) = 100 - 9 = 91`, grade A.

The dead code figure is the one worth following through. The weighted finding count is `1.0 × 1 + 0.5 × 2 = 2.0`, spread over 2 files, giving a rate of 1.0. That is one third of the way to the saturation point of 3.0, so the penalty is one third of 20, which rounds to 7. The category score is `100 - 7 × 5 = 65`.

## What moves the score most

Because every dimension saturates, the fastest gains come from whichever dimension is furthest from zero rather than from whichever has the most raw findings. A project with a dead code score of 65 and a complexity score of 100 should fix dead code first, even if it has far more functions than findings.

The dimensions also differ in how quickly they saturate. Complexity saturates at a weighted ratio of 5 percent, which is by far the tightest of the four. A handful of high complexity functions in a small codebase can max out that penalty on its own.

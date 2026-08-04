# Health Score

The health score is a single number from 0 to 100 summarizing everything jscan measured, with a letter grade attached. It appears at the end of the terminal summary, at the top of the HTML report, and in the `summary` object of the JSON output.

```text
Health Score: 91/100 (Grade: A)
```

This page documents exactly how that number is produced, so that you can predict how a change to your code will move it.

## The formula

The score starts at 100 and subtracts one penalty per category:

```text
score = 100 - (complexity + dead code + duplication + coupling + dependencies + architecture)
```

The result is clamped to the range 0 to 100. Every penalty is an integer.

| Category | Maximum penalty |
| --- | --- |
| Complexity | 20 |
| Dead code | 20 |
| Duplication | 20 |
| Coupling | 20 |
| Dependencies | 16 |
| Architecture | 12 |

Architecture validation is not implemented in jscan, so its penalty is always 0. In practice the largest penalty a JavaScript or TypeScript project can accumulate is 96, and the lowest reachable score is 4.

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

Directly below the health score, jscan prints the size of the repository it just analyzed:

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

Four of the categories use the same shape. A ratio is computed, and the penalty grows linearly from 0 until the ratio reaches a saturation point, beyond which the penalty stays at its maximum.

### Complexity

```text
ratio   = (high risk functions + 0.5 × medium risk functions) ÷ total functions
penalty = 20 × ratio ÷ 0.05, capped at 20
```

The penalty reaches its maximum once the weighted ratio hits 5 percent. That is a demanding target: a codebase where one function in twenty is high risk already receives the full 20 point penalty. Medium risk functions count half as much as high risk ones.

Which functions count as high or medium risk depends on your configured thresholds, so raising `complexity.medium_threshold` raises the score without changing any code.

### Dead code

```text
weighted = 1.0 × critical + 0.5 × warning + 0.2 × info
rate     = weighted ÷ total files
penalty  = 20 × rate ÷ 3.0, capped at 20
```

This is a per-file rate rather than a raw count, so a large codebase is not penalized simply for being large. The penalty maxes out at three weighted findings per file.

Note that the divisor is the file count reported by the dead code analysis. When dead code detection is not selected, the file count is 0 and the penalty is 0.

### Duplication

```text
penalty = 20 × duplication percentage ÷ 30, capped at 20
```

The duplication percentage is the proportion of code fragments that participate in at least one clone pair or group. It reaches the maximum penalty at 30 percent.

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

**Depth, up to 3 points.** jscan compares your longest import chain against the depth a healthy graph of that size would have:

```text
expected     = max(3, ⌈log₂(total modules + 1)⌉ + 1)
depth points = min(3, max(0, max depth - expected))
```

**Main sequence deviation, up to 3 points.** The Martin distance from the main sequence, a value between 0 and 1, multiplied by 3 and rounded. The [dependency graph guide](../guides/dependency-graph.md) explains what that metric means.

### Architecture

Always 0 in jscan. The category exists because the scoring code is shared with pyscn, where architectural layer validation is implemented.

## Category scores

Alongside the overall number, jscan reports a 0 to 100 score per category. These are presentation values and do not feed back into the total.

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

The architecture score is the compliance ratio times 100, which in jscan is always 0. Ignore the `architecture_score` field in JSON output.

## Selecting fewer analyses raises the score

A category that did not run contributes no penalty. That makes scores from different `--select` values incomparable:

```console
$ jscan analyze --json --select complexity src/ | jq .summary.health_score
100

$ jscan analyze --json --select complexity,deadcode src/ | jq .summary.health_score
93

$ jscan analyze --json src/ | jq .summary.health_score
91
```

The code is identical in all three runs. Only the number of categories being scored changed.

If you track the score over time, always run the same selection. The default, which runs all five analyses, is the right choice for a tracked metric.

## Worked example

Take the run summarized as 91 out of 100 with grade A:

| Category | Measurement | Penalty | Category score |
| --- | --- | --- | --- |
| Complexity | 0 high risk and 0 medium risk of 4 functions | 0 | 100 |
| Dead code | 1 critical and 2 warnings across 2 files, a rate of 1.0 | 7 | 65 |
| Duplication | 0 percent | 0 | 100 |
| Coupling | 0 of 2 modules at risk | 0 | 100 |
| Dependencies | 0 cycles, depth 1, deviation 0.6 | 2 | 85 |
| Architecture | not implemented | 0 | 0 |

The total penalty is 9, giving a score of 91 and a grade of A.

The dead code figure is the one worth following through. The weighted finding count is `1.0 × 1 + 0.5 × 2 = 2.0`, spread over 2 files, giving a rate of 1.0. That is one third of the way to the saturation point of 3.0, so the penalty is one third of 20, which rounds to 7. The category score is `100 - 7 × 5 = 65`.

## What moves the score most

Because every category saturates, the fastest gains come from whichever category is furthest from zero rather than from whichever has the most raw findings. A project with a dead code score of 65 and a complexity score of 100 should fix dead code first, even if it has far more functions than findings.

The categories also differ in how quickly they saturate. Complexity saturates at a weighted ratio of 5 percent, which is by far the tightest of the four. A handful of high complexity functions in a small codebase can max out that penalty on its own.

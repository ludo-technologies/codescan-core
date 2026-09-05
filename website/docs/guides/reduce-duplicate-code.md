# Reduce Duplicate Code

polyscan finds duplicated code by comparing syntax trees rather than text, in every supported language. That means it still recognizes a copy after the variables have been renamed, the statements reordered, or the formatting changed, which is what separates it from a plain text search.

```bash
polyscan analyze --select clone src/
```

## How the detection works

Two stages run in sequence, for reasons of cost.

First, every candidate fragment is reduced to a **MinHash fingerprint**, a compact summary of its structure. Fingerprints are indexed with locality-sensitive hashing so that fragments with similar structure land in the same bucket. This step is fast and approximate, and its job is to discard the overwhelming majority of pairs that could not possibly match.

Second, the surviving pairs are compared with **APTED**, an algorithm that computes the edit distance between two trees, meaning the cheapest sequence of insertions, deletions, and relabellings that turns one into the other. This is accurate but cubic in fragment size, which is exactly why the first stage exists.

Fragments of different languages are never compared with each other: a Go function and a TypeScript function can look alike without either being a copy of the other.

## Clone types

Results are graded by how much the copies differ.

| Type | Meaning | Similarity threshold | Reported for |
| --- | --- | --- | --- |
| Type 1 | Identical apart from whitespace and comments | 0.85 | Every language |
| Type 2 | Identical after renaming identifiers and literals | 0.80 for Go, Rust and C++; 0.75 for JS/TS | Every language |
| Type 3 | A copy with statements added, removed, or changed | 0.80 | Go, Rust and C++; off by default for JS/TS |
| Type 4 | Different code computing the same result | 0.65 | JavaScript/TypeScript |

For JavaScript/TypeScript, Type 3 is disabled by default: near-miss matching produces a high false positive rate there, and the findings it adds tend to be pairs that merely resemble each other rather than pairs worth merging.

A fragment must be at least **10 lines** and **20 syntax tree nodes** to be considered at all. This keeps short boilerplate such as getters, simple constructors, and one-line handlers out of the results, since those repeat everywhere and merging them makes code worse rather than better.

In Go, Rust and C++, test code is excluded from clone detection entirely, because test functions share a skeleton by convention. The [analyze reference](../cli/analyze.md#clone) lists exactly what counts as test code per language.

## A worked example

Two functions written months apart in different files:

```ts title="src/orders.ts"
export function summarizeOrders(rows: any[]) {
  const out: any[] = [];
  for (const row of rows) {
    if (!row) continue;
    if (row.status === "cancelled") continue;
    const total = row.price * row.qty;
    if (total <= 0) continue;
    out.push({ id: row.id, total, label: row.name.trim().toLowerCase() });
  }
  out.sort((a, b) => b.total - a.total);
  return out;
}
```

```ts title="src/invoices.ts"
export function summarizeInvoices(items: any[]) {
  const results: any[] = [];
  for (const item of items) {
    if (!item) continue;
    if (item.status === "cancelled") continue;
    const amount = item.price * item.qty;
    if (amount <= 0) continue;
    results.push({ id: item.id, total: amount, label: item.name.trim().toLowerCase() });
  }
  results.sort((a, b) => b.total - a.total);
  return results;
}
```

Every identifier differs. A text search finds nothing. polyscan reports:

```console
$ polyscan analyze --format text --select clone src/
=== Clone Detection ===

Statistics:
  Total clone pairs: 1
  Total clone groups: 1
  Files analyzed: 2
  Average similarity: 0.85

Clone Types:
  Type-2: 1

Top Clone Pairs:
  Type-2: src/invoices.ts:1:7-12:1 <-> src/orders.ts:1:7-12:1 (85.0% similar)
```

Type 2 at 85 percent similarity is the signature of a copy and paste followed by renaming.

## Pairs and groups

A **pair** is two fragments that match. A **group** collects fragments that are all mutually similar.

Groups are the more useful view once the same logic has been copied more than once. Five copies of one function produce ten pairs but a single group of five, and the group tells you the thing you actually want to know, which is that one function needs extracting rather than that ten pairs need reviewing.

The duplication percentage in the health score is computed from fragments rather than pairs:

```text
duplication percentage = clone fragments ÷ total fragments × 100
```

It reaches the maximum penalty of 20 points at 60 percent. The ratio counts every fragment with at least one partner, so it runs high on languages with conventional function shapes: well-kept Go libraries such as cobra and x/crypto sit near 20 percent, and wrapper-heavy ones such as testify and afero near 35 percent. See [the health score page](../output/health-score.md#duplication).

## What to do about a finding

Not every clone should be merged. Work through these questions in order.

**Do the copies change for the same reason?** This is the question that matters most. Two functions that look alike but serve different parts of the business will drift apart, and merging them creates a shared function full of conditional branches serving both callers badly. Duplication is cheaper than the wrong abstraction.

**Is the copy a bug waiting to happen?** If fixing a bug in one copy requires remembering to fix the others, merge them. This is the strongest argument for extraction and usually the correct one for validation logic, parsing, and formatting.

**Is the duplication accidental or structural?** Two functions that are similar because both iterate a list and filter it are structurally similar without being duplicated. polyscan cannot tell the difference, and Type 4 findings in particular often fall into this category.

**Is it test code?** Test files repeat setup deliberately, because a test that reads top to bottom without indirection is easier to debug. polyscan already excludes test code from clone detection for Go, Rust and C++; for JavaScript/TypeScript, exclude test directories with `analysis.exclude_patterns` if the findings are noise.

When you do merge, extract the varying parts as parameters. In the example above, the two functions differ only in their variable names, so a single `summarize(rows)` replaces both directly.

## Reviewing findings efficiently

Start with the highest similarity rather than the largest fragment. A pair at 95 percent is almost certainly a copy. A pair barely over its type's threshold is often a coincidence.

```bash
# The most similar pairs first, which is the default order
polyscan analyze --select clone --format text src/ | head -40
```

For a project you are reviewing over time, track the group count rather than the pair count. Pairs grow quadratically with each new copy and overstate the problem.

```bash
polyscan analyze --format json --select clone src/ 2>/dev/null \
  | jq '{groups: .summary.clone_groups, pairs: .summary.clone_pairs,
         percent: .summary.code_duplication_percentage}'
```

## Speed

Clone detection is usually the slowest of the analyses. When you are iterating on something else, leave it out:

```bash
polyscan analyze --select complexity,deadcode src/
```

## See also

- [`polyscan analyze` reference](../cli/analyze.md#clone) for the detection settings
- [Health score](../output/health-score.md#duplication) for how duplication affects the score

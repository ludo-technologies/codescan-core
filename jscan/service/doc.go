// Package service orchestrates the analyzers and turns their results into
// output. It sits between the CLI in cmd/jscan and the engines in
// internal/analyzer.
//
// Each analysis has a service that owns its pipeline: ComplexityServiceImpl,
// the dead code functions in dead_code_service.go and dead_code_aggregate.go,
// CloneServiceImpl, CBOServiceImpl, and DependencyGraphServiceImpl. They all
// take a domain request, fan the work out across files, and return a domain
// response. A file that fails to parse contributes a warning or an error to the
// response rather than aborting the run.
//
// Dead code detection is split in two because the two halves need different
// inputs. Per-function unreachable code comes from each file's control flow
// graph on its own, while unused imports, unused exports, and orphan files can
// only be decided once every file in the run has been read. That cross-file
// pass is why analyzing a subdirectory reports its exports as unused: the
// importers were never part of the input.
//
// Output is handled by OutputFormatterImpl for text, JSON, YAML, CSV, and HTML,
// and by DOTFormatter for Graphviz. Note that the CLI reaches only the text,
// JSON, HTML, and DOT paths today; analyze maps any --format value other than
// json or text to HTML, leaving the YAML and CSV writers unreachable.
//
// BuildAnalyzeSummary combines the five responses into a domain.AnalyzeSummary
// and computes the health score. Because a nil response is treated as an
// analysis that did not run, and a category that did not run contributes no
// penalty, summaries built from different --select values are not comparable.
package service

// Package reporter formats complexity results for text, JSON, CSV, and HTML
// output.
//
// Nothing in jscan currently imports this package. The CLI formats its output
// through service.OutputFormatterImpl instead, so the code here runs only under
// its own tests. It is kept because it is the only caller of
// config.ComplexityConfig.ExceedsMaxComplexity, which is the intended home for
// max-complexity reporting in analyze rather than only in check.
//
// Treat this as unwired rather than as a second supported path: changing the
// report format here will not change what any command prints.
package reporter

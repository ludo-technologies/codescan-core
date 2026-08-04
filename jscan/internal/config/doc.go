// Package config loads, validates, and generates jscan configuration.
//
// LoadConfigWithTarget is the entry point every command uses. Given an explicit
// path it loads exactly that file; given none it searches, starting at the
// analyzed path and walking upward to the filesystem root before falling back
// to the current directory, the XDG config directory, ~/.config/jscan, the home
// directory, and finally the JSCAN_CONFIG and PYSCN_CONFIG variables. Searching
// upward from the target rather than the working directory is what lets a
// monorepo package carry its own configuration. The pyscn-prefixed filenames
// and variables are accepted for backward compatibility.
//
// Viper supplies the parsing, so JSON, YAML, and TOML are all accepted and the
// format follows the file extension.
//
// Validate enforces the constraints on every group, and an invalid value fails
// the whole run.
//
// # Applied and unapplied settings
//
// Config models more than the commands currently consume. Validation covers the
// whole struct, but only these fields reach behavior:
//
//   - Complexity.LowThreshold and Complexity.MediumThreshold, in analyze and check
//   - Complexity.MaxComplexity, in check only, as the default for --max-complexity
//   - Output.MinComplexity, in analyze only
//   - Analysis.ExcludePatterns, in analyze, check, and deps
//
// Everything else is parsed, validated, and then ignored, including all of
// DeadCode, Clones, SystemAnalysis, Dependencies, Architecture, ModuleAnalysis,
// Analysis.IncludePatterns, Analysis.Recursive, Analysis.FollowSymlinks,
// Complexity.Enabled, and Complexity.ReportUnchanged.
//
// The helper methods AssessRiskLevel, ShouldReport, ShouldDetectDeadCode,
// GetMinSeverityLevel, and HasAnyDetectionEnabled exist for those unapplied
// settings and have no callers outside tests. Anything wiring a setting up
// should use them rather than reimplementing the checks.
//
// # Templates
//
// GetFullConfigTemplate and GetMinimalConfigTemplate produce the files written
// by jscan init, combining a ProjectType preset that selects file patterns with
// a Strictness preset that selects complexity thresholds. Both emit plain JSON
// with no comments.
//
// Note that the generated exclude list is shorter than DefaultConfig's, so
// writing a configuration file narrows what jscan skips rather than widening it.
package config

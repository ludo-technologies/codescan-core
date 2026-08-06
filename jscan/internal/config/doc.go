// Package config loads, validates, and generates jscan configuration.
//
// Load is the entry point every command uses. Given an explicit
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
// whole struct, but only the keys in appliedKeys reach behavior:
//
//   - Complexity.LowThreshold and Complexity.MediumThreshold, in analyze and check
//   - Complexity.MaxComplexity, in check only, as the default for --max-complexity
//   - Complexity.ReportUnchanged, wherever complexity runs
//   - DeadCode.MinSeverity and DeadCode.SortBy
//   - Output.MinComplexity, in analyze only, and Output.SortBy
//   - Analysis.IncludePatterns, Analysis.ExcludePatterns, and Analysis.Recursive
//
// Everything else is parsed, validated, and then ignored, including all of
// Clones, SystemAnalysis, Dependencies, Architecture, ModuleAnalysis, the rest
// of DeadCode, Output.Format, Output.ShowDetails, Output.Directory,
// Analysis.FollowSymlinks, and Complexity.Enabled.
//
// LoadResult.IgnoredKeys reports the keys a file sets that fall in that second
// group, so the commands can say so rather than leaving the user to find out.
// A key being wired up therefore means adding it to appliedKeys in the same
// change, and the templates below are tested against that same set.
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

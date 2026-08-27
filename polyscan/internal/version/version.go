// Package version carries the build information stamped into the binary.
package version

import "fmt"

// Set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
	BuiltBy = "source"
)

// Full returns the version together with its build provenance.
func Full() string {
	return fmt.Sprintf("%s (commit: %s, built: %s, by: %s)", Version, Commit, Date, BuiltBy)
}

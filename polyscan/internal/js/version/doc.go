// Package version carries the build metadata reported by jscan version.
//
// All four variables are set through linker flags at build time and hold
// placeholders otherwise: Version defaults to "dev", Commit and Date to
// "unknown", and BuiltBy to "source".
//
// Both the Makefile (BuiltBy="make") and the release workflow (BuiltBy="release")
// populate all four variables. A binary built directly via "go build" without
// linker flags retains the default placeholder values.
package version

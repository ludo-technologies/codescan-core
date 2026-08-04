// Package version carries the build metadata reported by jscan version.
//
// All four variables are set through linker flags at build time and hold
// placeholders otherwise: Version defaults to "dev", Commit and Date to
// "unknown", and BuiltBy to "source".
//
// The Makefile populates all four. The release workflow sets only Version, so
// an official release binary still reports "unknown" for the commit and date
// and "source" as its builder. Anything that reads those fields should treat
// them as unpopulated rather than as evidence about how the binary was built.
package version

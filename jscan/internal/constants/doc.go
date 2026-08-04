// Package constants holds the tuning values shared across the analyzers.
//
// Clone detection is the main consumer. DefaultCloneMinLines and
// DefaultCloneMinNodes set the floor on fragment size, which keeps short
// boilerplate such as getters and one-line handlers out of the results. The
// DefaultTypeNCloneThreshold values set the similarity required for each clone
// type, and DefaultLSHAutoThreshold is the fragment count above which
// locality-sensitive hashing switches on automatically.
//
// These are defaults rather than settings. The clones group in the
// configuration file is parsed but not applied, so changing a value here
// changes the behavior for every run.
package constants

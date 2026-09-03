package version

// Version information (set via ldflags during build)
var (
	// Version is the current version of jscan
	Version = "dev"

	// Commit is the git commit hash
	Commit = "unknown"

	// Date is the build date
	Date = "unknown"

	// BuiltBy indicates how the binary was built
	BuiltBy = "source"
)

// GetVersion returns the current version
func GetVersion() string {
	if Version == "" {
		return "dev"
	}
	return Version
}

// Short returns a shortened version string (same as GetVersion for now)
func Short() string {
	return GetVersion()
}

// GetCommit returns the git commit hash
func GetCommit() string {
	return Commit
}

// GetDate returns the build date
func GetDate() string {
	return Date
}

// GetBuiltBy returns how the binary was built
func GetBuiltBy() string {
	return BuiltBy
}

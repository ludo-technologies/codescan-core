package version

import (
	"testing"
)

func TestDefaultVersion(t *testing.T) {
	// Verify default values
	origVersion, origCommit, origDate, origBuiltBy := Version, Commit, Date, BuiltBy
	t.Cleanup(func() {
		Version, Commit, Date, BuiltBy = origVersion, origCommit, origDate, origBuiltBy
	})

	Version = "dev"
	Commit = "unknown"
	Date = "unknown"
	BuiltBy = "source"

	if got := GetVersion(); got != "dev" {
		t.Errorf("GetVersion() = %q, want %q", got, "dev")
	}
	if got := Short(); got != "dev" {
		t.Errorf("Short() = %q, want %q", got, "dev")
	}
	if got := GetCommit(); got != "unknown" {
		t.Errorf("GetCommit() = %q, want %q", got, "unknown")
	}
	if got := GetDate(); got != "unknown" {
		t.Errorf("GetDate() = %q, want %q", got, "unknown")
	}
	if got := GetBuiltBy(); got != "source" {
		t.Errorf("GetBuiltBy() = %q, want %q", got, "source")
	}
}

func TestEmptyVersionDefaultsToDev(t *testing.T) {
	origVersion := Version
	t.Cleanup(func() {
		Version = origVersion
	})

	Version = ""
	if got := GetVersion(); got != "dev" {
		t.Errorf("GetVersion() = %q, want %q", got, "dev")
	}
	if got := Short(); got != "dev" {
		t.Errorf("Short() = %q, want %q", got, "dev")
	}
}

func TestCustomVersionMetadata(t *testing.T) {
	origVersion, origCommit, origDate, origBuiltBy := Version, Commit, Date, BuiltBy
	t.Cleanup(func() {
		Version, Commit, Date, BuiltBy = origVersion, origCommit, origDate, origBuiltBy
	})

	Version = "1.2.3"
	Commit = "abc1234"
	Date = "2026-08-14T07:40:09Z"
	BuiltBy = "release"

	if got := GetVersion(); got != "1.2.3" {
		t.Errorf("GetVersion() = %q, want %q", got, "1.2.3")
	}
	if got := Short(); got != "1.2.3" {
		t.Errorf("Short() = %q, want %q", got, "1.2.3")
	}
	if got := GetCommit(); got != "abc1234" {
		t.Errorf("GetCommit() = %q, want %q", got, "abc1234")
	}
	if got := GetDate(); got != "2026-08-14T07:40:09Z" {
		t.Errorf("GetDate() = %q, want %q", got, "2026-08-14T07:40:09Z")
	}
	if got := GetBuiltBy(); got != "release" {
		t.Errorf("GetBuiltBy() = %q, want %q", got, "release")
	}
}

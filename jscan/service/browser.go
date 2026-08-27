package service

import "github.com/ludo-technologies/polyscan/core/util"

// IsSSH returns true if the session is running over SSH.
func IsSSH() bool {
	return util.IsSSH()
}

// IsInteractiveEnvironment returns true if the environment appears to be
// an interactive TTY session (and not CI).
func IsInteractiveEnvironment() bool {
	return util.IsInteractiveEnvironment()
}

// OpenBrowser opens the specified URL in the default browser.
func OpenBrowser(url string) error {
	return util.OpenBrowser(url)
}

package util

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// IsSSH returns true if the session is running over SSH.
func IsSSH() bool {
	return os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != ""
}

// IsInteractiveEnvironment returns true if the environment appears to be
// an interactive TTY session (and not CI).
func IsInteractiveEnvironment() bool {
	if os.Getenv("CI") != "" {
		return false
	}
	if fi, err := os.Stderr.Stat(); err == nil {
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

// OpenBrowser opens the specified URL in the default browser.
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		for _, openCmd := range []string{"xdg-open", "gnome-open", "kde-open"} {
			if _, err := exec.LookPath(openCmd); err == nil {
				cmd = openCmd
				args = []string{url}
				break
			}
		}
		if cmd == "" {
			return fmt.Errorf("no suitable browser opener found for Linux")
		}
	case "windows":
		// rundll32 hands the URL to the file protocol handler without a
		// shell. "cmd /c start" would let cmd.exe parse the URL, where an
		// & in a path is a command separator.
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if err := exec.Command(cmd, args...).Start(); err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}

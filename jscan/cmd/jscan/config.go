package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/ludo-technologies/polyscan/jscan/internal/config"
)

// configDocsURL documents which configuration keys change behavior.
const configDocsURL = "https://jscan.codescan.dev/configuration/#which-keys-take-effect-today"

// loadCommandConfig loads the configuration a command should run with and
// reports the keys the file sets that reach no behavior.
//
// The report goes to warn rather than stdout, which commands reserve for the
// results themselves, and it is written for every format: a key that quietly
// does nothing is exactly what the user needs to hear about.
func loadCommandConfig(configPath, targetPath string, warn io.Writer) (*config.Config, error) {
	result, err := config.Load(configPath, targetPath)
	if err != nil {
		return nil, err
	}

	if len(result.IgnoredKeys) > 0 {
		noun := "keys"
		if len(result.IgnoredKeys) == 1 {
			noun = "key"
		}
		fmt.Fprintf(warn, "Warning: %s sets %d %s that no command reads: %s\n",
			result.Path, len(result.IgnoredKeys), noun, strings.Join(result.IgnoredKeys, ", "))
		fmt.Fprintf(warn, "  See %s\n", configDocsURL)
	}

	return result.Config, nil
}

package config

import (
	"encoding/json"
	"testing"
)

// TestTemplatesEmitOnlyAppliedKeys is the guard that keeps jscan init honest: a
// generated file must not contain a key that changes nothing, because the first
// thing a new user reads is that file.
func TestTemplatesEmitOnlyAppliedKeys(t *testing.T) {
	templates := map[string]string{
		"minimal": GetMinimalConfigTemplate(),
	}
	for _, projectType := range []ProjectType{ProjectTypeGeneric, ProjectTypeReact, ProjectTypeVue, ProjectTypeNodeBackend} {
		for _, strictness := range []Strictness{StrictnessRelaxed, StrictnessStandard, StrictnessStrict} {
			templates[string(projectType)+"/"+string(strictness)] = GetFullConfigTemplate(projectType, strictness)
		}
	}

	for name, template := range templates {
		var groups map[string]map[string]any
		if err := json.Unmarshal([]byte(template), &groups); err != nil {
			t.Fatalf("%s template is not valid JSON: %v", name, err)
		}

		for group, keys := range groups {
			for key := range keys {
				fullKey := group + "." + key
				if !appliedKeys[fullKey] {
					t.Errorf("%s template sets %s, which no command reads", name, fullKey)
				}
			}
		}
	}
}

// TestTemplatesAreValidConfigurations checks that a generated file loads, since
// a template that fails validation would break jscan init followed by any run.
func TestTemplatesAreValidConfigurations(t *testing.T) {
	for _, template := range []string{
		GetMinimalConfigTemplate(),
		GetFullConfigTemplate(ProjectTypeReact, StrictnessStrict),
	} {
		config := DefaultConfig()
		if err := json.Unmarshal([]byte(template), config); err != nil {
			t.Fatalf("template is not valid JSON: %v", err)
		}
		if err := config.Validate(); err != nil {
			t.Errorf("template fails validation: %v", err)
		}
	}
}

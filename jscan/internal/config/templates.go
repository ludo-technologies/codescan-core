package config

import "strconv"

// ProjectType represents the type of JavaScript/TypeScript project
type ProjectType string

const (
	ProjectTypeGeneric     ProjectType = "generic"
	ProjectTypeReact       ProjectType = "react"
	ProjectTypeVue         ProjectType = "vue"
	ProjectTypeNodeBackend ProjectType = "node"
)

// Strictness represents the analysis strictness level
type Strictness string

const (
	StrictnessRelaxed  Strictness = "relaxed"
	StrictnessStandard Strictness = "standard"
	StrictnessStrict   Strictness = "strict"
)

// ProjectPreset holds configuration presets for different project types
type ProjectPreset struct {
	IncludePatterns []string
	ExcludePatterns []string
}

// StrictnessPreset holds threshold values for different strictness levels
type StrictnessPreset struct {
	LowThreshold    int
	MediumThreshold int
	MaxComplexity   int
}

// GetProjectPresets returns presets for different project types
func GetProjectPresets() map[ProjectType]ProjectPreset {
	return map[ProjectType]ProjectPreset{
		ProjectTypeGeneric: {
			IncludePatterns: []string{
				"**/*.js",
				"**/*.ts",
				"**/*.jsx",
				"**/*.tsx",
			},
			ExcludePatterns: []string{
				"node_modules",
				"dist",
				"build",
				"*.min.js",
				"*.bundle.js",
			},
		},
		ProjectTypeReact: {
			IncludePatterns: []string{
				"**/*.js",
				"**/*.ts",
				"**/*.jsx",
				"**/*.tsx",
			},
			ExcludePatterns: []string{
				"node_modules",
				"dist",
				"build",
				".next",
				"coverage",
				"*.min.js",
				"*.bundle.js",
			},
		},
		ProjectTypeVue: {
			IncludePatterns: []string{
				"**/*.js",
				"**/*.ts",
				"**/*.jsx",
				"**/*.tsx",
				"**/*.vue",
			},
			ExcludePatterns: []string{
				"node_modules",
				"dist",
				"build",
				".nuxt",
				"coverage",
				"*.min.js",
				"*.bundle.js",
			},
		},
		ProjectTypeNodeBackend: {
			IncludePatterns: []string{
				"**/*.js",
				"**/*.ts",
				"**/*.mjs",
				"**/*.cjs",
			},
			ExcludePatterns: []string{
				"node_modules",
				"dist",
				"build",
				"test",
				"tests",
				"__tests__",
				"*.min.js",
				"*.bundle.js",
			},
		},
	}
}

// GetStrictnessPresets returns presets for different strictness levels
func GetStrictnessPresets() map[Strictness]StrictnessPreset {
	return map[Strictness]StrictnessPreset{
		StrictnessRelaxed: {
			LowThreshold:    15,
			MediumThreshold: 30,
			MaxComplexity:   0, // No limit
		},
		StrictnessStandard: {
			LowThreshold:    10,
			MediumThreshold: 20,
			MaxComplexity:   0, // No limit
		},
		StrictnessStrict: {
			LowThreshold:    5,
			MediumThreshold: 10,
			MaxComplexity:   15,
		},
	}
}

// GetFullConfigTemplate returns a full config template as valid JSON.
//
// Both templates emit only the keys the commands read, so that a generated file
// never promises a setting that changes nothing. See appliedKeys.
func GetFullConfigTemplate(projectType ProjectType, strictness Strictness) string {
	projectPresets := GetProjectPresets()
	strictnessPresets := GetStrictnessPresets()

	preset := projectPresets[projectType]
	strict := strictnessPresets[strictness]

	// Build include patterns string
	includePatterns := formatJSONArray(preset.IncludePatterns)
	excludePatterns := formatJSONArray(preset.ExcludePatterns)

	return `{
  "complexity": {
    "low_threshold": ` + strconv.Itoa(strict.LowThreshold) + `,
    "medium_threshold": ` + strconv.Itoa(strict.MediumThreshold) + `,
    "max_complexity": ` + strconv.Itoa(strict.MaxComplexity) + `,
    "report_unchanged": false
  },
  "dead_code": {
    "min_severity": "info",
    "sort_by": "severity"
  },
  "output": {
    "sort_by": "complexity",
    "min_complexity": 1
  },
  "analysis": {
    "include_patterns": ` + includePatterns + `,
    "exclude_patterns": ` + excludePatterns + `,
    "recursive": true
  }
}
`
}

// GetMinimalConfigTemplate returns a minimal config template as valid JSON.
func GetMinimalConfigTemplate() string {
	return `{
  "complexity": {
    "low_threshold": 10,
    "medium_threshold": 20
  },
  "dead_code": {
    "min_severity": "info"
  },
  "analysis": {
    "include_patterns": ["**/*.js", "**/*.ts", "**/*.jsx", "**/*.tsx"],
    "exclude_patterns": ["node_modules", "dist"]
  }
}
`
}

// formatJSONArray formats a string slice as a JSON array with proper indentation
func formatJSONArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}

	result := "[\n"
	for i, item := range items {
		result += `      "` + item + `"`
		if i < len(items)-1 {
			result += ","
		}
		result += "\n"
	}
	result += "    ]"
	return result
}

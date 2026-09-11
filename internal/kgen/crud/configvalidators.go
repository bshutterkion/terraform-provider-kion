package crud

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigValidatorsPath is the authored list of cross-field API constraints.
const ConfigValidatorsPath = "codegen/config_validators.yaml"

// A constraint the API enforces across two or more attributes, which an
// attribute-level schema cannot express: "required" is per-attribute, and
// neither of an either-or pair is required on its own.
//
// Authored rather than derived. The OpenAPI spec carries almost no constraints
// (3 enums, 16 patterns, 5 maxLength across 474 schemas), so the source is the
// portal's own `validate:"atLeastOneFieldPresent=..."` struct tags.
type ConfigValidator struct {
	Body         string   `yaml:"body"`
	AtLeastOneOf []string `yaml:"at_least_one_of"`
}

// ConfigValidatorPolicy answers what a package's resource must validate.
type ConfigValidatorPolicy struct {
	byPkg map[string]ConfigValidator
}

// LoadConfigValidators reads ConfigValidatorsPath. A missing file is an error
// rather than an empty policy: silently dropping every cross-field check
// because a file went missing would turn a plan-time diagnostic back into the
// API error it exists to replace, with nothing to indicate it had happened.
func LoadConfigValidators(root string) (ConfigValidatorPolicy, error) {
	path := filepath.Join(root, ConfigValidatorsPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return ConfigValidatorPolicy{}, fmt.Errorf("reading %s: %w", ConfigValidatorsPath, err)
	}
	var entries map[string]ConfigValidator
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		return ConfigValidatorPolicy{}, fmt.Errorf("parsing %s: %w", ConfigValidatorsPath, err)
	}
	for pkg, v := range entries {
		if len(v.AtLeastOneOf) < 2 {
			return ConfigValidatorPolicy{}, fmt.Errorf(
				"%s: %s.at_least_one_of needs at least two attributes, got %d",
				ConfigValidatorsPath, pkg, len(v.AtLeastOneOf))
		}
	}
	return ConfigValidatorPolicy{byPkg: entries}, nil
}

// For returns the attributes pkg must require at least one of, or nil.
func (p ConfigValidatorPolicy) For(pkg string) []string {
	return p.byPkg[pkg].AtLeastOneOf
}

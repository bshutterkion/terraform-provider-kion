package crud_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Every constraint declared in codegen/config_validators.yaml must reach the
// generated resource, and every emitted validator must be declared.
//
// The declaration is the reviewed artifact; the generated method is derived. A
// one-way check would let a regeneration silently drop a constraint and turn a
// plan-time diagnostic back into the API error it exists to replace -- which
// looks like nothing at all until someone writes a configuration missing both
// attributes.
func TestConfigValidatorsMatchDeclaration(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "codegen", "config_validators.yaml"))
	require.NoError(t, err)

	var declared map[string]struct {
		Body         string   `yaml:"body"`
		AtLeastOneOf []string `yaml:"at_least_one_of"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &declared))
	require.NotEmpty(t, declared)

	emitted := map[string][]string{}
	root := filepath.Join("..", "..", "service")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)

	// path.MatchRoot("x") inside a ConfigValidators body.
	matchRoot := regexp.MustCompile(`path\.MatchRoot\("([a-z0-9_]+)"\)`)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, err := filepath.Glob(filepath.Join(root, e.Name(), "*.go"))
		require.NoError(t, err)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			require.NoError(t, err)
			body := string(src)
			i := strings.Index(body, ") ConfigValidators(")
			if i < 0 {
				continue
			}
			var attrs []string
			for _, m := range matchRoot.FindAllStringSubmatch(body[i:], -1) {
				attrs = append(attrs, m[1])
			}
			sort.Strings(attrs)
			emitted[e.Name()] = attrs
		}
	}

	for pkg, d := range declared {
		want := append([]string(nil), d.AtLeastOneOf...)
		sort.Strings(want)
		got, ok := emitted[pkg]
		assert.Truef(t, ok,
			"%s declares at_least_one_of %v but its resource emits no ConfigValidators; "+
				"run `make crud-force`", pkg, want)
		if ok {
			assert.Equalf(t, want, got, "%s: declared and emitted constraints differ", pkg)
		}
	}

	for pkg := range emitted {
		_, ok := declared[pkg]
		assert.Truef(t, ok,
			"%s emits ConfigValidators with no entry in codegen/config_validators.yaml", pkg)
	}
}

// A validator naming an attribute the schema does not declare is a runtime
// panic, not a compile error -- and the API body's spelling is not always the
// provider's (kion_project_enforcement exposes user_group_ids where its two
// siblings expose ugroup_ids), so this is a real way to get it wrong.
func TestConfigValidatorAttributesExist(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "codegen", "config_validators.yaml"))
	require.NoError(t, err)

	var declared map[string]struct {
		AtLeastOneOf []string `yaml:"at_least_one_of"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &declared))

	attrDecl := regexp.MustCompile(`"([a-z0-9_]+)":\s*schema\.`)

	for pkg, d := range declared {
		matches, err := filepath.Glob(filepath.Join("..", "..", "service", pkg, "*_schema_gen.go"))
		require.NoError(t, err)
		require.NotEmptyf(t, matches, "%s: no schema file", pkg)

		src, err := os.ReadFile(matches[0])
		require.NoError(t, err)
		body := string(src)

		// Scope to the resource schema; the data source declares its own set.
		if i := strings.Index(body, "ResourceSchema(ctx context.Context) schema.Schema"); i >= 0 {
			body = body[i:]
		}
		attrs := map[string]bool{}
		for _, m := range attrDecl.FindAllStringSubmatch(body, -1) {
			attrs[m[1]] = true
		}
		for _, a := range d.AtLeastOneOf {
			assert.Truef(t, attrs[a],
				"%s: config_validators.yaml names %q, which its resource schema does not declare", pkg, a)
		}
	}
}

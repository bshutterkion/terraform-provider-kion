package crud

import (
	"bytes"
	"cmp"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed restest.gtpl
var resourceTestTmpl string

//go:embed dstest.gtpl
var dataSourceTestTmpl string

// testValues are the create/update sample attribute values for one resource.
type testValues struct {
	// EnvArgs names environment variables whose values the config needs; each
	// becomes an extra parameter, readable as %[2]s, %[3]s, ... in order.
	EnvArgs []string `yaml:"env_args"`
	// Prereqs is HCL emitted ahead of the resource under test, for resources
	// that cannot exist without a parent. It is rendered through the same
	// Sprintf as the rest of the config, so %[1]s is the random name.
	Prereqs string            `yaml:"prereqs"`
	Create  map[string]string `yaml:"create"`
	Update  map[string]string `yaml:"update"`
}

// hclPrefix marks a value as an HCL expression rather than a literal: it is
// emitted unquoted and asserted only for presence, since a reference like
// kion_project.test.id has no value until apply.
const hclPrefix = "hcl:"

type testValuesFile struct {
	Resources map[string]testValues `yaml:"resources"`
}

// loadTestValues reads codegen/test_values.yaml and returns the entry for one
// resource. A missing file or entry yields ok=false (caller skips test gen).
func loadTestValues(path, resource string) (testValues, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return testValues{}, false, nil
		}
		return testValues{}, false, fmt.Errorf("reading test values %s: %w", path, err)
	}
	var tvf testValuesFile
	if err := yaml.Unmarshal(raw, &tvf); err != nil {
		return testValues{}, false, fmt.Errorf("parsing test values %s: %w", path, err)
	}
	tv, ok := tvf.Resources[resource]
	return tv, ok, nil
}

type acctestAttr struct {
	Name  string
	Value string
	// Quoted is false for an attribute the schema types as a number or bool.
	// Every test value was a string until env_args arrived, so the template
	// quoted unconditionally and a numeric value came out as "3" -- which
	// Terraform coerces but `make ci-acctest-config` rejects.
	Quoted bool
	// CheckExpr is the Go expression for the value this attribute must hold in
	// state: a literal, or a Sprintf matching the one that built the config.
	// Terraform stores every primitive as a string, so numbers and bools are
	// compared as their unquoted text. Empty for an IsHCL attribute.
	CheckExpr string
	// IsHCL marks a value that is an expression to emit verbatim.
	IsHCL bool
}

type acctestData struct {
	Pkg, Pascal, ResourceType string
	SDKAlias                  string
	ReadMethod, ReadParams    string
	ReadIDParam               string
	IDParamType               string // "int64" | "uint64"
	AttrNames                 []string
	CreateAttrs               []acctestAttr
	UpdateAttrs               []acctestAttr
	HasUpdate                 bool
	BasicUsesRName            bool
	UpdateUsesRName           bool
	// Prereqs is HCL for the parent resources the resource under test needs,
	// emitted ahead of it in both the basic and update configurations.
	Prereqs string
	// EnvArgs are environment variables whose VALUES the config interpolates,
	// in order, as %[2]s, %[3]s, ... A value the API requires but the schema
	// marks optional (kion_category's payer_id) is install-specific, so it
	// cannot be a literal in test_values.yaml.
	EnvArgs []acctestEnvArg
}

func buildAcctestData(rm ResourceModel, tv testValues) (acctestData, error) {
	readIDParam, readIDType, err := idParamName(rm.Read.Params)
	if err != nil {
		return acctestData{}, fmt.Errorf("%s acctest: %w", rm.Name, err)
	}
	d := acctestData{
		Pkg:          rm.Name,
		Pascal:       rm.Pascal,
		ResourceType: "kion_" + rm.Name,
		SDKAlias:     "generated",
		ReadMethod:   rm.Read.Method.Name,
		ReadParams:   rm.Read.Method.ParamsType,
		ReadIDParam:  readIDParam,
		IDParamType:  readIDType,
	}
	d.CreateAttrs = sortAttrs(tv.Create, rm)
	d.UpdateAttrs = sortAttrs(tv.Update, rm)
	for _, a := range d.CreateAttrs {
		d.AttrNames = append(d.AttrNames, a.Name)
	}
	d.HasUpdate = rm.Update != nil && len(tv.Update) > 0
	d.EnvArgs = envArgsFor(tv.EnvArgs)
	setCheckExprs(d.CreateAttrs, d.EnvArgs)
	setCheckExprs(d.UpdateAttrs, d.EnvArgs)
	d.Prereqs = strings.TrimRight(tv.Prereqs, "\n")
	prereqVerb := strings.Contains(d.Prereqs, "%")
	d.BasicUsesRName = usesFormatVerb(d.CreateAttrs) || prereqVerb
	d.UpdateUsesRName = usesFormatVerb(d.UpdateAttrs) || prereqVerb
	return d, nil
}

func sortAttrs(m map[string]string, rm ResourceModel) []acctestAttr {
	out := make([]acctestAttr, 0, len(m))
	for k, v := range m {
		if expr, ok := strings.CutPrefix(v, hclPrefix); ok {
			out = append(out, acctestAttr{Name: k, Value: expr, IsHCL: true})
			continue
		}
		out = append(out, acctestAttr{Name: k, Value: v, Quoted: attrIsString(rm, k)})
	}
	slices.SortFunc(out, func(a, b acctestAttr) int { return cmp.Compare(a.Name, b.Name) })
	return out
}

// setCheckExprs fills in each attribute's CheckExpr. A value with no verb is
// its own literal; one with a verb is rebuilt by the same Sprintf the config
// used, so the assertion and the configuration cannot drift apart.
func setCheckExprs(attrs []acctestAttr, envArgs []acctestEnvArg) {
	for i := range attrs {
		if attrs[i].IsHCL {
			continue
		}
		attrs[i].CheckExpr = checkExpr(attrs[i].Value, envArgs)
	}
}

func checkExpr(value string, envArgs []acctestEnvArg) string {
	if !strings.Contains(value, "%") {
		return strconv.Quote(value)
	}
	args := []string{"rName"}
	for _, e := range envArgs {
		args = append(args, fmt.Sprintf("os.Getenv(%q)", e.Env))
	}
	// Passing more arguments than the verbs consume makes Sprintf emit
	// %!(EXTRA ...) and trips vet, so the list stops at the highest index used.
	if n := maxFormatIndex(value); n > 0 && n < len(args) {
		args = args[:n]
	}
	return fmt.Sprintf("fmt.Sprintf(%s, %s)", strconv.Quote(value), strings.Join(args, ", "))
}

// maxFormatIndex returns the highest N across the value's `%[N]` verbs, or 0
// when it uses none (unindexed verbs consume arguments in order instead).
func maxFormatIndex(value string) int {
	highest := 0
	for i := 0; i+1 < len(value); i++ {
		if value[i] != '%' || value[i+1] != '[' {
			continue
		}
		end := strings.IndexByte(value[i+2:], ']')
		if end < 0 {
			continue
		}
		n, err := strconv.Atoi(value[i+2 : i+2+end])
		if err == nil && n > highest {
			highest = n
		}
	}
	return highest
}

// usesFormatVerb reports whether any value contains a `%` verb, so the config
// function must fmt.Sprintf(..., rName) rather than return a raw literal.
func usesFormatVerb(attrs []acctestAttr) bool {
	for _, a := range attrs {
		if strings.Contains(a.Value, "%") {
			return true
		}
	}
	return false
}

func renderResourceTest(rm ResourceModel, tv testValues) ([]byte, error) {
	return renderTest("resourcetest", resourceTestTmpl, rm, tv)
}

func renderDataSourceTest(rm ResourceModel, tv testValues) ([]byte, error) {
	return renderTest("datasourcetest", dataSourceTestTmpl, rm, tv)
}

func renderTest(name, tmpl string, rm ResourceModel, tv testValues) ([]byte, error) {
	data, err := buildAcctestData(rm, tv)
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return nil, fmt.Errorf("parse %s template: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute %s template for %s: %w", name, rm.Name, err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated %s for %s: %w\n%s", name, rm.Name, err, buf.Bytes())
	}
	return src, nil
}

// acctestEnvArg pairs an environment variable with the Go parameter name the
// generated config function receives it as.
type acctestEnvArg struct {
	Env   string // e.g. KION_ACC_BILLING_SOURCE_ID
	Param string // e.g. billingSourceID
}

// envArgsFor derives parameter names from variable names. Computed here rather
// than in the template: the templates are rendered with no FuncMap.
func envArgsFor(envs []string) []acctestEnvArg {
	out := make([]acctestEnvArg, 0, len(envs))
	for _, e := range envs {
		parts := strings.Split(strings.ToLower(strings.TrimPrefix(e, "KION_ACC_")), "_")
		for i, p := range parts {
			switch {
			case p == "":
			case i == 0:
			case p == "id":
				parts[i] = "ID"
			default:
				parts[i] = strings.ToUpper(p[:1]) + p[1:]
			}
		}
		out = append(out, acctestEnvArg{Env: e, Param: strings.Join(parts, "")})
	}
	return out
}

// attrIsString reports whether the model types an attribute as a string, which
// is what decides whether the generated HCL quotes its value.
func attrIsString(rm ResourceModel, tfsdk string) bool {
	for _, f := range rm.Fields {
		if f.TFSDK == tfsdk {
			return f.Type == "types.String"
		}
	}
	// Unknown attributes keep the historical behavior.
	return true
}

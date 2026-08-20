package crud

import (
	"bytes"
	"cmp"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"slices"
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
	Create map[string]string `yaml:"create"`
	Update map[string]string `yaml:"update"`
}

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
	d.CreateAttrs = sortAttrs(tv.Create)
	d.UpdateAttrs = sortAttrs(tv.Update)
	for _, a := range d.CreateAttrs {
		d.AttrNames = append(d.AttrNames, a.Name)
	}
	d.HasUpdate = rm.Update != nil && len(tv.Update) > 0
	d.BasicUsesRName = usesFormatVerb(d.CreateAttrs)
	d.UpdateUsesRName = usesFormatVerb(d.UpdateAttrs)
	return d, nil
}

func sortAttrs(m map[string]string) []acctestAttr {
	out := make([]acctestAttr, 0, len(m))
	for k, v := range m {
		out = append(out, acctestAttr{Name: k, Value: v})
	}
	slices.SortFunc(out, func(a, b acctestAttr) int { return cmp.Compare(a.Name, b.Name) })
	return out
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

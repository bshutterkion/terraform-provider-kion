package crud

import (
	"bytes"
	"go/parser"
	"go/token"
	"testing"
)

func TestRenderSweep_labelReal(t *testing.T) {
	got, err := renderSweep(labelResourceModel(t))
	if err != nil {
		t.Fatalf("renderSweep: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sweep.go", got, parser.ParseComments); err != nil {
		t.Fatalf("generated sweep.go does not parse: %v\n%s", err, got)
	}
	wants := []string{
		`resource.AddTestSweepers("kion_label"`,
		"func sweepLabel(_ string) error",
		"conn.Client.GetLabelIndex(ctx, generated.GetLabelIndexParams{",
		"conn.Client.DeleteLabel(ctx, generated.DeleteLabelParams{ID: id})",
		"func sweepLabelMatch(item generated.Label) bool",
		`strings.HasPrefix(s, "test-acc")`,
	}
	for _, w := range wants {
		if !bytes.Contains(got, []byte(w)) {
			t.Errorf("real sweeper missing %q", w)
		}
	}
}

func TestRenderSweep_stubFallback(t *testing.T) {
	rm := labelResourceModel(t)
	rm.List = nil // no list endpoint → stub
	got, err := renderSweep(rm)
	if err != nil {
		t.Fatalf("renderSweep: %v", err)
	}
	if bytes.Contains(got, []byte("GetLabelIndex")) {
		t.Error("stub sweeper must not paginate")
	}
	if !bytes.Contains(got, []byte("func sweepLabel(_ string) error")) {
		t.Error("stub sweeper missing sweepLabel")
	}
}

package kimport

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func results() []Result {
	return []Result{
		{TFType: "kion_ou", Status: "ok", Records: []Record{
			{ID: "1", Name: "Root"}, {ID: "5", Name: "Central Services"},
		}},
		{TFType: "kion_aws_resource_tag", Status: "unsupported", Reason: "kind: no_read"},
	}
}

func TestRenderImportsEmitsABlockPerRecord(t *testing.T) {
	t.Parallel()
	out := RenderImports(results(), "1.0.0")
	assert.Equal(t, 2, strings.Count(out, "import {"))
	assert.Contains(t, out, `to = kion_ou.root`)
	assert.Contains(t, out, `id = "5"`)
}

func TestRenderImportsIncludesTheProviderRequirement(t *testing.T) {
	t.Parallel()
	out := RenderImports(results(), "1.0.0")
	assert.Contains(t, out, `source  = "kionsoftware/kion"`)
	assert.Contains(t, out, `version = "1.0.0"`)
}

func TestRenderImportsNotesTheGapsAsComments(t *testing.T) {
	t.Parallel()
	out := RenderImports(results(), "1.0.0")
	assert.Contains(t, out, "# kion_aws_resource_tag")
	assert.Contains(t, out, "kind: no_read")
}

func TestRenderImportsIsDeterministic(t *testing.T) {
	t.Parallel()
	assert.Equal(t, RenderImports(results(), "1.0.0"), RenderImports(results(), "1.0.0"))
}

// --- Correction 2: a successful read with skipped records must not render
// as clean. Results with status "ok" (or "empty") but a non-empty Reason
// carry a real gap -- e.g. records skipped for missing id, association
// records skipped for a missing key field, or a flat-list 405 fallback --
// and must not be silently dropped just because Status isn't unsupported/error.

func TestRenderImportsNotesCaveatsForOkResultsWithReason(t *testing.T) {
	t.Parallel()
	rs := []Result{
		{TFType: "kion_scope", Status: "ok", Records: []Record{{ID: "1", Name: "Prod"}},
			Reason: "2 record(s) skipped: no id"},
	}
	out := RenderImports(rs, "1.0.0")
	assert.Contains(t, out, "# Read with caveats:")
	assert.Contains(t, out, "kion_scope")
	assert.Contains(t, out, "2 record(s) skipped: no id")
}

func TestRenderImportsOmitsCaveatsWhenReasonEmpty(t *testing.T) {
	t.Parallel()
	rs := []Result{
		{TFType: "kion_ou", Status: "ok", Records: []Record{{ID: "1", Name: "Root"}}, Reason: ""},
	}
	out := RenderImports(rs, "1.0.0")
	assert.NotContains(t, out, "# Read with caveats:")
}

func TestRenderImportsCaveatsDoNotDuplicateNotImportedGaps(t *testing.T) {
	t.Parallel()
	// A result already reported under "# Not imported:" (unsupported/error)
	// must not also appear under "# Read with caveats:".
	out := RenderImports(results(), "1.0.0")
	assert.Contains(t, out, "# Not imported:")
	assert.NotContains(t, out, "# Read with caveats:")
}

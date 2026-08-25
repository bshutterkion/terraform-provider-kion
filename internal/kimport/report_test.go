package kimport

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"terraform-provider-kion/codegen"
)

func TestLoadEmbeddedManifest(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest(codegen.ImportManifestJSON)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(m.Resources), 68)
}

func TestReportCountsEachStatus(t *testing.T) {
	t.Parallel()
	out := FormatReport([]Result{
		{TFType: "kion_ou", Status: "ok", Records: make([]Record, 22)},
		{TFType: "kion_label", Status: "empty"},
		{TFType: "kion_aws_resource_tag", Status: "unsupported", Reason: "kind: no_read"},
		{TFType: "kion_dashboard", Status: "error", Reason: "GET /beta/dashboard: 500"},
	})
	assert.Contains(t, out, "4 resource types")
	assert.Contains(t, out, "22 records")
	assert.Contains(t, out, "ok: 1")
	assert.Contains(t, out, "error: 1")
}

func TestReportListsGapsWithReasons(t *testing.T) {
	t.Parallel()
	out := FormatReport([]Result{
		{TFType: "kion_ou", Status: "ok", Records: make([]Record, 3)},
		{TFType: "kion_dashboard", Status: "error", Reason: "GET /beta/dashboard: 500"},
	})
	assert.Contains(t, out, "kion_dashboard")
	assert.Contains(t, out, "500")
	assert.False(t, strings.Contains(out, "kion_ou "), "healthy kinds are not listed individually")
}

// TestReportListsCaveatsForOkResultWithReason covers correction 1: Enumerate
// records real gaps in Reason on results whose status is "ok" (records
// skipped for a missing id, association records skipped for a missing key,
// a flat list that 405'd and fell back to a parent-scoped read). Those never
// land in the "Not imported" section (status ok, not unsupported/error), so
// FormatReport must surface them in a separate section or a partial read
// looks clean.
func TestReportListsCaveatsForOkResultWithReason(t *testing.T) {
	t.Parallel()
	out := FormatReport([]Result{
		{TFType: "kion_ou", Status: "ok", Records: make([]Record, 3)},
		{
			TFType:  "kion_cloud_access_role",
			Status:  "ok",
			Records: make([]Record, 5),
			Reason:  "2 record(s) skipped: no id",
		},
	})
	assert.Contains(t, out, "Read with caveats")
	assert.Contains(t, out, "kion_cloud_access_role")
	assert.Contains(t, out, "2 record(s) skipped: no id")
	// The clean "ok" result with no Reason must not be listed individually.
	assert.False(t, strings.Contains(out, "kion_ou "), "healthy kinds are not listed individually")
}

// TestReportOmitsCaveatsForOkResultWithoutReason ensures a clean "ok" result
// (empty Reason) does not trigger the caveats section.
func TestReportOmitsCaveatsForOkResultWithoutReason(t *testing.T) {
	t.Parallel()
	out := FormatReport([]Result{
		{TFType: "kion_ou", Status: "ok", Records: make([]Record, 3)},
		{TFType: "kion_label", Status: "empty"},
	})
	assert.NotContains(t, out, "Read with caveats")
}

// TestReportCaveatsDoNotDuplicateGaps ensures a result already listed under
// "Not imported" (status unsupported/error) is not also listed in the
// caveats section, even though it carries a non-empty Reason.
func TestReportCaveatsDoNotDuplicateGaps(t *testing.T) {
	t.Parallel()
	out := FormatReport([]Result{
		{TFType: "kion_dashboard", Status: "error", Reason: "GET /beta/dashboard: 500"},
	})
	assert.Contains(t, out, "Not imported")
	assert.NotContains(t, out, "Read with caveats")
	// The dashboard reason should appear exactly once across the whole report.
	assert.Equal(t, 1, strings.Count(out, "GET /beta/dashboard: 500"))
}

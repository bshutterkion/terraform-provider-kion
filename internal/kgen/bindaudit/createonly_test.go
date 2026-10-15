package bindaudit_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"terraform-provider-kion/internal/kgen/bindaudit"
)

// A create_only attribute takes its value from the create response, so it is
// response-bound, not dropped; an attribute bound nowhere is still reported.
func TestCreateOnlyIsResponseBound(t *testing.T) {
	t.Parallel()
	findings, err := bindaudit.Run(filepath.Join("testdata", "createonly"))
	require.NoError(t, err)

	var keys []string
	for _, f := range findings {
		keys = append(keys, f.Key())
	}
	require.ElementsMatch(t, []string{"widget.note:create", "widget.note:update"}, keys)
}

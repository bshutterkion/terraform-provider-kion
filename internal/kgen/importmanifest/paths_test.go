package importmanifest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEveryParentPathHasAPlaceholder(t *testing.T) {
	t.Parallel()
	for tfType, p := range ParentPaths {
		assert.Contains(t, p.ChildPath, "{parent_id}", tfType)
		assert.True(t, strings.HasPrefix(p.ListPath, "/"), tfType)
		assert.NotEmpty(t, p.ParentIDField, tfType)
		assert.NotEmpty(t, p.Kind, tfType)
	}
}

func TestParentPathsCoverTheParentListArchetypes(t *testing.T) {
	t.Parallel()
	for _, tfType := range []string{
		"kion_ou_enforcement",
		"kion_project_enforcement",
		"kion_funding_source_enforcement",
		"kion_scope_criteria",
	} {
		assert.Contains(t, ParentPaths, tfType)
	}
}

func TestParentPathsCoverTheAssociationArchetypes(t *testing.T) {
	t.Parallel()
	for _, tfType := range []string{
		"kion_ou_permission_mapping",
		"kion_project_permission_mapping",
		"kion_funding_source_permission_mapping",
	} {
		assert.Contains(t, ParentPaths, tfType)
	}
}

func TestSpecialPathsAreAbsolute(t *testing.T) {
	t.Parallel()
	for tfType, p := range SpecialPaths {
		assert.True(t, strings.HasPrefix(p, "/"), tfType)
	}
}

func TestExtraListPathsCoverTheCodegenGaps(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"account", "aws_cloudformation_template", "aws_iam_policy"} {
		assert.Contains(t, ExtraListPaths, kind)
	}
}

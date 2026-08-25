package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"terraform-provider-kion/internal/kgen/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSourceOperations(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "openapi3.json")
	// Includes: normal verbs with operationIds, an "options" method (skipped by
	// the verb switch), and a post with an empty operationId (skipped).
	require.NoError(t, os.WriteFile(spec, []byte(`{
      "paths": {
        "/v3/label": {
          "post": {"operationId": "PostLabel"},
          "get":  {"operationId": "GetLabelList"}
        },
        "/v3/label/{id}": {
          "get":    {"operationId": "GetLabel"},
          "patch":  {"operationId": "PatchLabel"},
          "delete": {"operationId": "DeleteLabel"}
        },
        "/v3/ignored": {
          "options": {"operationId": "OptionsThing"},
          "post":    {"operationId": ""}
        }
      }
    }`), 0o600))

	src := config.NewFileSource()
	ops, err := src.Operations(spec)
	require.NoError(t, err)

	assert.Equal(t, config.Op{Method: "POST", Path: "/v3/label"}, ops["PostLabel"])
	assert.Equal(t, config.Op{Method: "GET", Path: "/v3/label/{id}"}, ops["GetLabel"])
	assert.Equal(t, config.Op{Method: "DELETE", Path: "/v3/label/{id}"}, ops["DeleteLabel"])
	// options verb + empty operationId are not indexed.
	assert.NotContains(t, ops, "OptionsThing")
	assert.NotContains(t, ops, "")
}

func TestFileSourceOperationsErrors(t *testing.T) {
	src := config.NewFileSource()

	t.Run("missing file", func(t *testing.T) {
		_, err := src.Operations(filepath.Join(t.TempDir(), "nope.json"))
		require.Error(t, err)
	})

	t.Run("malformed json", func(t *testing.T) {
		spec := filepath.Join(t.TempDir(), "bad.json")
		require.NoError(t, os.WriteFile(spec, []byte("{not json"), 0o600))
		_, err := src.Operations(spec)
		require.Error(t, err)
	})
}

func TestFileSourceServiceOps(t *testing.T) {
	root := t.TempDir()

	// label: a full resource + data source, parseable Go (need not typecheck).
	labelDir := filepath.Join(root, "label")
	require.NoError(t, os.MkdirAll(labelDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(labelDir, "label.go"), []byte(`package label

func (r *Resource) Create() { conn.PostLabel() }
func (r *Resource) Read()   { conn.GetLabel() }
func (r *Resource) Update() { conn.PatchLabel() }
func (r *Resource) Delete() { conn.DeleteLabel() }
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(labelDir, "label_data_source.go"), []byte(`package label

func (d *DataSource) Read() { conn.GetLabel() }
`), 0o600))

	// empty: a service dir with no matching <name>.go. ParseGo returns nil, so
	// the CRUD slices stay empty (exercises the nil-file branch).
	require.NoError(t, os.MkdirAll(filepath.Join(root, "empty"), 0o755))

	// A non-directory entry at the root is skipped.
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("x"), 0o600))

	src := config.NewFileSource()
	got, err := src.ServiceOps(root)
	require.NoError(t, err)

	byName := map[string]config.ServiceOps{}
	for _, so := range got {
		byName[so.Name] = so
	}

	require.Contains(t, byName, "label")
	assert.Equal(t, []string{"PostLabel"}, byName["label"].Create)
	assert.Equal(t, []string{"GetLabel"}, byName["label"].Read)
	assert.Equal(t, []string{"PatchLabel"}, byName["label"].Update)
	assert.Equal(t, []string{"DeleteLabel"}, byName["label"].Delete)
	assert.Equal(t, []string{"GetLabel"}, byName["label"].DataSourceRead)

	require.Contains(t, byName, "empty")
	assert.Empty(t, byName["empty"].Create)
	assert.Empty(t, byName["empty"].DataSourceRead)

	assert.NotContains(t, byName, "README.md")
}

func TestFileSourceServiceOpsError(t *testing.T) {
	src := config.NewFileSource()
	_, err := src.ServiceOps(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}

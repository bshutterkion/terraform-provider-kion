package crud

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"terraform-provider-kion/internal/kgen/kfs"
)

// Companion tests are emitted with nil data; each must render against the
// default companion data, or generation silently skips the resource.
func TestCompanionTestsRenderWithNilData(t *testing.T) {
	sdkVersion = "v3_17"
	for name, files := range companionTestsByName {
		g := &generator{fs: kfs.OS{}}
		_, err := g.emitBespoke(t.TempDir(), name, nil, files, true)
		require.NoError(t, err, name)
	}
}

func TestEmitBespokeFailsOnBrokenTemplate(t *testing.T) {
	sdkVersion = "v3_17"
	g := &generator{fs: kfs.OS{}}
	_, err := g.emitBespoke(t.TempDir(), "thing", nil,
		[]bespokeFile{{tmpl: "package x\n{{.NoSuchField}}", outName: "x.go"}}, true)
	require.Error(t, err)
}

func TestFailuresNameEveryResource(t *testing.T) {
	g := &generator{}
	require.NoError(t, g.failureError())
	g.recordFailure("alpha", errors.New("boom"))
	g.recordFailure("beta", errors.New("bang"))
	err := g.failureError()
	require.Error(t, err)
	require.Contains(t, err.Error(), "alpha")
	require.Contains(t, err.Error(), "boom")
	require.Contains(t, err.Error(), "beta")
}

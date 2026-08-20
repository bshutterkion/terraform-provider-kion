package conns

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSharedClient_MissingURL(t *testing.T) {
	t.Setenv("KION_API_URL", "")
	t.Setenv("KION_API_KEY", "k")
	t.Setenv("KION_AUTH_TOKEN", "")

	_, err := buildSharedClient()
	require.Error(t, err)
	require.ErrorContains(t, err, "KION_API_URL")
}

func TestBuildSharedClient_MissingCredentials(t *testing.T) {
	t.Setenv("KION_API_URL", "https://kion.example.com")
	t.Setenv("KION_API_KEY", "")
	t.Setenv("KION_AUTH_TOKEN", "")

	_, err := buildSharedClient()
	require.Error(t, err)
	require.ErrorContains(t, err, "KION_API_KEY or KION_AUTH_TOKEN")
}

func TestBuildSharedClient_WithAPIKey(t *testing.T) {
	t.Setenv("KION_API_URL", "https://kion.example.com")
	t.Setenv("KION_API_KEY", "secret")
	t.Setenv("KION_AUTH_TOKEN", "")

	c, err := buildSharedClient()
	require.NoError(t, err)
	require.NotNil(t, c)
	// The API root, matching what the provider's Configure stores, so the raw
	// helpers and DetectVersion resolve the same way here as in production.
	require.Equal(t, "https://kion.example.com/api", c.APIURL)
}

func TestBuildSharedClient_WithAuthTokenAndSkipSSL(t *testing.T) {
	t.Setenv("KION_API_URL", "https://kion.example.com")
	t.Setenv("KION_API_KEY", "")
	t.Setenv("KION_AUTH_TOKEN", "tok")
	t.Setenv("KION_SKIP_SSL_VALIDATION", "1")

	c, err := buildSharedClient()
	require.NoError(t, err)
	require.NotNil(t, c)
}

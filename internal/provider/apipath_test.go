package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"terraform-provider-kion/internal/conns"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

// The apipath bug lived in the seam between resolveProviderConfig and the raw
// HTTP helpers, so neither package's own tests could see it: resolveProviderConfig
// correctly folded apipath into serverURL, Configure handed that to
// KionClient.APIURL, and the raw helpers then appended a hardcoded "/api" of
// their own — requesting /api/api/v1/payer/3 under a default configuration and
// ignoring apipath entirely. This test wires the two halves together the way
// Configure does and asserts on the path the server actually receives.
func TestRawHelpersHonorResolvedAPIPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		apipath  types.String
		wantPath string
	}{
		{"default", types.StringNull(), "/api/v1/payer/3"},
		{"emptied", types.StringValue(""), "/v1/payer/3"},
		{"custom", types.StringValue("/custom"), "/custom/v1/payer/3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				if _, werr := w.Write([]byte(`{}`)); werr != nil {
					t.Errorf("writing response: %v", werr)
				}
			}))
			defer srv.Close()

			resolved, diags := resolveProviderConfig(kionProviderModel{
				APIURL:  types.StringValue(srv.URL),
				APIKey:  types.StringValue("k"),
				Apipath: tt.apipath,
			}, env(nil))
			require.False(t, diags.HasError(), "unexpected diagnostics: %v", diags)

			// Built exactly as Configure builds it.
			c := &conns.KionClient{APIURL: resolved.serverURL, APIKey: "k", HTTPClient: srv.Client()}
			_, err := c.RawGet(t.Context(), "/v1/payer/3")
			require.NoError(t, err)
			require.Equal(t, tt.wantPath, gotPath)
		})
	}
}

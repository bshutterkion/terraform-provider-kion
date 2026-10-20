package conns_test

import (
	"net/http"
	"testing"

	"terraform-provider-kion/internal/conns"
	"terraform-provider-kion/internal/conns/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func listClient(t *testing.T, status int, body string) *conns.KionClient {
	t.Helper()
	doer := mocks.NewMockHTTPDoer(t)
	doer.EXPECT().Do(mock.Anything).Return(respJSON(status, body), nil)
	return &conns.KionClient{APIURL: "https://kion.example.com", HTTPClient: doer, APIKey: "k"}
}

func TestRawIDListed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		id         int64
		want       bool
	}{
		{"flat list, present", `{"data":[{"id":1},{"id":9}]}`, 9, true},
		{"flat list, absent", `{"data":[{"id":1},{"id":9}]}`, 10, false},
		{"paged items, present", `{"data":{"total":1,"items":[{"id":10}]}}`, 10, true},
		{"empty list", `{"data":[]}`, 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := listClient(t, http.StatusOK, tc.body).RawIDListed(t.Context(), "/v3/app-role", tc.id)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// A failed or unreadable list is an error, never "absent": guessing absent would
// drop a live record from state.
func TestRawIDListed_errorsAreNotAbsence(t *testing.T) {
	t.Parallel()
	_, err := listClient(t, http.StatusInternalServerError, `{}`).RawIDListed(t.Context(), "/v3/app-role", 1)
	assert.Error(t, err)
	_, err = listClient(t, http.StatusOK, `{"data":"nope"}`).RawIDListed(t.Context(), "/v3/app-role", 1)
	assert.Error(t, err)
}

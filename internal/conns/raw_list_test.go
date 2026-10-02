package conns_test

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"terraform-provider-kion/internal/conns"
	"terraform-provider-kion/internal/conns/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// listServer serves ids 1..total in the {data:{total,items}} envelope, in
// pages of whatever count is asked for, recording each request.
func listServer(t *testing.T, total, reportTotal int) (*mocks.MockHTTPDoer, *[]string) {
	t.Helper()
	doer := mocks.NewMockHTTPDoer(t)
	var seen []string
	doer.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		seen = append(seen, req.URL.RequestURI())
		q := req.URL.Query()
		page, err := strconv.Atoi(q.Get("page"))
		require.NoError(t, err)
		count, err := strconv.Atoi(q.Get("count"))
		require.NoError(t, err)
		var items []string
		for i := (page-1)*count + 1; i <= page*count && i <= total; i++ {
			items = append(items, fmt.Sprintf(`{"id":%d}`, i))
		}
		return respJSON(http.StatusOK, fmt.Sprintf(`{"status":200,"data":{"total":%d,"items":[%s]}}`,
			reportTotal, strings.Join(items, ","))), nil
	}).Maybe()
	return doer, &seen
}

func TestRawListContainsID_FindsRecordOnALaterPage(t *testing.T) {
	t.Parallel()
	doer, seen := listServer(t, 150, 150)
	c := &conns.KionClient{APIURL: "https://kion.example.com/api", HTTPClient: doer, APIKey: "k"}
	ok, err := c.RawListContainsID(t.Context(), "/v4/idms/open-id/3/group-association", "142")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []string{
		"/api/v4/idms/open-id/3/group-association?count=100&page=1",
		"/api/v4/idms/open-id/3/group-association?count=100&page=2",
	}, *seen)
}

func TestRawListContainsID_AbsentFromACompleteList(t *testing.T) {
	t.Parallel()
	doer, _ := listServer(t, 3, 3)
	c := &conns.KionClient{APIURL: "https://kion.example.com", HTTPClient: doer, APIKey: "k"}
	ok, err := c.RawListContainsID(t.Context(), "/v4/x/1/y", "8")
	require.NoError(t, err)
	require.False(t, ok)
}

// A walk that stops short of the reported total proves nothing, and calling a
// live record gone would plan its recreation, so it counts as listed.
func TestRawListContainsID_IncompleteWalkIsInconclusive(t *testing.T) {
	t.Parallel()
	doer, _ := listServer(t, 3, 500)
	c := &conns.KionClient{APIURL: "https://kion.example.com", HTTPClient: doer, APIKey: "k"}
	ok, err := c.RawListContainsID(t.Context(), "/v4/x/1/y", "8")
	require.NoError(t, err)
	require.True(t, ok)
}

// The parent's own 404 means the record went with it.
func TestRawListContainsID_ParentNotFound(t *testing.T) {
	t.Parallel()
	doer := mocks.NewMockHTTPDoer(t)
	doer.EXPECT().Do(mock.Anything).Return(respJSON(http.StatusNotFound, `{"message":"IDMS not found"}`), nil)
	c := &conns.KionClient{APIURL: "https://kion.example.com", HTTPClient: doer, APIKey: "k"}
	ok, err := c.RawListContainsID(t.Context(), "/v4/x/1/y", "8")
	require.NoError(t, err)
	require.False(t, ok)
}

// A body without the paginated envelope must not read as an empty list.
func TestRawListContainsID_RejectsUnexpectedShape(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"status":200}`, `{"data":{"items":[]}}`, `[]`} {
		doer := mocks.NewMockHTTPDoer(t)
		doer.EXPECT().Do(mock.Anything).Return(respJSON(http.StatusOK, body), nil)
		c := &conns.KionClient{APIURL: "https://kion.example.com", HTTPClient: doer, APIKey: "k"}
		_, err := c.RawListContainsID(t.Context(), "/v4/x/1/y", "8")
		require.Error(t, err, body)
	}
}

func TestRawListContainsID_OtherErrorsPassThrough(t *testing.T) {
	t.Parallel()
	doer := mocks.NewMockHTTPDoer(t)
	doer.EXPECT().Do(mock.Anything).Return(respJSON(http.StatusForbidden, `{}`), nil)
	c := &conns.KionClient{APIURL: "https://kion.example.com", HTTPClient: doer, APIKey: "k"}
	_, err := c.RawListContainsID(t.Context(), "/v4/x/1/y", "8")
	require.Error(t, err)
}

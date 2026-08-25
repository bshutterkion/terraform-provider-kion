package kimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListBareArray(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[{"id":1},{"id":2}]`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListDataEnvelope(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"status":200,"data":[{"id":1}]}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

// TestListNamedCollectionEnvelope is List's end-to-end counterpart to
// TestUnwrapNamedCollectionEnvelope: a real /v1/dashboards response --
// {"status":200,"data":{"dashboards":[...],"hidden_dashboard_count":0}}, with
// a sibling scalar key alongside the array -- must come back as records
// through the full HTTP round trip, not just the pure unwrap helper.
func TestListNamedCollectionEnvelope(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"status":200,"data":{"dashboards":[{"id":1},{"id":2},{"id":3},{"id":4}],"hidden_dashboard_count":0}}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v1/dashboards")
	require.NoError(t, err)
	assert.Len(t, got, 4)
}

func TestListPaginatesItemsEnvelope(t *testing.T) {
	t.Parallel()
	var pages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"items":[{"id":1},{"id":2}],"total":3}`)
			return
		}
		fmt.Fprint(w, `{"items":[{"id":3}],"total":3}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.NoError(t, err)
	assert.Len(t, got, 3)
	assert.Equal(t, 2, pages)
}

func TestListStopsOnEmptyPage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"items":[{"id":1}],"total":99}`)
			return
		}
		fmt.Fprint(w, `{"items":[],"total":99}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestListSendsBearerAuth(t *testing.T) {
	t.Parallel()
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "secret", false, "").List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret", auth)
}

func TestListErrorsOnNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v4/compliance/family")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "405")
}

// --- StatusError: a non-2xx response must be recoverable by errors.As, with
// the same message text the old fmt.Errorf calls produced. ---

func TestListErrorsOnNon2xxIsStatusError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"status":404,"message":"IDMS not found"}`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v4/idms/open-id/1/access-rule")
	require.Error(t, err)

	var statusErr *StatusError
	require.True(t, errors.As(err, &statusErr), "expected a *StatusError, got %T: %v", err, err)
	assert.Equal(t, http.StatusNotFound, statusErr.Status)
	assert.Equal(t,
		`GET /v4/idms/open-id/1/access-rule: 404 Not Found: {"status":404,"message":"IDMS not found"}`,
		err.Error(),
	)
}

func TestListErrorsOnNon2xxNoBodyStillMatchesOldMessageShape(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v4/compliance/family")
	require.Error(t, err)

	var statusErr *StatusError
	require.True(t, errors.As(err, &statusErr))
	assert.Equal(t, http.StatusMethodNotAllowed, statusErr.Status)
	assert.Equal(t, "GET /v4/compliance/family: 405 Method Not Allowed", err.Error())
}

func TestListPage2TransportErrorIsStatusErrorThroughPagingWrap(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"items":[{"id":1},{"id":2}],"total":5}`)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.Error(t, err)
	assert.Len(t, got, 2) // page 1 records are returned despite page 2 error

	var statusErr *StatusError
	require.True(t, errors.As(err, &statusErr), "expected a *StatusError through the paging wrap, got %T: %v", err, err)
	assert.Equal(t, http.StatusBadGateway, statusErr.Status)
}

func TestListPage2TransportError(t *testing.T) {
	t.Parallel()
	var pageCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"items":[{"id":1},{"id":2}],"total":5}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"server error"}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.Error(t, err)
	assert.Len(t, got, 2) // page 1 records are returned despite page 2 error
	assert.Contains(t, err.Error(), "500")
}

func TestListPage2UnwrapError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"items":[{"id":1},{"id":2}],"total":5}`)
			return
		}
		// Page 2 returns bare JSON that cannot be unwrapped
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `123`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.Error(t, err)
	assert.Len(t, got, 2) // page 1 records are returned despite page 2 unwrap error
}

func TestListExceedsMaxPages(t *testing.T) {
	t.Parallel()
	var pageCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++
		// Always return one record with a huge total to force many page requests
		fmt.Fprint(w, `{"items":[{"id":1}],"total":999999}`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max pages")
	// Should have hit the cap and stopped
	assert.Less(t, pageCount, 2000)
}

func TestListNullItemsEnvelope(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"items":null,"total":0}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

func TestListEmptyItemsEnvelope(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"items":[],"total":0}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

// paddedComplianceHandler mimics /v4/compliance/program/{id}/control: every
// page answers with an items array of length total, carrying only the
// requested page's window of real records and zero-valued filler everywhere
// else.
func paddedComplianceHandler(t *testing.T, total int, pagesSeen *int) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		*pagesSeen++
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 {
			page = 1
		}
		start, end := (page-1)*pageSize, page*pageSize
		items := make([]map[string]any, 0, total)
		for i := range total {
			if i >= start && i < end {
				items = append(items, map[string]any{"id": i + 1, "name": fmt.Sprintf("c%d", i+1), "compliance_levels": []int{}})
				continue
			}
			items = append(items, map[string]any{"id": 0, "name": "", "compliance_levels": nil})
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"data":   map[string]any{"pagination": map[string]any{"page": page}, "total": total, "items": items},
		}))
	}
}

// TestListDropsZeroValuedPagePadding is the live shape from issue #32: the
// compliance parent-scoped collections pre-size items to total and populate
// only the page window, so page 1 alone looked complete and every later
// record was lost as "no id".
func TestListDropsZeroValuedPagePadding(t *testing.T) {
	t.Parallel()
	const total = 250
	var pages int
	srv := httptest.NewServer(paddedComplianceHandler(t, total, &pages))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v4/compliance/program/12/control")
	require.NoError(t, err)
	require.Len(t, got, total)
	assert.Equal(t, 3, pages)
	for i, rec := range got {
		assert.Equal(t, float64(i+1), rec["id"], "record %d", i)
	}
}

// TestListLeavesWellBehavedPagesUntouched guards the filter's blast radius:
// an ordinary paginated endpoint whose pages are at most pageSize long must
// not be inspected for padding at all.
func TestListLeavesWellBehavedPagesUntouched(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"items":[{"id":1},{"id":0,"name":""}],"total":3}`)
			return
		}
		fmt.Fprint(w, `{"items":[{"id":3}],"total":3}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/beta/scope")
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

// TestListUnpaddedOversizedPageIsNotFiltered covers an endpoint that ignores
// count and returns every record on page 1: longer than pageSize, but all
// real, so nothing may be dropped and no second page is fetched.
func TestListUnpaddedOversizedPageIsNotFiltered(t *testing.T) {
	t.Parallel()
	const total = pageSize + 5
	var pages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pages++
		items := make([]map[string]any, 0, total)
		for i := range total {
			items = append(items, map[string]any{"id": i + 1})
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"items": items, "total": total}))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Len(t, got, total)
	assert.Equal(t, 1, pages)
}

func TestDropPagePadding(t *testing.T) {
	t.Parallel()
	oversized := func(populated int) []map[string]any {
		out := make([]map[string]any, 0, pageSize+10)
		for i := range pageSize + 10 {
			if i < populated {
				out = append(out, map[string]any{"id": float64(i + 1)})
				continue
			}
			out = append(out, map[string]any{"id": float64(0), "name": ""})
		}
		return out
	}
	tests := []struct {
		name    string
		records []map[string]any
		total   int
		want    int
	}{
		{"unpaginated is untouched", oversized(3), -1, pageSize + 10},
		{"at or under the page size is untouched", []map[string]any{{"id": float64(0)}}, 99, 1},
		{"padding past the page window is dropped", oversized(pageSize), pageSize + 10, pageSize},
		{"all real records survive", oversized(pageSize + 10), pageSize + 10, pageSize + 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Len(t, dropPagePadding(tt.records, tt.total), tt.want)
		})
	}
}

func TestIsZeroRecord(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rec  map[string]any
		want bool
	}{
		{"empty object", map[string]any{}, true},
		{"live compliance padding slot", map[string]any{
			"id": float64(0), "compliance_family_id": float64(0), "control_number": float64(0),
			"name": "", "description": "", "severity": "", "title": "",
			"compliance_levels": nil, "cloud_provider_policy_ids": nil,
		}, true},
		{"nested all-zero object", map[string]any{"id": float64(0), "meta": map[string]any{"x": ""}}, true},
		{"empty array", map[string]any{"ids": []any{}}, true},
		{"nonzero id", map[string]any{"id": float64(3), "name": ""}, false},
		{"nonempty name only", map[string]any{"id": float64(0), "name": "x"}, false},
		{"true bool", map[string]any{"enabled": true}, false},
		{"populated array", map[string]any{"ids": []any{float64(1)}}, false},
		{"nested nonzero object", map[string]any{"meta": map[string]any{"x": float64(1)}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, isZeroRecord(tt.rec))
		})
	}
}

// TestUnwrapNestedDataEnvelope covers the doubly-nested envelope shape seen
// on real installs: {"status":200,"data":{"pagination":{...},"total":N,"items":[...]}}.
// unwrap must recurse into the inner envelope rather than returning it as one
// bogus record with no id.
func TestUnwrapNestedDataEnvelope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		body        string
		wantRecords int
		wantTotal   int
	}{
		{
			name:        "nested envelope with populated items",
			body:        `{"status":200,"data":{"pagination":{"augmented":false,"page":1,"count":2,"sort_method":"","sort_order":""},"total":17,"items":[{"id":2,"account_creation":true},{"id":5}]}}`,
			wantRecords: 2,
			wantTotal:   17,
		},
		{
			name:        "nested envelope with empty items array",
			body:        `{"status":200,"data":{"pagination":{"augmented":false,"page":1,"count":0,"sort_method":"","sort_order":""},"total":0,"items":[]}}`,
			wantRecords: 0,
			wantTotal:   0,
		},
		{
			name:        "nested envelope with null items",
			body:        `{"status":200,"data":{"pagination":{"augmented":false,"page":1,"count":0,"sort_method":"","sort_order":""},"total":0,"items":null}}`,
			wantRecords: 0,
			wantTotal:   0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			records, total, err := unwrap(json.RawMessage(tt.body))
			require.NoError(t, err)
			assert.Len(t, records, tt.wantRecords)
			assert.Equal(t, tt.wantTotal, total)
		})
	}
}

// TestUnwrapGenuineSingleObjectDataStillOneRecord is a regression guard: a
// real single-object {"data":{...}} response (no "items" key inside) must
// still come back as one record, not get swept up by the nested-envelope fix.
func TestUnwrapGenuineSingleObjectDataStillOneRecord(t *testing.T) {
	t.Parallel()
	records, total, err := unwrap(json.RawMessage(`{"status":200,"data":{"id":1,"name":"x"}}`))
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, float64(1), records[0]["id"])
	assert.Equal(t, "x", records[0]["name"])
	assert.Equal(t, -1, total)
}

// TestUnwrapNamedCollectionEnvelope covers /v1/dashboards' real shape:
// {"status":200,"data":{"dashboards":[{...}],"hidden_dashboard_count":0}} --
// an array-valued key plus a sibling scalar. unwrap must recognize data as
// having exactly one array-valued key and use that array as the records,
// ignoring the scalar sibling, the same way it already recurses into an
// {"items":[...]} nested envelope.
func TestUnwrapNamedCollectionEnvelope(t *testing.T) {
	t.Parallel()
	records, total, err := unwrap(json.RawMessage(`{"status":200,"data":{"dashboards":[{"id":1,"name":"a"},{"id":2,"name":"b"}],"hidden_dashboard_count":0}}`))
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, float64(1), records[0]["id"])
	assert.Equal(t, "a", records[0]["name"])
	assert.Equal(t, float64(2), records[1]["id"])
	assert.Equal(t, -1, total)
}

// TestUnwrapNamedCollectionEnvelopeWithSeveralScalarSiblings extends the
// real-shape case to several sibling scalar/object keys, not just one --
// exactly one array-valued key must still be enough to identify the records,
// regardless of how many non-array keys ride alongside it.
func TestUnwrapNamedCollectionEnvelopeWithSeveralScalarSiblings(t *testing.T) {
	t.Parallel()
	records, total, err := unwrap(json.RawMessage(`{"status":200,"data":{"dashboards":[{"id":1}],"hidden_dashboard_count":0,"page":1,"meta":{"note":"x"}}}`))
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, float64(1), records[0]["id"])
	assert.Equal(t, -1, total)
}

// TestUnwrapNamedCollectionEnvelopeEmptyArray covers the empty-list case of
// the same shape: {"data":{"dashboards":[],"hidden_dashboard_count":0}} must
// yield zero records, not an error and not one bogus record.
func TestUnwrapNamedCollectionEnvelopeEmptyArray(t *testing.T) {
	t.Parallel()
	records, _, err := unwrap(json.RawMessage(`{"status":200,"data":{"dashboards":[],"hidden_dashboard_count":0}}`))
	require.NoError(t, err)
	assert.Len(t, records, 0)
}

// TestUnwrapTwoArrayValuedKeysFallsThroughUnchanged is a regression guard:
// two (or more) ARRAY-VALUED keys under data must NOT trigger the
// named-collection shortcut -- it isn't unambiguous which array is the
// record list, so this must fall through to the existing singleton-object
// behavior instead of picking one arbitrarily. A sibling scalar key does not
// count towards this -- see TestUnwrapNamedCollectionEnvelope.
func TestUnwrapTwoArrayValuedKeysFallsThroughUnchanged(t *testing.T) {
	t.Parallel()
	records, total, err := unwrap(json.RawMessage(`{"status":200,"data":{"dashboards":[{"id":1}],"favorites":[{"id":2}]}}`))
	require.NoError(t, err)
	require.Len(t, records, 1)
	// Falls through to the whole data object as one record, not either inner array.
	assert.Contains(t, records[0], "dashboards")
	assert.Contains(t, records[0], "favorites")
	assert.Equal(t, -1, total)
}

// TestUnwrapSingleKeyNonArrayValueFallsThroughUnchanged is a regression
// guard: a data object with zero array-valued keys (e.g. {"data":{"id":1}})
// must not be treated as a named-collection envelope -- it must still come
// back as one record via the existing singleton branch.
func TestUnwrapSingleKeyNonArrayValueFallsThroughUnchanged(t *testing.T) {
	t.Parallel()
	records, total, err := unwrap(json.RawMessage(`{"status":200,"data":{"id":1}}`))
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, float64(1), records[0]["id"])
	assert.Equal(t, -1, total)
}

func TestListErrorIncludesBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message":"invalid request parameter"}`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v3/ou")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request parameter")
}

// TestSnippetOfDoesNotSplitAMultiByteRuneAtTheLimit: small item #2. Byte-
// slicing buf[:bodyLimit] can land in the middle of a multi-byte UTF-8 rune,
// emitting invalid UTF-8 into a customer-facing error message. 511 ASCII
// bytes plus one 3-byte rune ("世", E4 B8 96) is 514 bytes total, so
// truncating to bodyLimit (512) keeps the rune's leading byte but drops its
// two continuation bytes -- exactly the split this guards against.
//
// A bare utf8.ValidString(got) check does NOT discriminate here: the
// strings.Map call further down snippetOf decodes any invalid byte sequence
// as the U+FFFD replacement rune and re-emits it verbatim, which is itself
// valid UTF-8 -- so the output is always valid UTF-8 whether or not
// strings.ToValidUTF8 ran first. The actual difference the fix makes is in
// the *content*: ToValidUTF8(..., "") drops the truncated partial rune
// entirely, while the unfixed path leaves a U+FFFD in its place. Assert the
// exact expected snippet (and the absence of U+FFFD) so the test fails
// against the unfixed code.
func TestSnippetOfDoesNotSplitAMultiByteRuneAtTheLimit(t *testing.T) {
	t.Parallel()
	buf := append([]byte(strings.Repeat("a", bodyLimit-1)), "世"...)
	require.Greater(t, len(buf), bodyLimit, "test setup: buf must exceed bodyLimit to exercise truncation")

	got := snippetOf(buf)
	assert.True(t, utf8.ValidString(got), "snippetOf must never return invalid UTF-8: %q", got)
	assert.NotContains(t, got, string(utf8.RuneError), "snippetOf must not leave a replacement character from a split rune: %q", got)
	assert.Equal(t, strings.Repeat("a", bodyLimit-1)+"…", got)
}

// TestJoinAPIPrefix covers the pure URL-normalization helper: default prefix
// applied, empty prefix omitting it, no doubling when the base URL already
// ends with the prefix, and tolerance for a prefix given with or without its
// leading/trailing slash.
func TestJoinAPIPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		baseURL string
		prefix  string
		want    string
	}{
		{"default appended", "https://kion.example.com", "/api", "https://kion.example.com/api"},
		{"empty prefix omits it", "https://kion.example.com", "", "https://kion.example.com"},
		{"empty prefix still trims trailing slash on base", "https://kion.example.com/", "", "https://kion.example.com"},
		{"no doubling when URL already ends in prefix", "https://kion.example.com/api", "/api", "https://kion.example.com/api"},
		{"no doubling with trailing slash on base", "https://kion.example.com/api/", "/api", "https://kion.example.com/api"},
		{"prefix without leading slash", "https://kion.example.com", "api", "https://kion.example.com/api"},
		{"prefix with leading and trailing slash", "https://kion.example.com", "/api/", "https://kion.example.com/api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, joinAPIPrefix(tt.baseURL, tt.prefix))
		})
	}
}

// TestNewClientAppliesAPIPrefixToRequests confirms the prefix actually reaches
// the wire, not just the pure helper: --url https://host with the default
// --api-prefix /api must request /api/v3/ou, not /v3/ou.
func TestNewClientAppliesAPIPrefixToRequests(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "/api").List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Equal(t, "/api/v3/ou", gotPath)
}

// TestNewClientEmptyAPIPrefixOmitsIt covers the --api-prefix "" escape hatch
// for an app hit directly (e.g. localhost), which serves the API at the root.
func TestNewClientEmptyAPIPrefixOmitsIt(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Equal(t, "/v3/ou", gotPath)
}

// TestNewClientAPIPrefixNoDoublingWhenURLAlreadyHasSuffix covers --url
// https://host/api with the default --api-prefix /api: the request must hit
// /api/v3/ou, not /api/api/v3/ou.
func TestNewClientAPIPrefixNoDoublingWhenURLAlreadyHasSuffix(t *testing.T) {
	t.Parallel()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL+"/api", "k", false, "/api").List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Equal(t, "/api/v3/ou", gotPath)
}

// TestGetHTMLBodyProducesSpecificNonJSONError covers the case that motivated
// the /api-prefix work: hitting the bare host returns the web app's HTML
// shell with HTTP 200, so the failure only shows up as a JSON decode error.
// That must be replaced with a specific message naming the likely cause
// instead of a raw/opaque unmarshal error.
func TestGetHTMLBodyProducesSpecificNonJSONError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<!DOCTYPE html>\n<html><head><title>Kion</title></head><body>app shell</body></html>")
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v3/ou")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTML")
	assert.Contains(t, err.Error(), "--api-prefix")
	assert.NotContains(t, err.Error(), "invalid character")
}

// TestGetHTMLBodyWithLeadingWhitespaceStillDetected is a regression guard for
// looksLikeHTML's whitespace tolerance.
func TestGetHTMLBodyWithLeadingWhitespaceStillDetected(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "\n\n  <html><body>app shell</body></html>")
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false, "").List(context.Background(), "/v3/ou")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTML")
}

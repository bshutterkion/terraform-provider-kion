package kimport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListBareArray(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[{"id":1},{"id":2}]`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false).List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListDataEnvelope(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"status":200,"data":[{"id":1}]}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false).List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Len(t, got, 1)
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

	got, err := NewClient(srv.URL, "k", false).List(context.Background(), "/beta/scope")
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

	got, err := NewClient(srv.URL, "k", false).List(context.Background(), "/beta/scope")
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

	_, err := NewClient(srv.URL, "secret", false).List(context.Background(), "/v3/ou")
	require.NoError(t, err)
	assert.Equal(t, "Bearer secret", auth)
}

func TestListErrorsOnNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false).List(context.Background(), "/v4/compliance/family")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "405")
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

	got, err := NewClient(srv.URL, "k", false).List(context.Background(), "/beta/scope")
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

	got, err := NewClient(srv.URL, "k", false).List(context.Background(), "/beta/scope")
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

	_, err := NewClient(srv.URL, "k", false).List(context.Background(), "/beta/scope")
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

	got, err := NewClient(srv.URL, "k", false).List(context.Background(), "/beta/scope")
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

func TestListEmptyItemsEnvelope(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"items":[],"total":0}`)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k", false).List(context.Background(), "/beta/scope")
	require.NoError(t, err)
	assert.Len(t, got, 0)
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

func TestListErrorIncludesBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message":"invalid request parameter"}`)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", false).List(context.Background(), "/v3/ou")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request parameter")
}

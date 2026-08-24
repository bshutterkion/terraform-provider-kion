package kimport

import (
	"context"
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

func TestListPage2Error(t *testing.T) {
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

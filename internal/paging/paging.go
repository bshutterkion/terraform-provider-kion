// Package paging walks a Kion list endpoint to exhaustion.
//
// Kion serves collections ten records at a time unless asked otherwise, and
// reports the collection size alongside the page. Reading such an endpoint with
// a single unpaged request returns the first ten records and reports them as
// the whole collection -- a truncation the caller cannot detect, because the
// response looks exactly like a complete small collection.
//
// The control flow is here; decoding is not. Callers differ in what a page
// contains (untyped records for the import tool, a typed wire struct for a
// resource) and in how the envelope is unwrapped, which for the import tool
// carries a long defect history. Each supplies a fetch that returns one decoded
// page, and this package decides how many to ask for.
package paging

import (
	"context"
	"fmt"
)

const (
	// DefaultPageSize is what list endpoints are asked for. It is not the
	// server's default, which is ten.
	DefaultPageSize = 100

	// MaxPages bounds the walk. It is a backstop against a server whose
	// reported total never agrees with what it serves, not a limit any real
	// collection is expected to reach.
	MaxPages = 1000
)

// Page is one decoded page of a list response.
type Page[T any] struct {
	Items []T

	// Total is the collection size the server reports. A NEGATIVE Total means
	// the response was not a paginated envelope at all, so the first page is
	// the entire collection and no further requests are made.
	Total int
}

// Fetch returns one decoded page. Pages are 1-based.
type Fetch[T any] func(ctx context.Context, page int) (Page[T], error)

// All walks every page and returns the whole collection.
//
// label prefixes this package's own error (the page cap); errors from fetch are
// returned verbatim, so the caller controls their wording.
//
// On failure after the first page the records gathered so far are returned
// ALONGSIDE the error, so a caller that would rather report a partial
// collection than nothing can, provided it says which it is doing.
func All[T any](ctx context.Context, label string, fetch Fetch[T]) ([]T, error) {
	first, err := fetch(ctx, 1)
	if err != nil {
		return nil, err
	}
	records := first.Items
	if first.Total < 0 {
		return records, nil
	}

	for page := 2; len(records) < first.Total; page++ {
		if page > MaxPages {
			return records, fmt.Errorf("%s: paging exceeded max pages (%d)", label, MaxPages)
		}
		next, err := fetch(ctx, page)
		if err != nil {
			return records, err
		}
		// An empty page ends the walk even when total disagrees, so a wrong
		// total cannot spin this to the cap.
		if len(next.Items) == 0 {
			break
		}
		records = append(records, next.Items...)
	}
	return records, nil
}

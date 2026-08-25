// Package kimport enumerates a live Kion install into Terraform import blocks.
//
// It reads through arbitrary paths supplied by the embedded import manifest
// rather than the typed SDK, because the manifest's unit of work is a path, not
// an SDK method. Auth and transport mirror internal/provider/provider.go.
package kimport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	pageSize  = 100
	bodyLimit = 512  // max bytes to include in error messages
	maxPages  = 1000 // hard cap to prevent infinite paging loops
)

// Lister is the read seam the enumerators depend on, so they can be tested
// without a server.
type Lister interface {
	List(ctx context.Context, path string) ([]map[string]any, error)
}

// StatusError is a non-2xx response. Callers need the code, not just the text:
// a 404 from a child collection means the parent has none of that resource,
// which is an expected answer, while a 502 is a real failure.
type StatusError struct {
	Path   string
	Status int
	Body   string // truncated snippet, may be empty
}

func (e *StatusError) Error() string {
	line := fmt.Sprintf("%d %s", e.Status, http.StatusText(e.Status))
	if e.Body != "" {
		return fmt.Sprintf("GET %s: %s: %s", e.Path, line, e.Body)
	}
	return fmt.Sprintf("GET %s: %s", e.Path, line)
}

// Client is a minimal raw-HTTP Kion client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient builds a Client. apiPrefix (e.g. "/api") is appended to baseURL,
// tolerating a leading slash either way and never doubling a prefix baseURL
// already ends with. Hosted installs serve their API under "/api" -- pass
// that so callers can hand this a bare install URL unmodified. Pass "" for
// an install that serves the API at the root (e.g. an app hit directly on
// localhost). This mirrors kion-env-copy's KION_API_PREFIX convention.
func NewClient(baseURL, apiKey string, skipSSL bool, apiPrefix string) *Client {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		defaultTransport = &http.Transport{}
	}
	transport := defaultTransport.Clone()
	if skipSSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-requested
	}
	return &Client{
		baseURL: joinAPIPrefix(baseURL, apiPrefix),
		apiKey:  apiKey,
		http:    &http.Client{Transport: transport, Timeout: 60 * time.Second},
	}
}

// joinAPIPrefix appends prefix to baseURL. Both sides are normalized of
// leading/trailing slashes before joining, and if baseURL already ends with
// prefix (e.g. a caller passes --url https://host/api with the default
// --api-prefix /api), the prefix is not doubled. An empty (post-trim) prefix
// leaves baseURL untouched.
func joinAPIPrefix(baseURL, prefix string) string {
	base := strings.TrimSuffix(baseURL, "/")
	trimmedPrefix := strings.Trim(prefix, "/")
	if trimmedPrefix == "" {
		return base
	}
	if base == trimmedPrefix || strings.HasSuffix(base, "/"+trimmedPrefix) {
		return base
	}
	return base + "/" + trimmedPrefix
}

func (c *Client) get(ctx context.Context, path string, page int) (json.RawMessage, error) {
	u := c.baseURL + path
	q := url.Values{}
	q.Set("page", fmt.Sprint(page))
	q.Set("count", fmt.Sprint(pageSize))
	u += "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // close errors are not actionable

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodySnippet := truncateBody(resp)
		return nil, &StatusError{Path: path, Status: resp.StatusCode, Body: bodySnippet}
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: read body: %w", path, err)
	}
	var body json.RawMessage
	if err := json.Unmarshal(buf, &body); err != nil {
		if looksLikeHTML(buf) {
			return nil, fmt.Errorf(
				"GET %s: got an HTML page instead of JSON (likely a missing or wrong --api-prefix -- "+
					"the install may serve its API under a different prefix, or at the root if you are "+
					"hitting it directly): %s", path, snippetOf(buf))
		}
		return nil, fmt.Errorf("GET %s: decode: %w", path, err)
	}
	return body, nil
}

// looksLikeHTML reports whether buf, after leading whitespace, starts with
// "<" -- the shape of an HTML error/login page a web app returns with HTTP
// 200 for an unrecognized path, which a plain JSON-decode error would
// otherwise report as an opaque "invalid character '<'" message.
func looksLikeHTML(buf []byte) bool {
	trimmed := bytes.TrimLeft(buf, " \t\r\n")
	return len(trimmed) > 0 && trimmed[0] == '<'
}

// truncateBody reads up to bodyLimit bytes from resp.Body and returns a single-line snippet.
func truncateBody(resp *http.Response) string {
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, int64(bodyLimit)+1)) //nolint:errcheck // body read errors are acceptable here
	return snippetOf(buf)
}

// snippetOf renders buf as a single-line, length-capped preview for error messages.
func snippetOf(buf []byte) string {
	if len(buf) == 0 {
		return ""
	}
	truncated := len(buf) > bodyLimit
	if truncated {
		buf = buf[:bodyLimit]
	}
	// A byte-count truncation can land mid-rune, splitting a multi-byte UTF-8
	// character and turning the tail into invalid UTF-8 that would otherwise
	// flow straight into a customer-facing error message.
	snippet := strings.ToValidUTF8(string(buf), "")
	// Remove newlines and tabs for single-line output
	snippet = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return ' '
		}
		return r
	}, snippet)
	if truncated {
		snippet += "…"
	}
	return strings.TrimSpace(snippet)
}

// unwrap pulls records out of a bare list or an {items|data,total} envelope,
// and reports the envelope's total (-1 when there is none).
func unwrap(body json.RawMessage) ([]map[string]any, int, error) {
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err == nil {
		return list, -1, nil
	}

	var env struct {
		Items json.RawMessage `json:"items"`
		Data  json.RawMessage `json:"data"`
		Total *int            `json:"total"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, -1, err
	}

	total := -1
	isEnvelope := false
	if env.Total != nil {
		total = *env.Total
		isEnvelope = true
	}

	// Handle items field (may be null or empty)
	if len(env.Items) > 0 {
		var records []map[string]any
		if err := json.Unmarshal(env.Items, &records); err == nil {
			return records, total, nil
		}
		// A singleton endpoint returns one object rather than a list.
		var single map[string]any
		if err := json.Unmarshal(env.Items, &single); err == nil {
			return []map[string]any{single}, total, nil
		}
	}

	// Handle data field (may be null or empty)
	if len(env.Data) > 0 {
		var records []map[string]any
		if err := json.Unmarshal(env.Data, &records); err == nil {
			return records, total, nil
		}
		// A doubly-nested envelope: {"status":200,"data":{"items":[...],"total":N,...}}.
		// Some endpoints (e.g. /v4/billing-source, /v3/label, /beta/scope) wrap
		// their items/total envelope inside "data" instead of putting it at the
		// top level. Detect that shape structurally and unwrap the inner
		// envelope recursively, rather than falling through to the
		// single-object branch below and returning the envelope itself as one
		// bogus record.
		if isNestedEnvelope(env.Data) {
			return unwrap(env.Data)
		}
		// A named-collection envelope:
		// {"status":200,"data":{"dashboards":[{...}],"hidden_dashboard_count":0}}.
		// /v1/dashboards wraps its list under a descriptive key, alongside
		// sibling scalar/object metadata, instead of using "items" or a bare
		// array. Detect that shape structurally by counting the data object's
		// keys whose value is itself a JSON array: exactly one such key means
		// that array is the records and every other key (scalar or object) is
		// metadata to ignore. Zero array-valued keys, or two or more, fall
		// through unchanged to the singleton branch below, so a genuine
		// single-object record (e.g. {"data":{"id":1,"name":"x"}}) is still
		// returned as one record.
		if arr, ok := soleArrayValue(env.Data); ok {
			var records []map[string]any
			if err := json.Unmarshal(arr, &records); err == nil {
				return records, total, nil
			}
		}
		// A singleton endpoint returns one object rather than a list.
		var single map[string]any
		if err := json.Unmarshal(env.Data, &single); err == nil {
			return []map[string]any{single}, total, nil
		}
	}

	// If we detected an envelope (total field present), return empty list.
	// Never return the envelope itself as a record.
	if isEnvelope {
		return []map[string]any{}, total, nil
	}

	// A bare singleton object with no envelope.
	var single map[string]any
	if err := json.Unmarshal(body, &single); err == nil && len(single) > 0 {
		return []map[string]any{single}, -1, nil
	}
	return nil, total, nil
}

// isNestedEnvelope reports whether raw is a JSON object carrying an "items"
// key, meaning it is itself an envelope (e.g. the inner object of
// {"status":200,"data":{"pagination":{...},"total":17,"items":[...]}}) rather
// than a genuine single-object record. The key is checked for presence, not
// truthiness, so {"items":null} still counts -- that's an empty envelope, not
// a record named "items".
func isNestedEnvelope(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return false
	}
	_, ok := obj["items"]
	return ok
}

// soleArrayValue reports whether raw is a JSON object with exactly one
// array-valued key -- regardless of how many other, non-array keys it also
// has -- returning that array's raw bytes when so. Used to detect a
// named-collection envelope like {"dashboards":[...],"hidden_count":0} that
// wraps a list under a descriptive key, alongside sibling scalar/object
// metadata, rather than using "items" or a bare array.
func soleArrayValue(raw json.RawMessage) (json.RawMessage, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	var found json.RawMessage
	arrayKeys := 0
	for _, v := range obj {
		trimmed := bytes.TrimLeft(v, " \t\r\n")
		if len(trimmed) > 0 && trimmed[0] == '[' {
			arrayKeys++
			found = v
		}
	}
	if arrayKeys != 1 {
		return nil, false
	}
	return found, true
}

// dropPagePadding removes the zero-valued filler some paginated endpoints
// return alongside the page they were asked for. The compliance parent-scoped
// collections (/v4/compliance/program/{id}/control and .../family) answer
// every page with an items array of length total, populating only the
// requested page's window and leaving the rest as zero-valued structs. Left
// in, page 1 already satisfies len(records) >= total, paging stops, and every
// record past the first page is lost -- reported as "N record(s) skipped: no
// id" once the padding reaches toRecords.
//
// Only a paginated envelope that overshot the page size is filtered, so a
// well-behaved endpoint keeps its records untouched. Nothing real is dropped
// either way: an all-zero record has no id and is unimportable regardless.
func dropPagePadding(records []map[string]any, total int) []map[string]any {
	if total < 0 || len(records) <= pageSize {
		return records
	}
	kept := records[:0]
	for _, rec := range records {
		if !isZeroRecord(rec) {
			kept = append(kept, rec)
		}
	}
	return kept
}

// isZeroRecord reports whether every value in rec is its type's zero value --
// the shape of a padding slot in a pre-sized response array.
func isZeroRecord(rec map[string]any) bool {
	for _, v := range rec {
		switch t := v.(type) {
		case nil:
		case string:
			if t != "" {
				return false
			}
		case bool:
			if t {
				return false
			}
		case float64:
			if t != 0 {
				return false
			}
		case []any:
			if len(t) > 0 {
				return false
			}
		case map[string]any:
			if !isZeroRecord(t) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// List GETs path, unwrapping and paging as needed.
func (c *Client) List(ctx context.Context, path string) ([]map[string]any, error) {
	body, err := c.get(ctx, path, 1)
	if err != nil {
		return nil, err
	}
	records, total, err := unwrap(body)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	if total < 0 {
		return records, nil // not a paginated envelope
	}
	records = dropPagePadding(records, total)

	for page := 2; len(records) < total; page++ {
		// Enforce hard cap to prevent infinite loops
		if page > maxPages {
			return records, fmt.Errorf("GET %s: paging exceeded max pages (%d)", path, maxPages)
		}
		body, err := c.get(ctx, path, page)
		if err != nil {
			return records, err
		}
		batch, _, err := unwrap(body)
		// An unwrap error on any page is reported with the partial records gathered so far
		if err != nil {
			return records, fmt.Errorf("GET %s (page %d): %w", path, page, err)
		}
		batch = dropPagePadding(batch, total)
		// Empty batch is a legitimate stop signal
		if len(batch) == 0 {
			break
		}
		records = append(records, batch...)
	}
	return records, nil
}

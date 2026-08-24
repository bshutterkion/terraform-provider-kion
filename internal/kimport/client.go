// Package kimport enumerates a live Kion install into Terraform import blocks.
//
// It reads through arbitrary paths supplied by the embedded import manifest
// rather than the typed SDK, because the manifest's unit of work is a path, not
// an SDK method. Auth and transport mirror internal/provider/provider.go.
package kimport

import (
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

// Client is a minimal raw-HTTP Kion client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient builds a Client. baseURL should include any /api prefix the install
// serves under; an app hit directly serves at the root.
func NewClient(baseURL, apiKey string, skipSSL bool) *Client {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		defaultTransport = &http.Transport{}
	}
	transport := defaultTransport.Clone()
	if skipSSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-requested
	}
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Transport: transport, Timeout: 60 * time.Second},
	}
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
		if bodySnippet != "" {
			return nil, fmt.Errorf("GET %s: %s: %s", path, resp.Status, bodySnippet)
		}
		return nil, fmt.Errorf("GET %s: %s", path, resp.Status)
	}
	var body json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("GET %s: decode: %w", path, err)
	}
	return body, nil
}

// truncateBody reads up to bodyLimit bytes from resp.Body and returns a single-line snippet.
func truncateBody(resp *http.Response) string {
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, int64(bodyLimit)+1)) //nolint:errcheck // body read errors are acceptable here
	if len(buf) == 0 {
		return ""
	}
	snippet := string(buf)
	// Remove newlines and tabs for single-line output
	snippet = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return ' '
		}
		return r
	}, snippet)
	// Cap and add ellipsis if truncated
	if len(buf) > bodyLimit {
		snippet = snippet[:bodyLimit] + "…"
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
		// Empty batch is a legitimate stop signal
		if len(batch) == 0 {
			break
		}
		records = append(records, batch...)
	}
	return records, nil
}

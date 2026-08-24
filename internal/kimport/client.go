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
	"net/http"
	"net/url"
	"strings"
	"time"
)

const pageSize = 100

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
	transport := http.DefaultTransport.(*http.Transport).Clone() //nolint:errcheck // Clone always succeeds
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
		return nil, fmt.Errorf("GET %s: %d %s", path, resp.StatusCode, resp.Status)
	}
	var body json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("GET %s: decode: %w", path, err)
	}
	return body, nil
}

// unwrap pulls records out of a bare list or an {items|data,total} envelope,
// and reports the envelope's total (-1 when there is none).
func unwrap(body json.RawMessage) ([]map[string]any, int, error) {
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err == nil {
		return list, -1, nil
	}

	var env struct {
		Items []map[string]any `json:"items"`
		Data  json.RawMessage  `json:"data"`
		Total *int             `json:"total"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, -1, err
	}

	total := -1
	if env.Total != nil {
		total = *env.Total
	}
	if env.Items != nil {
		return env.Items, total, nil
	}
	if len(env.Data) > 0 {
		var records []map[string]any
		if err := json.Unmarshal(env.Data, &records); err == nil {
			return records, total, nil
		}
		// A singleton endpoint returns one object rather than a list.
		var single map[string]any
		if err := json.Unmarshal(env.Data, &single); err == nil {
			return []map[string]any{single}, total, nil
		}
	}

	// A bare singleton object with no envelope.
	var single map[string]any
	if err := json.Unmarshal(body, &single); err == nil && len(single) > 0 {
		return []map[string]any{single}, -1, nil
	}
	return nil, total, nil
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
		body, err := c.get(ctx, path, page)
		if err != nil {
			return records, err
		}
		batch, _, err := unwrap(body)
		if err != nil || len(batch) == 0 {
			break
		}
		records = append(records, batch...)
	}
	return records, nil
}

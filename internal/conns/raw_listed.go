package conns

import (
	"context"
	"encoding/json"
	"fmt"
)

// RawIDListed reports whether the collection at path lists a record with id.
// It accepts both list envelopes Kion uses: {"data":[…]} and
// {"data":{"items":[…]}}.
//
// It exists for private by-id reads that answer a missing record with a 500
// rather than a 404 (GET /v1/app-role/{id}): the public list, which omits
// deleted records, is the only way to tell "gone" from "broken". Any failure
// here is an error, never "absent", so a broken list cannot drop a live record
// from state.
func (c *KionClient) RawIDListed(ctx context.Context, path string, id int64) (bool, error) {
	body, err := c.RawGet(ctx, path)
	if err != nil {
		return false, err
	}
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false, fmt.Errorf("decoding %s: %w", path, err)
	}
	type rec struct {
		ID int64 `json:"id"`
	}
	var recs []rec
	if err := json.Unmarshal(env.Data, &recs); err != nil {
		var paged struct {
			Items []rec `json:"items"`
		}
		if perr := json.Unmarshal(env.Data, &paged); perr != nil {
			return false, fmt.Errorf("decoding %s: data is neither a list nor {items}", path)
		}
		recs = paged.Items
	}
	for _, r := range recs {
		if r.ID == id {
			return true, nil
		}
	}
	return false, nil
}

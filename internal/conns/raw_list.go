package conns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"terraform-provider-kion/internal/paging"
)

// RawListContainsID reports whether the paginated collection at path lists a
// record with the given id, walking every page.
//
// It exists for a by-id GET that serves soft-deleted records while its parent's
// list does not: the list is then the only way to tell a deleted record from a
// live one. The answer errs toward "listed", because calling a live record gone
// makes Terraform plan its recreation:
//   - a 404 for the collection means its parent is gone, so the record is too;
//   - a walk that gathers fewer records than the reported total proves nothing
//     and counts as listed;
//   - a body that is not the {"data":{"total":N,"items":[…]}} envelope is an
//     error, never an empty list.
func (c *KionClient) RawListContainsID(ctx context.Context, path, id string) (bool, error) {
	type item struct {
		ID json.Number `json:"id"`
	}
	total := 0
	all, err := paging.All(ctx, "listing "+path, func(ctx context.Context, page int) (paging.Page[item], error) {
		q := url.Values{}
		q.Set("page", strconv.Itoa(page))
		q.Set("count", strconv.Itoa(paging.DefaultPageSize))
		body, err := c.RawGet(ctx, path+"?"+q.Encode())
		if err != nil {
			return paging.Page[item]{}, err
		}
		var env struct {
			Data *struct {
				Total *int   `json:"total"`
				Items []item `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return paging.Page[item]{}, fmt.Errorf("decoding %s: %w", path, err)
		}
		if env.Data == nil || env.Data.Total == nil {
			return paging.Page[item]{}, fmt.Errorf("decoding %s: not a paginated list response", path)
		}
		total = *env.Data.Total
		return paging.Page[item]{Items: env.Data.Items, Total: total}, nil
	})
	if IsRawNotFound(err) && len(all) == 0 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, it := range all {
		if it.ID.String() == id {
			return true, nil
		}
	}
	return len(all) < total, nil
}

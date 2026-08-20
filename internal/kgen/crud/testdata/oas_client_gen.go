package testdata

import "context"

// Client is a miniature stand-in for the ogen-generated SDK client. Only the
// Label ops are present. The crud generator parses this file with go/parser
// (no type-checking), so unresolved identifiers in the signatures are fine;
// keeping it minimal avoids cross-fixture duplicate declarations and gofmt
// churn. Go tooling never compiles files under testdata/.

// PostLabel invokes PostLabel operation.
//
// Creates a new app label in the application.
//
// POST /v3/label
func (c *Client) PostLabel(ctx context.Context, request OptCreateLabel) (PostLabelRes, error) {
	return nil, nil
}

// GetLabel invokes GetLabel operation.
//
// GET /v3/label/{id}
func (c *Client) GetLabel(ctx context.Context, params GetLabelParams) (GetLabelRes, error) {
	return nil, nil
}

// PatchLabel invokes PatchLabel operation.
//
// PATCH /v3/label/{id}
func (c *Client) PatchLabel(ctx context.Context, request OptUpdateLabel, params PatchLabelParams) (PatchLabelRes, error) {
	return nil, nil
}

// DeleteLabel invokes DeleteLabel operation.
//
// DELETE /v3/label/{id}
func (c *Client) DeleteLabel(ctx context.Context, params DeleteLabelParams) (DeleteLabelRes, error) {
	return nil, nil
}

// GetLabelIndex invokes GetLabelIndex operation.
//
// GET /v3/label
func (c *Client) GetLabelIndex(ctx context.Context, params GetLabelIndexParams) (GetLabelIndexRes, error) {
	return nil, nil
}

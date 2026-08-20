package errs_test

import (
	"testing"

	"terraform-provider-kion/internal/errs"

	generated "github.com/kionsoftware/kion-sdk-go/generated/v3_16"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatedID_Success(t *testing.T) {
	t.Parallel()

	res := &generated.CreatedResponse{RecordID: generated.NewOptUint64(42)}
	id, diags := errs.CreatedID(res)

	require.False(t, diags.HasError())
	assert.Equal(t, int64(42), id)
}

func TestCreatedID_MissingRecordID(t *testing.T) {
	t.Parallel()

	res := &generated.CreatedResponse{} // RecordID unset
	_, diags := errs.CreatedID(res)

	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Summary(), "Missing Record ID")
}

func TestCreatedID_ErrorResponseType(t *testing.T) {
	t.Parallel()

	res := &generated.BadRequestResponse{Message: generated.NewOptString("nope")}
	_, diags := errs.CreatedID(res)

	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Detail(), "Bad Request: nope")
}

func TestCreatedID_UnexpectedType(t *testing.T) {
	t.Parallel()

	_, diags := errs.CreatedID("not a response")

	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Summary(), "Unexpected Response")
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	assert.True(t, errs.IsNotFound(&generated.NotFoundResponse{}))
	assert.False(t, errs.IsNotFound(&generated.CreatedResponse{}))
	assert.False(t, errs.IsNotFound("nope"))
}

func TestResponseDiagnostics(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		res        any
		wantErr    bool
		wantDetail string
	}{
		{"bad request", &generated.BadRequestResponse{Message: generated.NewOptString("b")}, true, "Bad Request: b"},
		{"unauthorized", &generated.UnauthorizedResponse{Message: generated.NewOptString("u")}, true, "Unauthorized: u"},
		{"forbidden", &generated.ForbiddenResponse{Message: generated.NewOptString("f")}, true, "Forbidden: f"},
		{"not found", &generated.NotFoundResponse{Message: generated.NewOptString("n")}, true, "Not Found: n"},
		{"internal", &generated.InternalServerErrorResponse{Message: generated.NewOptString("i")}, true, "Internal Server Error: i"},
		{"unprocessable", &generated.UnprocessableEntityResponse{Message: generated.NewOptString("e")}, true, "Unprocessable Entity: e"},
		{"no message uses fallback", &generated.NotFoundResponse{}, true, "Not Found: no message"},
		{"non-error type is empty", &generated.CreatedResponse{}, false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			diags := errs.ResponseDiagnostics("op failed", tc.res)
			require.Equal(t, tc.wantErr, diags.HasError())
			if tc.wantErr {
				assert.Equal(t, "op failed", diags.Errors()[0].Summary())
				assert.Contains(t, diags.Errors()[0].Detail(), tc.wantDetail)
			}
		})
	}
}

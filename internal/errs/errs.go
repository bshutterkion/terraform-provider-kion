// Package errs provides helper functions for extracting errors from SDK responses.
package errs

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	generated "github.com/kionsoftware/kion-sdk-go/generated/v3_16"
)

// CreatedID extracts the record ID from a CreatedResponse returned by POST
// endpoints. Returns an error diagnostic if the response is not a CreatedResponse
// or if the record ID is not set.
func CreatedID(res any) (int64, diag.Diagnostics) {
	var diags diag.Diagnostics

	cr, ok := res.(*generated.CreatedResponse)
	if !ok {
		diags.Append(ResponseDiagnostics("extracting created ID", res)...)
		if !diags.HasError() {
			diags.AddError("Unexpected Response", fmt.Sprintf("expected *CreatedResponse, got %T", res))
		}
		return 0, diags
	}

	if !cr.RecordID.IsSet() {
		diags.AddError("Missing Record ID", "the created response did not contain a record ID")
		return 0, diags
	}

	return int64(cr.RecordID.Value), diags
}

// IsNotFound returns true if the response is a *NotFoundResponse.
func IsNotFound(res any) bool {
	_, ok := res.(*generated.NotFoundResponse)
	return ok
}

// ResponseDiagnostics converts error response types (BadRequest, Unauthorized,
// Forbidden, NotFound, InternalServerError) into Terraform diagnostics.
// Returns empty diagnostics for success response types.
func ResponseDiagnostics(summary string, res any) diag.Diagnostics {
	var diags diag.Diagnostics

	switch r := res.(type) {
	case *generated.BadRequestResponse:
		diags.AddError(summary, fmt.Sprintf("Bad Request: %s", r.Message.Or("no message")))
	case *generated.UnauthorizedResponse:
		diags.AddError(summary, fmt.Sprintf("Unauthorized: %s", r.Message.Or("no message")))
	case *generated.ForbiddenResponse:
		diags.AddError(summary, fmt.Sprintf("Forbidden: %s", r.Message.Or("no message")))
	case *generated.NotFoundResponse:
		diags.AddError(summary, fmt.Sprintf("Not Found: %s", r.Message.Or("no message")))
	case *generated.InternalServerErrorResponse:
		diags.AddError(summary, fmt.Sprintf("Internal Server Error: %s", r.Message.Or("no message")))
	case *generated.UnprocessableEntityResponse:
		diags.AddError(summary, fmt.Sprintf("Unprocessable Entity: %s", r.Message.Or("no message")))
	}

	return diags
}

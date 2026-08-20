package testdata

// Miniature stand-in for the ogen oas_parameters_gen.go, Label ops only.

type GetLabelParams struct{ ID int64 }

type PatchLabelParams struct{ ID int64 }

type DeleteLabelParams struct{ ID int64 }

type GetLabelIndexParams struct {
	Page  OptInt64 `json:",omitempty,omitzero"`
	Count OptInt64 `json:",omitempty,omitzero"`
}

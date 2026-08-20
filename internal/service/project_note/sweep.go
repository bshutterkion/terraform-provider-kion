package project_note

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	resource.AddTestSweepers("kion_project_note", &resource.Sweeper{
		Name: "kion_project_note",
		F:    sweepProjectNote,
	})
}

func sweepProjectNote(_ string) error {
	// TODO: List all kion_project_note resources.
	// Delete any with "test-acc" prefix in their name.
	return nil
}

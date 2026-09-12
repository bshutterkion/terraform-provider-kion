package project_cloud_access_role_exemption_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-kion/internal/acctest"
)

// What this test can and cannot cover.
//
// kion_project_cloud_access_role_exemption is a no_read resource, and not by
// choice: the collection endpoint that should list an exemption returns an
// empty array for one that demonstrably exists. Verified against a live
// install:
//
//	POST /v3/project-cloud-access-role-exemption  -> 201 {"record_id":2}
//	GET  /v3/project/1/cloud-rule/exemption       -> 200 {"data":[]}
//	DELETE /v3/project-cloud-access-role-exemption/2 -> 200
//
// That is #80, which the OU variant shows too, and the consequence is worse than
// "cannot import". The resource DOES read -- through that collection -- so the
// refresh after every apply finds nothing, removes the resource from state, and
// the next plan proposes creating it again. Applying repeatedly leaks a new
// exemption each time, and the configuration never converges.
//
// ExpectNonEmptyPlan records exactly that, so the create is covered now and this
// test starts failing the moment #80 is fixed -- at which point the flag comes
// off and a read-back assertion goes in.
//
// What is covered meanwhile is still worth having: that the create is accepted,
// and that a real id reaches state rather than the zero that would leave an
// unaddressable record.
func TestAccKionProjectCloudAccessRoleExemption_basic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running test in short mode")
	}

	rName := acctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "kion_project_cloud_access_role_exemption.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectCloudAccessRoleExemptionConfig_basic(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					// Not just "set": a zero id means the create extracted
					// nothing and the record is unaddressable, which reads
					// identically to "deleted" on the next plan.
					resource.TestCheckResourceAttrWith(resourceName, "id", func(v string) error {
						if v == "" || v == "0" {
							return fmt.Errorf("id is %q, so the created exemption cannot be addressed", v)
						}
						return nil
					}),
					resource.TestCheckResourceAttrPair(resourceName, "project_id", "kion_project.test", "id"),
				),
				// See the comment above: the post-apply refresh cannot find the
				// record, so a plan is always outstanding. #80.
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// The exemption needs a project and an OU-level cloud access role. Scheme 2 is
// Kion's system-managed "Default OU Permissions Scheme", present on every
// install, so the OU does not need one built for it.
func testAccProjectCloudAccessRoleExemptionConfig_basic(rName string) string {
	return acctest.FundingSourceConfig(rName) + fmt.Sprintf(`
resource "kion_permission_scheme" "test_perm_project" {
  name = "%[1]s-perm-project"
  type = "project"
}

resource "kion_project" "test" {
  name                 = "%[1]s-project"
  ou_id                = kion_ou.test_fs_ou.id
  permission_scheme_id = kion_permission_scheme.test_perm_project.id
  owner_user_ids       = [1]

  budget = [{
    amount             = 500
    start_datecode     = "2026-01"
    end_datecode       = "2026-12"
    funding_source_ids = [kion_funding_source.test_fs.id]
  }]
}

resource "kion_ou_cloud_access_role" "test_car" {
  name  = "%[1]s-car"
  ou_id = kion_ou.test_fs_ou.id
}

resource "kion_project_cloud_access_role_exemption" "test" {
  project_id              = kion_project.test.id
  ou_cloud_access_role_id = kion_ou_cloud_access_role.test_car.id
  reason                  = "test-acc exemption"
}
`, rName)
}

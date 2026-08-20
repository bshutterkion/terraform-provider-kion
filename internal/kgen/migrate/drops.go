package migrate

// ConfigDrops lists, per resource type, the old settable attributes kmigrate
// removes from a customer's .tf because the new schema no longer accepts them.
// These are OBSOLETE fields with no new home — timestamps the provider now
// computes (last_updated), removed toggles (system_managed_policy), and settings
// the new schema dropped. Removing them is exactly what a customer would do by
// hand to make `terraform plan` parse; kmigrate reports each removal.
//
// Deliberately EXCLUDED (handled elsewhere, never auto-dropped here):
//   - Renamed attributes (e.g. account name → account_name): the rename pass
//     rewrites them; they are not obsolete.
//   - kion_azure_policy's body (name/description/policy/parameters): folded into
//     the nested azure_policy object — a documented manual config change, not a
//     drop (auto-dropping would silently discard the policy body).
//
// The *_account resources are NOT excluded. Their state migrates via Layer 3
// import, but their config is still rewritten here — kmigrate already renames
// name → account_name on all three — so an obsolete settable attribute left in
// place fails `terraform validate` just like anywhere else.
//
// TestConfigDropsAreSettableAndRemoved keeps this consistent with the schema
// snapshots: every entry must be an old settable attribute genuinely absent from
// the new schema. TestCompleteness_settableDropsAreDocumented is the wider
// ledger of every settable attribute the new schema dropped, reviewed or not.
var ConfigDrops = map[string][]string{
	"kion_app_config":         {"forecasting_enabled", "idms_groups_as_viewers_default", "saml_debug"},
	"kion_azure_arm_template": {"last_updated"},
	// cft alias (kion_aws_cloudformation_template → kion_cft): last_updated dropped.
	"kion_aws_cloudformation_template": {"last_updated"},
	// azure_policy's name/description/policy/parameters are folded into the
	// nested azure_policy object (AttrsToObject); only last_updated is a plain drop.
	"kion_azure_policy": {"last_updated"},
	"kion_azure_role":   {"last_updated"},
	// The new custom_account has no skip_access_checking (gcp/azure kept theirs).
	"kion_custom_account": {"skip_access_checking"},
	"kion_funding_source": {"labels", "last_updated"},
	// create_mode was the old gcp_account's import-vs-create switch; the new
	// resource infers it from the fields supplied.
	"kion_gcp_account":            {"create_mode"},
	"kion_gcp_iam_role":           {"last_updated", "system_managed_policy"},
	"kion_project_note":           {"last_updated"},
	"kion_service_control_policy": {"last_updated"},
}

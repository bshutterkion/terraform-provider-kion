package crud

import _ "embed"

//go:embed companion_account_linkage_data_source_test.gtpl
var accountLinkageDataSourceTestTmpl string

//go:embed companion_account_test.gtpl
var accountTestTmpl string

//go:embed companion_account_data_source_test.gtpl
var accountDataSourceTestTmpl string

//go:embed companion_app_config_test.gtpl
var appConfigTestTmpl string

//go:embed companion_app_config_data_source_test.gtpl
var appConfigDataSourceTestTmpl string

//go:embed companion_automation_policy_test.gtpl
var automationPolicyTestTmpl string

//go:embed companion_automation_policy_data_source_test.gtpl
var automationPolicyDataSourceTestTmpl string

//go:embed companion_aws_account_test.gtpl
var awsAccountTestTmpl string

//go:embed companion_aws_account_data_source_test.gtpl
var awsAccountDataSourceTestTmpl string

//go:embed companion_billing_rule_data_source_test.gtpl
var billingRuleDataSourceTestTmpl string

//go:embed companion_custom_variable_override_test.gtpl
var customVariableOverrideTestTmpl string

//go:embed companion_custom_variable_override_data_source_test.gtpl
var customVariableOverrideDataSourceTestTmpl string

//go:embed companion_gcp_regions_data_source_test.gtpl
var gcpRegionsDataSourceTestTmpl string

//go:embed companion_ou_permission_mapping_data_source_test.gtpl
var ouPermissionMappingDataSourceTestTmpl string

//go:embed companion_project_enforcement_data_source_test.gtpl
var projectEnforcementDataSourceTestTmpl string

//go:embed companion_project_permission_mapping_data_source_test.gtpl
var projectPermissionMappingDataSourceTestTmpl string

//go:embed companion_scope_criteria_test.gtpl
var scopeCriteriaTestTmpl string

//go:embed companion_user_test.gtpl
var userTestTmpl string

//go:embed companion_user_data_source_test.gtpl
var userDataSourceTestTmpl string

//go:embed companion_webhook_test.gtpl
var webhookTestTmpl string

//go:embed companion_webhook_data_source_test.gtpl
var webhookDataSourceTestTmpl string

//go:embed companion_account_linkage_data_source.gtpl
var accountLinkageDataSourceTmpl string

//go:embed companion_account_linkage_sweep.gtpl
var accountLinkageSweepTmpl string

//go:embed companion_billing_rule_data_source.gtpl
var billingRuleDataSourceTmpl string

//go:embed companion_billing_rule_sweep.gtpl
var billingRuleSweepTmpl string

// companionBlendedByName are the data source and sweeper a blended resource
// ships but its archetype does not derive: it emits only the resource and the
// service package. Without these the committed files claimed "Code generated"
// while nothing regenerated them.
var companionBlendedByName = map[string][]bespokeFile{
	"account_linkage": {
		{accountLinkageDataSourceTmpl, "account_linkage_data_source.go"},
		{accountLinkageSweepTmpl, "sweep.go"},
	},
	"billing_rule": {
		{billingRuleDataSourceTmpl, "billing_rule_data_source.go"},
		{billingRuleSweepTmpl, "sweep.go"},
	},
}

// companionTestsByName are acceptance tests kept verbatim for resources whose
// archetype derives no test. Registered separately from companionsByName so
// neither map depends on package init order.
var companionTestsByName = map[string][]bespokeFile{
	"account": {
		{accountDataSourceTestTmpl, "account_data_source_test.go"},
		{accountTestTmpl, "account_test.go"},
	},
	"account_linkage": {
		{accountLinkageDataSourceTestTmpl, "account_linkage_data_source_test.go"},
	},
	"app_config": {
		{appConfigDataSourceTestTmpl, "app_config_data_source_test.go"},
		{appConfigTestTmpl, "app_config_test.go"},
	},
	"automation_policy": {
		{automationPolicyDataSourceTestTmpl, "automation_policy_data_source_test.go"},
		{automationPolicyTestTmpl, "automation_policy_test.go"},
	},
	"aws_account": {
		{awsAccountDataSourceTestTmpl, "aws_account_data_source_test.go"},
		{awsAccountTestTmpl, "aws_account_test.go"},
	},
	"billing_rule": {
		{billingRuleDataSourceTestTmpl, "billing_rule_data_source_test.go"},
	},
	"custom_variable_override": {
		{customVariableOverrideDataSourceTestTmpl, "custom_variable_override_data_source_test.go"},
		{customVariableOverrideTestTmpl, "custom_variable_override_test.go"},
	},
	"gcp_regions": {
		{gcpRegionsDataSourceTestTmpl, "gcp_regions_data_source_test.go"},
	},
	"ou_permission_mapping": {
		{ouPermissionMappingDataSourceTestTmpl, "ou_permission_mapping_data_source_test.go"},
	},
	"project_enforcement": {
		{projectEnforcementDataSourceTestTmpl, "project_enforcement_data_source_test.go"},
	},
	"project_permission_mapping": {
		{projectPermissionMappingDataSourceTestTmpl, "project_permission_mapping_data_source_test.go"},
	},
	"scope_criteria": {
		{scopeCriteriaTestTmpl, "scope_criteria_test.go"},
	},
	"user": {
		{userDataSourceTestTmpl, "user_data_source_test.go"},
		{userTestTmpl, "user_test.go"},
	},
	"webhook": {
		{webhookDataSourceTestTmpl, "webhook_data_source_test.go"},
		{webhookTestTmpl, "webhook_test.go"},
	},
}

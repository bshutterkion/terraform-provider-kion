package tests

// ResourceMeta describes a Terraform resource's SDK methods, dependencies, and
// field overrides so the test generator can produce working acceptance tests.
type ResourceMeta struct {
	// TypeName is the full Terraform type name (e.g. "kion_label").
	TypeName string

	// SDKGetMethod is the generated client method for reading one resource
	// (e.g. "GetLabel"). Empty means no single-get endpoint.
	SDKGetMethod string

	// SDKGetParams is a Go expression that builds the get-method params struct.
	// The placeholder "id" refers to the parsed int64 resource ID.
	// Example: "generated.GetLabelParams{ID: id}"
	SDKGetParams string

	// SDKDeleteMethod is the generated client method for deleting one resource.
	SDKDeleteMethod string

	// SDKDeleteParams is a Go expression for the delete-method params struct.
	SDKDeleteParams string

	// Dependencies lists other resources that must be created first in HCL.
	Dependencies []Dependency

	// FieldOverrides provides domain-valid values for specific fields,
	// keyed by the HCL attribute name.
	FieldOverrides map[string]FieldValue

	// ExtraHCLBlocks are literal HCL blocks appended inside the resource
	// (e.g. "owner_users { id = 1 }").
	ExtraHCLBlocks []string
}

// Dependency represents a Terraform resource that must be created before
// the resource under test.
type Dependency struct {
	// TypeName is the full Terraform type name (e.g. "kion_permission_scheme").
	TypeName string

	// RefName is the HCL label (e.g. "test_perm").
	RefName string

	// Fields maps HCL attribute names to their literal values in the
	// dependency block. Use %[1]q for values that need the rName parameter.
	Fields map[string]string

	// RefAttribute is the attribute on the dependency to reference (e.g. "id").
	RefAttribute string

	// TargetField is the field on the resource under test that references
	// the dependency (e.g. "permission_scheme_id").
	TargetField string
}

// FieldValue holds the basic and (optional) update values for a field.
type FieldValue struct {
	// Basic is the value used in the _basic config.
	Basic string

	// Update is the value used in the _update config. If empty, Basic is reused.
	Update string
}

// GetMeta returns the ResourceMeta for the given Terraform type name, or nil
// if the type is not in the registry.
func GetMeta(typeName string) *ResourceMeta {
	m, ok := registry[typeName]
	if !ok {
		return nil
	}
	return &m
}

// registry maps Terraform type names to their metadata.
var registry = map[string]ResourceMeta{
	// ── Tier 0: No dependencies ─────────────────────────────────────────
	"kion_label": {
		TypeName:        "kion_label",
		SDKGetMethod:    "GetLabel",
		SDKGetParams:    "generated.GetLabelParams{ID: id}",
		SDKDeleteMethod: "DeleteLabel",
		SDKDeleteParams: "generated.DeleteLabelParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"key":   {Basic: `"test-acc-%[1]s"`, Update: `"test-acc-%[1]s-upd"`},
			"value": {Basic: `"test-acc-%[1]s"`, Update: `"test-acc-%[1]s-upd"`},
			"color": {Basic: `"#0088ff"`, Update: `"#ff0000"`},
		},
	},
	"kion_category": {
		TypeName:        "kion_category",
		SDKDeleteMethod: "DeleteCategoryByID",
		SDKDeleteParams: "generated.DeleteCategoryByIDParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":  {Basic: `%[1]q`, Update: `%[1]q`},
			"color": {Basic: `"#0088ff"`, Update: `"#ff0000"`},
		},
	},
	"kion_idms": {
		TypeName:        "kion_idms",
		SDKGetMethod:    "GetIDMS",
		SDKGetParams:    "generated.GetIDMSParams{ID: id}",
		SDKDeleteMethod: "DeleteIDMS",
		SDKDeleteParams: "generated.DeleteIDMSParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":                {Basic: `%[1]q`, Update: `%[1]q`},
			"idms_type_id":        {Basic: "1"},
			"password_expiration": {Basic: "0"},
		},
	},
	"kion_permission_scheme": {
		TypeName:        "kion_permission_scheme",
		SDKGetMethod:    "GetPermissionScheme",
		SDKGetParams:    "generated.GetPermissionSchemeParams{ID: id}",
		SDKDeleteMethod: "DeletePermissionScheme",
		SDKDeleteParams: "generated.DeletePermissionSchemeParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},

	// ── Tier 1: Simple dependencies ─────────────────────────────────────
	"kion_user_group": {
		TypeName:        "kion_user_group",
		SDKGetMethod:    "GetUserGroup",
		SDKGetParams:    "generated.GetUserGroupParams{ID: id}",
		SDKDeleteMethod: "DeleteUserGroup",
		SDKDeleteParams: "generated.DeleteUserGroupParams{ID: id}",
		Dependencies: []Dependency{
			{
				TypeName:     "kion_idms",
				RefName:      "test_idms",
				Fields:       map[string]string{"name": `"test-acc-idms-%[1]s"`, "idms_type_id": "1", "password_expiration": "0"},
				RefAttribute: "id",
				TargetField:  "idms_id",
			},
		},
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc group"`, Update: `"test-acc group updated"`},
		},
	},
	"kion_ou": {
		TypeName:        "kion_ou",
		SDKGetMethod:    "GetOU",
		SDKGetParams:    "generated.GetOUParams{ID: id}",
		SDKDeleteMethod: "DeleteOU",
		SDKDeleteParams: "generated.DeleteOUParams{ID: id}",
		Dependencies: []Dependency{
			{
				TypeName:     "kion_permission_scheme",
				RefName:      "test_perm",
				Fields:       map[string]string{"name": `"test-acc-perm-%[1]s"`},
				RefAttribute: "id",
				TargetField:  "permission_scheme_id",
			},
		},
		FieldOverrides: map[string]FieldValue{
			"name":         {Basic: `%[1]q`, Update: `%[1]q`},
			"description":  {Basic: `"test-acc OU"`, Update: `"test-acc OU updated"`},
			"parent_ou_id": {Basic: "0"},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},

	// ── Tier 2: Resources with owner ────────────────────────────────────
	"kion_cloud_rule": {
		TypeName:        "kion_cloud_rule",
		SDKGetMethod:    "GetCloudRuleShow",
		SDKGetParams:    "generated.GetCloudRuleShowParams{ID: id}",
		SDKDeleteMethod: "DeleteCloudRule",
		SDKDeleteParams: "generated.DeleteCloudRuleParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc cloud rule"`, Update: `"test-acc cloud rule updated"`},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_iam_policy": {
		TypeName:        "kion_iam_policy",
		SDKGetMethod:    "GetIAMPolicy",
		SDKGetParams:    "generated.GetIAMPolicyParams{ID: id}",
		SDKDeleteMethod: "DeleteIAMPolicy",
		SDKDeleteParams: "generated.DeleteIAMPolicyParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc IAM policy"`, Update: `"test-acc IAM policy updated"`},
			"policy": {
				Basic:  `"{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Deny\",\"Action\":\"s3:*\",\"Resource\":\"*\"}]}"`,
				Update: `"{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Deny\",\"Action\":\"ec2:*\",\"Resource\":\"*\"}]}"`,
			},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_cft": {
		TypeName:        "kion_cft",
		SDKGetMethod:    "GetCFT",
		SDKGetParams:    "generated.GetCFTParams{ID: id}",
		SDKDeleteMethod: "DeleteCFT",
		SDKDeleteParams: "generated.DeleteCFTParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc CFT"`, Update: `"test-acc CFT updated"`},
			"regions":     {Basic: `["us-east-1"]`},
			"policy": {
				Basic: `"{\"AWSTemplateFormatVersion\":\"2010-09-09\",\"Description\":\"Test\"}"`,
			},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_ami": {
		TypeName:        "kion_ami",
		SDKGetMethod:    "GetAMI",
		SDKGetParams:    "generated.GetAMIParams{ID: id}",
		SDKDeleteMethod: "DeleteAMI",
		SDKDeleteParams: "generated.DeleteAMIParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc AMI"`, Update: `"test-acc AMI updated"`},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_azure_arm_template": {
		TypeName:        "kion_azure_arm_template",
		SDKGetMethod:    "GetAzureARMTemplate",
		SDKGetParams:    "generated.GetAzureARMTemplateParams{ID: id}",
		SDKDeleteMethod: "DeleteARMTemplate",
		SDKDeleteParams: "generated.DeleteARMTemplateParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc ARM template"`, Update: `"test-acc ARM template updated"`},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_azure_policy": {
		TypeName:        "kion_azure_policy",
		SDKGetMethod:    "GetAzurePolicyByID",
		SDKGetParams:    "generated.GetAzurePolicyByIDParams{ID: id}",
		SDKDeleteMethod: "DeleteAzurePolicy",
		SDKDeleteParams: "generated.DeleteAzurePolicyParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc Azure policy"`, Update: `"test-acc Azure policy updated"`},
			"policy": {
				Basic: `"{\"if\":{\"field\":\"type\",\"equals\":\"Microsoft.Resources/subscriptions\"},\"then\":{\"effect\":\"audit\"}}"`,
			},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_azure_role": {
		TypeName:        "kion_azure_role",
		SDKGetMethod:    "GetAzureRole",
		SDKGetParams:    "generated.GetAzureRoleParams{ID: id}",
		SDKDeleteMethod: "DeleteAzureRole",
		SDKDeleteParams: "generated.DeleteAzureRoleParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc Azure role"`, Update: `"test-acc Azure role updated"`},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_compliance_check": {
		TypeName:        "kion_compliance_check",
		SDKGetMethod:    "GetComplianceCheck",
		SDKGetParams:    "generated.GetComplianceCheckParams{ID: id}",
		SDKDeleteMethod: "DeleteComplianceCheck",
		SDKDeleteParams: "generated.DeleteComplianceCheckParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc compliance check"`, Update: `"test-acc compliance check updated"`},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_compliance_standard": {
		TypeName:        "kion_compliance_standard",
		SDKGetMethod:    "GetComplianceStandard",
		SDKGetParams:    "generated.GetComplianceStandardParams{ID: id}",
		SDKDeleteMethod: "DeleteComplianceStandard",
		SDKDeleteParams: "generated.DeleteComplianceStandardParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_compliance_family": {
		TypeName:        "kion_compliance_family",
		SDKGetMethod:    "GetComplianceFamily",
		SDKGetParams:    "generated.GetComplianceFamilyParams{ID: id}",
		SDKDeleteMethod: "DeleteComplianceFamily",
		SDKDeleteParams: "generated.DeleteComplianceFamilyParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_compliance_level": {
		TypeName:        "kion_compliance_level",
		SDKGetMethod:    "GetComplianceLevel",
		SDKGetParams:    "generated.GetComplianceLevelParams{ID: id}",
		SDKDeleteMethod: "DeleteComplianceLevel",
		SDKDeleteParams: "generated.DeleteComplianceLevelParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_compliance_program": {
		TypeName:        "kion_compliance_program",
		SDKGetMethod:    "GetComplianceProgram",
		SDKGetParams:    "generated.GetComplianceProgramParams{ID: id}",
		SDKDeleteMethod: "DeleteComplianceProgram",
		SDKDeleteParams: "generated.DeleteComplianceProgramParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_billing_rule": {
		TypeName:        "kion_billing_rule",
		SDKGetMethod:    "GetBillingRule",
		SDKGetParams:    "generated.GetBillingRuleParams{ID: id}",
		SDKDeleteMethod: "DeleteBillingRule",
		SDKDeleteParams: "generated.DeleteBillingRuleParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_budget": {
		TypeName:        "kion_budget",
		SDKGetMethod:    "GetBudget",
		SDKGetParams:    "generated.GetBudgetParams{ID: id}",
		SDKDeleteMethod: "DeleteBudget",
		SDKDeleteParams: "generated.DeleteBudgetParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_funding_source": {
		TypeName:        "kion_funding_source",
		SDKGetMethod:    "GetFundingSource",
		SDKGetParams:    "generated.GetFundingSourceParams{ID: id}",
		SDKDeleteMethod: "DeleteFundingSource",
		SDKDeleteParams: "generated.DeleteFundingSourceParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc funding source"`, Update: `"test-acc funding source updated"`},
			"amount":      {Basic: "1000.00", Update: "2000.00"},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_account": {
		TypeName:        "kion_account",
		SDKGetMethod:    "GetAccount",
		SDKGetParams:    "generated.GetAccountParams{ID: id}",
		SDKDeleteMethod: "DeleteAccount",
		SDKDeleteParams: "generated.DeleteAccountParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_account_cache": {
		TypeName:        "kion_account_cache",
		SDKGetMethod:    "GetAccountCache",
		SDKGetParams:    "generated.GetAccountCacheParams{ID: id}",
		SDKDeleteMethod: "DeleteAccountCache",
		SDKDeleteParams: "generated.DeleteAccountCacheParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_app_api_key": {
		TypeName:        "kion_app_api_key",
		SDKGetMethod:    "GetAppAPIKey",
		SDKGetParams:    "generated.GetAppAPIKeyParams{ID: id}",
		SDKDeleteMethod: "DeleteAppAPIKey",
		SDKDeleteParams: "generated.DeleteAppAPIKeyParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_gcp_iam_role": {
		TypeName: "kion_gcp_iam_role",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc GCP IAM role"`, Update: `"test-acc GCP IAM role updated"`},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_service_control_policy": {
		TypeName: "kion_service_control_policy",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc SCP"`, Update: `"test-acc SCP updated"`},
			"policy": {
				Basic: `"{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Deny\",\"Action\":\"s3:*\",\"Resource\":\"*\"}]}"`,
			},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_custom_variable": {
		TypeName: "kion_custom_variable",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc custom variable"`, Update: `"test-acc custom variable updated"`},
		},
	},
	"kion_forecast": {
		TypeName: "kion_forecast",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_forecast_category": {
		TypeName: "kion_forecast_category",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_gcp_service_account": {
		TypeName: "kion_gcp_service_account",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_idms_group_association": {
		TypeName: "kion_idms_group_association",
	},
	"kion_ou_cloud_access_role": {
		TypeName: "kion_ou_cloud_access_role",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_project": {
		TypeName:        "kion_project",
		SDKGetMethod:    "GetProject",
		SDKGetParams:    "generated.GetProjectParams{ID: id}",
		SDKDeleteMethod: "DeleteProject",
		SDKDeleteParams: "generated.DeleteProjectParams{ID: id}",
		Dependencies: []Dependency{
			{
				TypeName:     "kion_ou",
				RefName:      "test_ou",
				Fields:       map[string]string{"name": `"test-acc-ou-%[1]s"`, "parent_ou_id": "0", "permission_scheme_id": "1"},
				RefAttribute: "id",
				TargetField:  "ou_id",
			},
		},
		FieldOverrides: map[string]FieldValue{
			"name":                  {Basic: `%[1]q`, Update: `%[1]q`},
			"description":           {Basic: `"test-acc project"`, Update: `"test-acc project updated"`},
			"permission_scheme_id":  {Basic: "1"},
			"default_aws_region":    {Basic: `"us-east-1"`},
			"project_permission_id": {Basic: "0"},
		},
		ExtraHCLBlocks: []string{
			"owner_users { id = 1 }",
		},
	},
	"kion_project_cloud_access_role": {
		TypeName: "kion_project_cloud_access_role",
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_project_line_item": {
		TypeName: "kion_project_line_item",
	},
}

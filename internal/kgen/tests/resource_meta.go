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

	// ExtraHCLBlocks are literal HCL lines appended inside the resource
	// (e.g. "owner_user_ids = [1]"). These are emitted verbatim, so they must
	// match the resource's own schema: the old provider's `owner_users { id = 1 }`
	// block syntax is not what any generated schema declares.
	ExtraHCLBlocks []string

	// RequiredEnv names environment variables the test cannot run without,
	// each with the reason shown in the skip message. A resource whose create
	// needs a record this install has no way to invent (a billing source, a
	// cloud account) skips loudly rather than failing or silently passing.
	//
	// These are guards only: the variable's value is not threaded into the HCL.
	// A test that must interpolate an install-specific id into its config is
	// hand-written, as internal/service/billing_rule already is.
	RequiredEnv []EnvRequirement

	// ImportIDParentField names the attribute holding the parent id for a
	// resource whose ImportState expects "<parent>/<id>" rather than a bare id
	// (the parent_list and association archetypes). Without it the import step
	// hands over rs.Primary.ID alone and the resource answers "Invalid import
	// ID".
	ImportIDParentField string

	// NoUpdate suppresses the _update test for a resource with no update
	// endpoint, whose Update method answers "cannot be updated in place;
	// changes force replacement". Such a test can only ever fail, and its
	// failure says nothing about the provider.
	NoUpdate bool

	// KnownIssues are emitted as a comment at the top of the generated test
	// file. Use it to name the open defect a test is expected to surface, so a
	// red test reads as a recorded finding rather than an unexplained failure.
	KnownIssues []string
}

// EnvRequirement is an environment variable a test needs, and why.
type EnvRequirement struct {
	// Name is the variable, e.g. "KION_ACC_BILLING_SOURCE_ID".
	Name string

	// Reason completes the sentence "<Name> must be set to ...".
	Reason string
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
	// POST /v3/category rejects a body without PayerID, so a category cannot be
	// created on an install with no billing source even though the schema (and
	// the spec it came from) marks payer_id optional.
	"kion_category": {
		TypeName:        "kion_category",
		SDKGetMethod:    "GetCategoryByID",
		SDKGetParams:    "generated.GetCategoryByIDParams{ID: id}",
		SDKDeleteMethod: "DeleteCategoryByID",
		SDKDeleteParams: "generated.DeleteCategoryByIDParams{ID: id}",
		RequiredEnv: []EnvRequirement{
			{Name: "KION_ACC_BILLING_SOURCE_ID", Reason: "a billing source on this install; POST /v3/category requires payer_id"},
		},
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	"kion_idms": {
		TypeName:        "kion_idms",
		SDKGetMethod:    "GetIDMS",
		SDKGetParams:    "generated.GetIDMSParams{ID: id}",
		SDKDeleteMethod: "DeleteIDMS",
		SDKDeleteParams: "generated.DeleteIDMSParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":         {Basic: `%[1]q`, Update: `%[1]q`},
			"idms_type_id": {Basic: "1"},
			// The update step has to change something the API stores: name is
			// held at rName across both configs, and idms_type_id is the
			// directory kind. Days-to-password-reset is the one free field.
			"password_expiration": {Basic: "90", Update: "60"},
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
		// POST /v3/user-group requires one of owner_user_ids /
		// owner_user_group_ids: without either it answers "Field validation for
		// 'OwnerUserIDs' failed on the 'atLeastOneFieldPresent' tag". The schema
		// marks both Optional, which is true of each alone but not of the pair,
		// so the constraint has to be met by the fixture (#62).
		ExtraHCLBlocks: []string{
			"owner_user_ids = [1]",
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
			"owner_user_ids = [1]",
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
	// An AMI is registered against an existing AWS account in Kion (account_id)
	// and a real image id, neither of which a test can invent.
	"kion_ami": {
		TypeName:        "kion_ami",
		SDKGetMethod:    "GetAMI",
		SDKGetParams:    "generated.GetAMIParams{ID: id}",
		SDKDeleteMethod: "DeleteAMI",
		SDKDeleteParams: "generated.DeleteAMIParams{ID: id}",
		RequiredEnv: []EnvRequirement{
			{Name: "KION_ACC_AWS_ACCOUNT_NUMBER", Reason: "an AWS account on this install; kion_ami registers an image against one"},
		},
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc AMI"`, Update: `"test-acc AMI updated"`},
			"aws_ami_id":  {Basic: `"ami-00000000000000000"`},
			"region":      {Basic: `"us-east-1"`},
		},
		ExtraHCLBlocks: []string{
			"owner_user_ids = [1]",
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
		// POST /v3/azure-role answers 500 for every role_permissions payload on
		// an install with no Azure billing source (raw curl reproduces it, so
		// it is not the provider). Gate on the Azure payer rather than let the
		// test fail with an undiagnosable server error.
		RequiredEnv: []EnvRequirement{
			{Name: "KION_ACC_AZURE_PAYER_ID", Reason: "an Azure billing source on this install; POST /v3/azure-role returns 500 without one"},
		},
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc Azure role"`, Update: `"test-acc Azure role updated"`},
			"role_permissions": {
				Basic:  `jsonencode([{ actions = ["Microsoft.Resources/subscriptions/read"], notActions = [] }])`,
				Update: `jsonencode([{ actions = ["Microsoft.Resources/subscriptions/resourceGroups/read"], notActions = [] }])`,
			},
		},
		ExtraHCLBlocks: []string{
			"owner_user_ids = [1]",
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
	// compliance_family and compliance_level hang off a program: their
	// collection is GET /v4/compliance/program/{id}/..., so each test stands up
	// its own program rather than assuming one exists on the install.
	"kion_compliance_family": {
		TypeName:        "kion_compliance_family",
		SDKGetMethod:    "GetComplianceFamily",
		SDKGetParams:    "generated.GetComplianceFamilyParams{ID: id}",
		SDKDeleteMethod: "DeleteComplianceFamily",
		SDKDeleteParams: "generated.DeleteComplianceFamilyParams{ID: id}",
		NoUpdate:        true,
		Dependencies: []Dependency{
			{
				TypeName:     "kion_compliance_program",
				RefName:      "test_program",
				Fields:       map[string]string{"name": `"test-acc-program-%[1]s"`, "version": `"1.0"`},
				RefAttribute: "id",
				TargetField:  "compliance_program_id",
			},
		},
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc family"`, Update: `"test-acc family updated"`},
		},
	},
	"kion_compliance_level": {
		TypeName:        "kion_compliance_level",
		SDKGetMethod:    "GetComplianceLevel",
		SDKGetParams:    "generated.GetComplianceLevelParams{ID: id}",
		SDKDeleteMethod: "DeleteComplianceLevel",
		SDKDeleteParams: "generated.DeleteComplianceLevelParams{ID: id}",
		NoUpdate:        true,
		Dependencies: []Dependency{
			{
				TypeName:     "kion_compliance_program",
				RefName:      "test_program",
				Fields:       map[string]string{"name": `"test-acc-program-%[1]s"`, "version": `"1.0"`},
				RefAttribute: "id",
				TargetField:  "compliance_program_id",
			},
		},
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc level"`, Update: `"test-acc level updated"`},
		},
	},
	"kion_compliance_program": {
		TypeName:        "kion_compliance_program",
		SDKGetMethod:    "GetComplianceProgram",
		SDKGetParams:    "generated.GetComplianceProgramParams{ID: id}",
		SDKDeleteMethod: "DeleteComplianceProgram",
		SDKDeleteParams: "generated.DeleteComplianceProgramParams{ID: id}",
		// PATCH /v4/compliance/program does not exist; the resource's Update
		// answers "cannot be updated in place; changes force replacement".
		NoUpdate: true,
		FieldOverrides: map[string]FieldValue{
			"name":    {Basic: `%[1]q`, Update: `%[1]q`},
			"version": {Basic: `"1.0"`, Update: `"2.0"`},
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
		NoUpdate:        true,
		KnownIssues: []string{
			"#69 amount is Optional+Computed and no flatten assigns it, so " +
				"ImportStateVerify fails. Create expands it into per-month data rows " +
				"that flattenBudget ignores. Left failing on purpose.",
		},
		// POST /v3/budget answers "project ID or OU ID is required", so the
		// test brings its own OU rather than assuming the install has one.
		Dependencies: []Dependency{
			{
				TypeName: "kion_ou",
				RefName:  "test_ou",
				Fields: map[string]string{
					"name":                 `"test-acc-ou-%[1]s"`,
					"parent_ou_id":         "0",
					"permission_scheme_id": "2",
					// POST /v3/ou requires at least one of owner_user_ids /
					// owner_user_group_ids.
					"owner_user_ids": "[1]",
				},
				RefAttribute: "id",
				TargetField:  "ou_id",
			},
		},
		FieldOverrides: map[string]FieldValue{
			"start_datecode": {Basic: `"2026-01"`},
			"end_datecode":   {Basic: `"2026-12"`, Update: `"2027-12"`},
		},
		ExtraHCLBlocks: []string{
			"amount = 1000",
		},
	},
	"kion_funding_source": {
		TypeName:        "kion_funding_source",
		SDKGetMethod:    "GetFundingSource",
		SDKGetParams:    "generated.GetFundingSourceParams{ID: id}",
		SDKDeleteMethod: "DeleteFundingSource",
		SDKDeleteParams: "generated.DeleteFundingSourceParams{ID: id}",
		KnownIssues: []string{
			"#68 owner_user_ids and permission_scheme_id are dropped on read, so " +
				"ImportStateVerify fails. GET /v3/funding-source/{id} returns neither; " +
				"the owners live at /v3/funding-source/{id}/permission-mapping, which " +
				"Read never calls. Left failing on purpose.",
		},
		FieldOverrides: map[string]FieldValue{
			"name":           {Basic: `%[1]q`, Update: `%[1]q`},
			"description":    {Basic: `"test-acc funding source"`, Update: `"test-acc funding source updated"`},
			"amount":         {Basic: "1000.00", Update: "2000.00"},
			"start_datecode": {Basic: `"2026-01"`},
			"end_datecode":   {Basic: `"2026-12"`, Update: `"2027-12"`},
		},
		// POST /v3/funding-source rejects a body without PolicyID even though the
		// spec marks permission_scheme_id optional; 4 is the system-managed
		// "Default Funding Source Permissions Scheme", present on every install.
		ExtraHCLBlocks: []string{
			"owner_user_ids = [1]",
			"permission_scheme_id = 4",
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
		// POST /v3/gcp-iam-role rejects a body without GCPRoleLaunchStage even
		// though the schema marks gcp_role_launch_stage optional. 2 is GA.
		ExtraHCLBlocks: []string{
			"owner_user_ids = [1]",
			`role_permissions = ["resourcemanager.projects.get"]`,
			"gcp_role_launch_stage = 2",
		},
	},
	"kion_service_control_policy": {
		TypeName:        "kion_service_control_policy",
		SDKGetMethod:    "GetServiceControlPolicy",
		SDKGetParams:    "generated.GetServiceControlPolicyParams{ID: id}",
		SDKDeleteMethod: "DeleteServiceControlPolicy",
		SDKDeleteParams: "generated.DeleteServiceControlPolicyParams{ID: id}",
		FieldOverrides: map[string]FieldValue{
			"name":        {Basic: `%[1]q`, Update: `%[1]q`},
			"description": {Basic: `"test-acc SCP"`, Update: `"test-acc SCP updated"`},
			// Written through jsonencode, not as a raw string. Reads
			// canonicalise JSON to the compact, key-sorted form jsonencode
			// emits (see codegen/schema_overrides.yaml); a hand-ordered literal
			// comes back reordered and the apply fails with "Provider produced
			// inconsistent result after apply".
			"policy": {
				Basic:  `jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Deny", Action = "s3:*", Resource = "*" }] })`,
				Update: `jsonencode({ Version = "2012-10-17", Statement = [{ Effect = "Deny", Action = "ec2:*", Resource = "*" }] })`,
			},
		},
		ExtraHCLBlocks: []string{
			"owner_user_ids = [1]",
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
	// A GCP service account is a record of one that already exists in Google:
	// email, gcp_project_id and unique_id are all Google-issued and cannot be
	// invented, so the test skips unless the install names a real one.
	"kion_gcp_service_account": {
		TypeName: "kion_gcp_service_account",
		RequiredEnv: []EnvRequirement{
			{Name: "KION_ACC_GCP_SERVICE_ACCOUNT_EMAIL", Reason: "the email of a real GCP service account; kion_gcp_service_account records a Google-issued identity that cannot be invented"},
		},
		FieldOverrides: map[string]FieldValue{
			"name": {Basic: `%[1]q`, Update: `%[1]q`},
		},
	},
	// Tests are HAND-WRITTEN (internal/service/account_linkage). A linkage row
	// carries a foreign key to `payer`, so without a billing source the create
	// fails on the constraint rather than on anything the provider did (#62).
	// The test interpolates KION_ACC_PAYER_ID into its HCL, which RequiredEnv
	// cannot do; the entry records the requirement so a regeneration does not
	// drop the gate silently.
	"kion_account_linkage": {
		TypeName: "kion_account_linkage",
		RequiredEnv: []EnvRequirement{
			{Name: "KION_ACC_PAYER_ID", Reason: "a billing source on this install; a linkage row references one by foreign key"},
		},
	},
	// Tests are HAND-WRITTEN (internal/service/idms_group_association). A group
	// association can only be created under a SAML IDMS — POST answers
	// "Bad Request: saml idms not found" otherwise — so the test cannot stand up
	// its own parent and must interpolate KION_ACC_SAML_IDMS_ID into its HCL,
	// which RequiredEnv cannot do. Left in the registry so a `kgen tests` run
	// does not treat the type as unknown; the files are skipped without --force.
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

	// Fully private note: all CRUD runs over raw HTTP against /v2, so there is
	// no SDK get method and the Exists/Destroy checks stay stubs. Apply and
	// ImportStateVerify are the real assertions here.
	"kion_funding_source_note": {
		TypeName: "kion_funding_source_note",
		KnownIssues: []string{
			"#71 every note is created with id 0: POST /v2/funding-source-note returns " +
				"{\"status\":201,\"data\":\"\"} and Create decodes a record_id that is not " +
				"there. Refresh then finds nothing and destroy deletes id 0. " +
				"Left failing on purpose.",
		},
		Dependencies: []Dependency{
			{
				TypeName: "kion_funding_source",
				RefName:  "test_fs",
				Fields: map[string]string{
					"name":                 `"test-acc-fs-%[1]s"`,
					"amount":               "1000.00",
					"start_datecode":       `"2026-01"`,
					"end_datecode":         `"2026-12"`,
					"owner_user_ids":       "[1]",
					"permission_scheme_id": "4",
				},
				RefAttribute: "id",
				TargetField:  "funding_source_id",
			},
		},
		// Every attribute is Optional, so nothing is emitted without this.
		ExtraHCLBlocks: []string{
			`name = "test-acc-note"`,
			`text = "test-acc note body"`,
		},
	},

	// ── Enforcements: parent_list, compound "<parent>/<id>" import ───────
	// Each stands its own parent up, so neither depends on what happens to
	// exist on the install.
	"kion_funding_source_enforcement": {
		TypeName:            "kion_funding_source_enforcement",
		ImportIDParentField: "funding_source_id",
		KnownIssues: []string{
			"#70 cloud_rule_id is dropped on read (the record carries the cloud rule " +
				"as a nested object and flatten never unwraps it), so the _update " +
				"test's ImportStateVerify fails. Left failing on purpose.",
		},
		Dependencies: []Dependency{
			{
				TypeName: "kion_funding_source",
				RefName:  "test_fs",
				Fields: map[string]string{
					"name":           `"test-acc-fs-%[1]s"`,
					"amount":         "1000.00",
					"start_datecode": `"2026-01"`,
					"end_datecode":   `"2026-12"`,
					"owner_user_ids": "[1]",
					// POST /v3/funding-source requires PolicyID; 4 is the
					// system-managed funding source scheme.
					"permission_scheme_id": "4",
				},
				RefAttribute: "id",
				TargetField:  "funding_source_id",
			},
		},
		FieldOverrides: map[string]FieldValue{
			"threshold": {Basic: "100", Update: "200"},
			"timeframe": {Basic: `"month"`},
		},
		ExtraHCLBlocks: []string{
			// "required if no user group IDs are listed"
			"user_ids = [1]",
		},
	},
	"kion_ou_enforcement": {
		TypeName:            "kion_ou_enforcement",
		ImportIDParentField: "ou_id",
		KnownIssues: []string{
			"#70 cloud_rule_id and service_id are dropped on read (the record carries " +
				"both as nested objects and flatten never unwraps them), so the _update " +
				"test's ImportStateVerify fails. Left failing on purpose.",
		},
		Dependencies: []Dependency{
			{
				TypeName: "kion_ou",
				RefName:  "test_ou",
				Fields: map[string]string{
					"name":                 `"test-acc-ou-%[1]s"`,
					"parent_ou_id":         "0",
					"permission_scheme_id": "2",
					"owner_user_ids":       "[1]",
				},
				RefAttribute: "id",
				TargetField:  "ou_id",
			},
		},
		FieldOverrides: map[string]FieldValue{
			"threshold": {Basic: "100", Update: "200"},
			"timeframe": {Basic: `"month"`},
		},
		ExtraHCLBlocks: []string{
			"user_ids = [1]",
		},
	},
}

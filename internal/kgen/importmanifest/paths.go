package importmanifest

// The tables below are AUTHORED, not derived. generator_config.yaml records a
// by-id read per resource, which is precisely what parent-scoped, association
// and special resources do not have -- so their collection endpoints cannot be
// derived from it.
//
// That makes this file the plan's highest-risk surface: a wrong path here is a
// resource silently missing from an import. cmd/kion-import's --probe mode
// exists to check every one of them against a live install.

// ParentPaths gives the parent enumeration and child collection for every
// resource read per-parent (parent_list, compound_key_parent_read, association).
var ParentPaths = map[string]Parent{
	// parent_list / compound_key_parent_read
	"kion_ou_enforcement": {
		Kind: "ou", ListPath: "/v3/ou",
		ChildPath: "/v3/ou/{parent_id}/enforcement", ParentIDField: "ou_id",
	},
	"kion_project_enforcement": {
		Kind: "project", ListPath: "/v3/project",
		ChildPath: "/v3/project/{parent_id}/enforcement", ParentIDField: "project_id",
	},
	"kion_funding_source_enforcement": {
		Kind: "funding_source", ListPath: "/v3/funding-source",
		ChildPath: "/v3/funding-source/{parent_id}/enforcement", ParentIDField: "funding_source_id",
	},
	"kion_scope_criteria": {
		Kind: "scope", ListPath: "/beta/scope",
		ChildPath: "/beta/scope/{parent_id}/criteria", ParentIDField: "scope_id",
	},

	// association
	"kion_ou_permission_mapping": {
		Kind: "ou", ListPath: "/v3/ou",
		ChildPath: "/v3/ou/{parent_id}/permission-mapping", ParentIDField: "ou_id",
	},
	"kion_project_permission_mapping": {
		Kind: "project", ListPath: "/v3/project",
		ChildPath: "/v3/project/{parent_id}/permission-mapping", ParentIDField: "project_id",
	},
	"kion_funding_source_permission_mapping": {
		Kind: "funding_source", ListPath: "/v3/funding-source",
		ChildPath: "/v3/funding-source/{parent_id}/permission-mapping", ParentIDField: "funding_source_id",
	},

	// Fallbacks for two resources whose flat list 405s on real installs even
	// though codegen describes them as ordinary entities. Recorded in
	// kion-env-copy/docs/ONBOARDING_REPORT.md as a live corrective pass.
	"kion_compliance_family": {
		Kind: "compliance_program", ListPath: "/v4/compliance/program",
		ChildPath: "/v4/compliance/program/{parent_id}/family", ParentIDField: "compliance_program_id",
	},
	"kion_compliance_level": {
		Kind: "compliance_program", ListPath: "/v4/compliance/program",
		ChildPath: "/v4/compliance/program/{parent_id}/level", ParentIDField: "compliance_program_id",
	},
}

// SpecialPaths gives the bespoke list (or singleton) endpoint per special-archetype resource.
var SpecialPaths = map[string]string{
	"kion_app_config":               "/v3/app-config",
	"kion_dashboard":                "/beta/dashboard",
	"kion_funding_source_note":      "/v3/funding-source-note",
	"kion_custom_variable_override": "/v3/custom-variable-override",
}

// ExtraListPaths covers resources the provider serves under a name
// generator_config.yaml does not use: kion_account (codegen models the
// per-provider aws/azure/gcp/custom account resources instead),
// kion_aws_cloudformation_template (codegen: cft) and kion_aws_iam_policy
// (codegen: iam_policy). Keyed by the codegen kind derived from the tf_type.
var ExtraListPaths = map[string]string{
	"account":                     "/v3/account",
	"aws_cloudformation_template": "/v3/cft",
	"aws_iam_policy":              "/v3/iam-policy",
}

// GlobalAssociationPaths are association resources with no parent to enumerate.
var GlobalAssociationPaths = map[string]string{
	"kion_global_permission_mapping": "/v3/global-permission-mapping",
}

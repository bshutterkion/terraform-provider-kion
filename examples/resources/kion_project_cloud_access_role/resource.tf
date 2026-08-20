resource "kion_project_cloud_access_role" "example" {
  # Required
  name       = "example"
  project_id = 1

  # Optional
  # account_ids                  = []
  # apply_to_all_accounts        = false
  # aws_iam_path                 = "example"
  # aws_iam_permissions_boundary = 1
  # aws_iam_policies             = []
  # aws_iam_role_name            = "example"
  # aws_session_tags = {
  #   cloud_access_role_id    = 1
  #   id                      = 1
  #   ou_cloud_access_role_id = 1
  #   tag_key                 = "example"
  #   tag_value               = "example"
  # }
  # azure_role_definitions       = []
  # cloud_provider_ids           = []
  # future_accounts              = false
  # gcp_iam_roles                = []
  # last_updated                 = "example"
  # long_term_access_keys        = false
  # policytype                   = "example"
  # short_term_access_keys       = false
  # user_group_ids               = []
  # user_ids                     = []
  # web_access                   = false
}

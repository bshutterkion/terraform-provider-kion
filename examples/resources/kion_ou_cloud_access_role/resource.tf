resource "kion_ou_cloud_access_role" "example" {
  # Required
  name  = "example"
  ou_id = 1

  # Optional
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
  # gcp_iam_roles                = []
  # last_updated                 = "example"
  # long_term_access_keys        = false
  # short_term_access_keys       = false
  # user_group_ids               = []
  # user_ids                     = []
  # web_access                   = false
}

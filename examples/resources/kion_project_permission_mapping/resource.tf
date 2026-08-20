resource "kion_project_permission_mapping" "example" {
  # Required
  app_role_id     = 1
  project_id      = 1
  user_groups_ids = []
  user_ids        = []
}

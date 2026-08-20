data "kion_global_permission_mappings" "all" {}

output "global_permission_mappings" {
  value = data.kion_global_permission_mappings.all.data
}

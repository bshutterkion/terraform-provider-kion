# ============================================
# Label Resources
# ============================================

resource "kion_label" "env_production" {
  key   = "Environment"
  value = "Production"
  color = "#28a745"
}

resource "kion_label" "env_staging" {
  key   = "Environment"
  value = "Staging"
  color = "#ffc107"
}

resource "kion_label" "team_platform" {
  key   = "Team"
  value = "Platform Engineering"
  color = "#0366d6"
}

output "label_ids" {
  value = {
    production = kion_label.env_production.id
    staging    = kion_label.env_staging.id
    platform   = kion_label.team_platform.id
  }
}

# ============================================
# Label Data Source
# ============================================

data "kion_label" "production_lookup" {
  id = kion_label.env_production.id
}

output "label_data_source" {
  value = {
    id    = data.kion_label.production_lookup.id
    key   = data.kion_label.production_lookup.key
    value = data.kion_label.production_lookup.value
    color = data.kion_label.production_lookup.color
  }
}

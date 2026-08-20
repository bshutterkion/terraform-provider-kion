# Plan-only. The provider is configured with a placeholder endpoint and is
# never contacted, so no Kion credentials are required.

provider "kion" {
  api_url = "http://127.0.0.1:1"
  api_key = "test"
}

run "plan" {
  command = plan

  variables {
    billing_source_ids = []
    description        = "example"
    name               = "example"
    rule_type          = 1
    rule_value         = 1
    start_month        = 1
  }
}

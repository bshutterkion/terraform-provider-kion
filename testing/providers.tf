terraform {
  required_providers {
    kion = {
      source = "kionsoftware/kion"
    }
  }
}

provider "kion" {
  api_url = var.kion_url
  api_key = var.kion_apikey
}

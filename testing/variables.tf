variable "kion_url" {
  description = "The URL of the Kion instance"
  type        = string
  default     = ""
}

variable "kion_apikey" {
  description = "The API key for the Kion instance"
  type        = string
  default     = ""
  sensitive   = true
}

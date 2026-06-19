terraform {
  required_providers {
    omv = {
      source = "sandy-rt/omv"
    }
  }
}

# Credentials can also be supplied via the OMV_ENDPOINT / OMV_USERNAME /
# OMV_PASSWORD environment variables instead of the arguments below.
provider "omv" {
  endpoint = "http://omv.example.com"
  username = "admin"
  password = var.omv_password

  # Safety: when false (default), the provider refuses to delete shares,
  # even on `terraform destroy`. Set true only to intentionally remove shares.
  allow_destroy = false
}

variable "omv_password" {
  type      = string
  sensitive = true
}

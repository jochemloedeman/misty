terraform {
  required_version = ">= 1.5.7"

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "1.60.1"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "5.18.0"
    }
  }
}

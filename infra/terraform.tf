terraform {
  required_version = ">= 1.11"

  backend "pg" {}

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.60"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.18"
    }
    tailscale = {
      source  = "tailscale/tailscale"
      version = "~> 0.28"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.1"
    }
  }
}

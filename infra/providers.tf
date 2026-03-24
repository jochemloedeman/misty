provider "hcloud" {}

provider "cloudflare" {}

provider "tailscale" {
  tailnet = var.tailscale_tailnet
}

provider "tls" {}

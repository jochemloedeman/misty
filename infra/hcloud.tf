locals {
  cloudflare_ips = concat(
    data.cloudflare_ip_ranges.this.ipv4_cidrs,
    data.cloudflare_ip_ranges.this.ipv6_cidrs,
  )
}

data "cloudflare_ip_ranges" "this" {}

resource "hcloud_server" "this" {
  name        = "misty"
  image       = "ubuntu-24.04"
  server_type = "cx23"
  location    = "fsn1"
  ssh_keys    = [hcloud_ssh_key.this.id]
  backups     = true

  user_data = templatefile("${path.module}/templates/cloud-init.yml.tftpl", {
    tailscale_authkey = tailscale_tailnet_key.this.key
  })

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }

  lifecycle {
    prevent_destroy = true
    ignore_changes = [
      user_data
    ]
  }
}

resource "hcloud_ssh_key" "this" {
  name       = "misty"
  public_key = file(var.ssh_public_key_path)
}


resource "hcloud_firewall" "this" {
  name = "misty"

  rule {
    description = "Allow HTTPS from Cloudflare"
    direction   = "in"
    protocol    = "tcp"
    port        = "443"
    source_ips  = local.cloudflare_ips
  }

  rule {
    description = "Allow ICMP"
    direction   = "in"
    protocol    = "icmp"
    source_ips  = ["0.0.0.0/0", "::/0"]
  }

  apply_to {
    server = hcloud_server.this.id
  }
}

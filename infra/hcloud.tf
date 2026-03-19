locals {
  cloudflare_ips = concat(
    data.cloudflare_ip_ranges.this.ipv4_cidr_blocks,
    data.cloudflare_ip_ranges.this.ipv6_cidr_blocks,
  )
}

data "cloudflare_ip_ranges" "this" {}

resource "hcloud_floating_ip" "this" {
  type = "ipv4"
}

resource "hcloud_floating_ip_assignment" "this" {
  floating_ip_id = hcloud_floating_ip.this.id
  server_id      = hcloud_server.this.id
}

resource "hcloud_server" "this" {
  name        = "misty"
  image       = "debian-13"
  server_type = "cx23"
  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
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
    description = "Allow SSH from trusted IPs"
    direction   = "in"
    protocol    = "tcp"
    port        = "22"
    source_ips  = var.ssh_allowed_ips
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

variable "cloudflare_zone_id" {
  description = "Cloudflare zone ID for DNS records"
  type        = string
}

variable "cloudflare_account_id" {
  description = "Cloudflare account ID for R2 storage"
  type        = string
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key file"
  type        = string
}

variable "tailscale_tailnet" {
  description = "Tailscale tailnet name (e.g. example.com)"
  type        = string
}


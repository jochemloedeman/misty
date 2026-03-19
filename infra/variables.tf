variable "cloudflare_zone_id" {
  description = "Zone ID for jochemapps.com in Cloudflare"
  type        = string
}

variable "ssh_public_key_path" {
  description = "Path to the SSH public key file"
  type        = string
}

variable "ssh_allowed_ips" {
  description = "CIDR blocks allowed to SSH into the server (e.g. [\"203.0.113.10/32\"])"
  type        = list(string)
}

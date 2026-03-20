resource "tailscale_tailnet_key" "this" {
  ephemeral     = true
  reusable      = false
  preauthorized = true
  description   = "misty server"
  tags          = ["tag:server"]
}

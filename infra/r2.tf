data "cloudflare_account_api_token_permission_groups_list" "r2_write" {
  account_id = var.cloudflare_account_id
  name       = "Workers R2 Storage Bucket Item Write"
}

data "cloudflare_account_api_token_permission_groups_list" "r2_read" {
  account_id = var.cloudflare_account_id
  name       = "Workers R2 Storage Bucket Item Read"
}

resource "cloudflare_r2_bucket" "this" {
  account_id    = var.cloudflare_account_id
  name          = "misty-backups"
  location      = "weur"
  storage_class = "Standard"
}

resource "cloudflare_account_token" "r2_backup" {
  account_id = var.cloudflare_account_id
  name       = "misty-backup"

  policies = [{
    effect = "allow"
    permission_groups = [
      { id = data.cloudflare_account_api_token_permission_groups_list.r2_write.result[0].id },
      { id = data.cloudflare_account_api_token_permission_groups_list.r2_read.result[0].id },
    ]
    resources = jsonencode({
      "com.cloudflare.api.account.${var.cloudflare_account_id}" = "*"
    })
  }]
}

# R2's S3 API derives credentials from the token: the access key id is the token
# id, the secret is the SHA-256 hex of the token value.
locals {
  r2_access_key_id     = cloudflare_account_token.r2_backup.id
  r2_secret_access_key = sha256(cloudflare_account_token.r2_backup.value)
}

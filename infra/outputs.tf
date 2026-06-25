output "misty_hostname" {
  description = "FQDN for the misty backend"
  value       = cloudflare_dns_record.misty.name
}

output "aop_ca_cert_pem" {
  description = "CA certificate for verifying Cloudflare AOP client certificates"
  value       = tls_self_signed_cert.aop_ca.cert_pem
  sensitive   = true
}

output "r2_bucket" {
  description = "R2 bucket name for database backups"
  value       = cloudflare_r2_bucket.this.name
}

output "r2_endpoint" {
  description = "S3 endpoint for the R2 backup bucket"
  value       = "https://${var.cloudflare_account_id}.r2.cloudflarestorage.com"
}

output "r2_access_key_id" {
  description = "S3 access key id for the R2 backup bucket"
  value       = local.r2_access_key_id
  sensitive   = true
}

output "r2_secret_access_key" {
  description = "S3 secret access key for the R2 backup bucket"
  value       = local.r2_secret_access_key
  sensitive   = true
}

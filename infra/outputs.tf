output "misty_hostname" {
  description = "FQDN for the misty backend"
  value       = cloudflare_dns_record.misty.name
}

output "aop_ca_cert_pem" {
  description = "CA certificate for verifying Cloudflare AOP client certificates"
  value       = tls_self_signed_cert.aop_ca.cert_pem
  sensitive   = true
}

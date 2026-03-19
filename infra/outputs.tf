output "misty_hostname" {
  description = "FQDN for the misty backend"
  value       = cloudflare_dns_record.misty.name
}

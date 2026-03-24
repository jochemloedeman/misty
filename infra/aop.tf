resource "tls_private_key" "aop_ca" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "aop_ca" {
  private_key_pem = tls_private_key.aop_ca.private_key_pem

  subject {
    common_name  = "Misty AOP CA"
    organization = "Misty"
  }

  validity_period_hours = 87600
  is_ca_certificate     = true
  allowed_uses          = ["cert_signing", "crl_signing"]
}

resource "tls_private_key" "aop_client" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "aop_client" {
  private_key_pem = tls_private_key.aop_client.private_key_pem

  subject {
    common_name  = "Misty AOP Client"
    organization = "Misty"
  }
}

resource "tls_locally_signed_cert" "aop_client" {
  cert_request_pem      = tls_cert_request.aop_client.cert_request_pem
  ca_private_key_pem    = tls_private_key.aop_ca.private_key_pem
  ca_cert_pem           = tls_self_signed_cert.aop_ca.cert_pem
  validity_period_hours = 43800
  allowed_uses          = ["client_auth"]
}

resource "cloudflare_authenticated_origin_pulls_certificate" "this" {
  zone_id     = var.cloudflare_zone_id
  certificate = tls_locally_signed_cert.aop_client.cert_pem
  private_key = tls_private_key.aop_client.private_key_pem

  lifecycle {
    ignore_changes = [private_key]
  }
}

resource "cloudflare_authenticated_origin_pulls_settings" "this" {
  zone_id = var.cloudflare_zone_id
  enabled = true
}

resource "cloudflare_dns_record" "misty" {
  zone_id = var.cloudflare_zone_id
  name    = "misty"
  type    = "A"
  content = hcloud_floating_ip.this.ip_address
  proxied = true
  ttl     = 1
}

resource "cloudflare_zone_setting" "ssl" {
  zone_id    = var.cloudflare_zone_id
  setting_id = "ssl"
  value      = "strict"
}

resource "cloudflare_zone_setting" "always_use_https" {
  zone_id    = var.cloudflare_zone_id
  setting_id = "always_use_https"
  value      = "on"
}

resource "cloudflare_zone_setting" "min_tls_version" {
  zone_id    = var.cloudflare_zone_id
  setting_id = "min_tls_version"
  value      = "1.2"
}

resource "cloudflare_zone_setting" "tls_1_3" {
  zone_id    = var.cloudflare_zone_id
  setting_id = "tls_1_3"
  value      = "zrt"
}

resource "cloudflare_ruleset" "waf_managed" {
  zone_id = var.cloudflare_zone_id
  name    = "Managed WAF ruleset"
  kind    = "zone"
  phase   = "http_request_firewall_managed"

  rules = [{
    ref        = "execute_cloudflare_managed_ruleset"
    expression = "true"
    action     = "execute"
    action_parameters = {
      id = "77454fe2d30c4220b5701f6fdfb893ba"
    }
  }]
}

resource "cloudflare_ruleset" "rate_limiting" {
  zone_id = var.cloudflare_zone_id
  name    = "Rate limiting rules"
  kind    = "zone"
  phase   = "http_ratelimit"

  rules = [{
    ref        = "rate_limit_catch_all"
    expression = "true"
    action     = "block"
    action_parameters = {
      response = {
        status_code  = 429
        content      = "{\"error\": \"Too many requests\"}"
        content_type = "application/json"
      }
    }
    ratelimit = {
      characteristics     = ["ip.src", "cf.colo.id"]
      period              = 10
      requests_per_period = 8
      mitigation_timeout  = 10
    }
  }]
}

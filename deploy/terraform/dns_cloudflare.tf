# Cloudflare Load Balancing — health-checked DNS steering for the wildcard tunnel
# record, as an upgrade over the plain DigitalOcean round-robin in dns.tf.
#
# ─────────────────────────────────────────────────────────────────────────────
# EVERYTHING HERE IS INERT until `enable_cloudflare_lb = true`. The default DO-only
# topology is unchanged. To activate (owner action — cannot be done/tested here):
#
#   1. Host var.domain's DNS at Cloudflare (this repo already recommends Cloudflare
#      DNS-only / "grey cloud" — orange-cloud proxying breaks trqsh's own TLS/QUIC/
#      wildcard, so the load balancer below is created with proxied = false, which
#      returns plain A-record answers and only adds health-checked steering).
#   2. Have a Cloudflare plan that includes Load Balancing (it is a paid add-on).
#      NOTE: a WILDCARD load-balancer hostname (`*.<domain>`) requires an
#      Enterprise-level zone; on lower tiers create per-hostname load balancers, or
#      keep the DO wildcard round-robin and use Cloudflare LB only for apex/api.
#   3. Set these variables (terraform.tfvars or TF_VAR_*):
#         enable_cloudflare_lb  = true
#         cloudflare_api_token  = "<token: Load Balancing + DNS edit>"
#         cloudflare_account_id = "<account id>"
#         cloudflare_zone_id    = "<zone id for var.domain>"
#   4. `terraform apply`. Enabling this automatically drops the DO wildcard records
#      (dns.tf) so the two never both own *.<domain>.
#
# Health checking: the default monitor is a TCP check on :443 (cloudflare_lb_monitor_type
# = "tcp"), which verifies the public HTTPS ingress is accepting connections and
# needs NO new firewall exposure. The edge's application /healthz lives on the
# internal metrics port (:9090, firewalled off), so to health-check it instead set
# cloudflare_lb_monitor_type = "http", cloudflare_lb_monitor_port = 9090, and open
# 9090 to Cloudflare's published health-check IP ranges in edge.tf's firewall (that
# also exposes /metrics to those ranges — weigh it before switching).
# ─────────────────────────────────────────────────────────────────────────────

# Health monitor for the edge pool (account-scoped in the v4 provider).
resource "cloudflare_load_balancer_monitor" "edge" {
  count      = var.enable_cloudflare_lb ? 1 : 0
  account_id = var.cloudflare_account_id

  type = var.cloudflare_lb_monitor_type
  port = var.cloudflare_lb_monitor_port

  # http/https-only fields are null for a tcp monitor (which just confirms the TCP
  # handshake completes).
  method         = var.cloudflare_lb_monitor_type == "tcp" ? "connection_established" : "GET"
  path           = var.cloudflare_lb_monitor_type == "tcp" ? null : var.cloudflare_lb_monitor_path
  expected_codes = var.cloudflare_lb_monitor_type == "tcp" ? null : var.cloudflare_lb_monitor_expected_codes

  interval    = 60
  timeout     = 5
  retries     = 2
  description = "trqsh edge liveness"
}

# Pool of edge origins = every edge reserved IP. minimum_origins = 1 keeps the pool
# healthy as long as one edge answers; Cloudflare steers away from unhealthy IPs.
resource "cloudflare_load_balancer_pool" "edge" {
  count      = var.enable_cloudflare_lb ? 1 : 0
  account_id = var.cloudflare_account_id
  name       = "${var.project_name}-edges"
  monitor    = cloudflare_load_balancer_monitor.edge[0].id

  dynamic "origins" {
    for_each = digitalocean_reserved_ip.edge
    content {
      name    = "edge-${origins.key}"
      address = origins.value.ip_address
      enabled = true
    }
  }

  minimum_origins = 1
  description     = "trqsh edge reserved IPs"
}

# The load balancer itself, bound to the wildcard tunnel hostname. proxied = false
# is the whole point: DNS-only steering that returns A-record-style answers instead
# of routing traffic through Cloudflare's proxy (which would break trqsh's custom
# TLS/QUIC). steering_policy drives failover vs. latency selection.
resource "cloudflare_load_balancer" "wildcard" {
  count   = var.enable_cloudflare_lb ? 1 : 0
  zone_id = var.cloudflare_zone_id
  name    = "*.${var.domain}"

  default_pool_ids = [cloudflare_load_balancer_pool.edge[0].id]
  fallback_pool_id = cloudflare_load_balancer_pool.edge[0].id

  # ttl is intentionally omitted (defaults to "automatic") — it conflicts
  # with proxied in the provider schema, and proxied=false is the setting
  # that actually matters here (see the file header).
  proxied         = false
  steering_policy = var.cloudflare_lb_steering_policy

  description = "trqsh wildcard edge steering (DNS-only)"
}

output "cloudflare_lb_hostname" {
  description = "Wildcard hostname steered by Cloudflare LB (empty unless enable_cloudflare_lb)."
  # try() avoids an index-out-of-range on the count=0 (disabled) case.
  value = try(cloudflare_load_balancer.wildcard[0].name, "")
}

# DNS: the apex domain, the wildcard tunnel record (*.trqsh.uz) round-robined
# across every edge reserved IP, and the api/app hostnames.
#
# NOTE: DO DNS is not GeoDNS. For latency-based routing to the nearest edge, use
# a GeoDNS/anycast provider (NS1, Route 53, Cloudflare) for the wildcard in prod;
# the round-robin here spreads load across regions as a baseline.
resource "digitalocean_domain" "trqsh" {
  name = var.domain
}

# Wildcard tunnels: one A record per edge reserved IP (DNS round-robin).
resource "digitalocean_record" "wildcard" {
  for_each = digitalocean_reserved_ip.edge

  domain = digitalocean_domain.trqsh.name
  type   = "A"
  name   = "*"
  value  = each.value.ip_address
  ttl    = 60
}

# Control-plane hostnames → ingress LB (set control_ingress_ip after helm install,
# or manage with external-dns and leave the var blank).
resource "digitalocean_record" "api" {
  count  = var.control_ingress_ip != "" ? 1 : 0
  domain = digitalocean_domain.trqsh.name
  type   = "A"
  name   = "api"
  value  = var.control_ingress_ip
  ttl    = 300
}

resource "digitalocean_record" "app" {
  count  = var.control_ingress_ip != "" ? 1 : 0
  domain = digitalocean_domain.trqsh.name
  type   = "A"
  name   = "app"
  value  = var.control_ingress_ip
  ttl    = 300
}

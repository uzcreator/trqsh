# Per-region edge droplets. Each runs the edge container on the host network so
# it owns :80/:443 (TCP+UDP/QUIC). A reserved IP per droplet gives GeoDNS/anycast
# stable targets (dns.tf round-robins the wildcard across them, or Cloudflare LB
# steers across them — see dns_cloudflare.tf). Edges also form Stage D's
# cross-edge forwarding mesh over an internal, token-authenticated port.

locals {
  # Flatten regions × nodes into a map keyed "region-idx".
  edge_nodes = merge([
    for region in var.edge_regions : {
      for i in range(var.edge_nodes_per_region) :
      "${region}-${i}" => { region = region, index = i }
    }
  ]...)
}

# cloud-init: install Docker and run the edge, wired to the managed data plane
# and the public control API (authenticated by the internal token).
#
# Stage C (shared TLS): TRQSH_REDIS_URL is passed below, which makes the edge use
# the shared Redis cert store (buildCertStorage in internal/server/tls_acme.go)
# AUTOMATICALLY — every edge then shares one ACME cert cache + issuance lock rather
# than each ordering the same names from Let's Encrypt. No cert dir or volume is
# needed on the droplet; the Redis URL alone activates it.
#
# Stage D (cross-edge forwarding): TRQSH_FORWARD_ADDR opens the internal,
# token-authenticated forwarding listener; TRQSH_FORWARD_ADVERTISE_ADDR is the
# address peer edges dial to reach it — this droplet's ${var.edge_forward_iface} IP,
# read from the DO metadata service at boot (the reserved-IP resource can't be
# referenced in a droplet's own user_data without a dependency cycle, and the edge
# re-advertises the address on every heartbeat anyway). The firewall (below)
# restricts the port to edge droplets, so it is never exposed to the internet.
locals {
  edge_user_data = { for k, n in local.edge_nodes : k => <<-EOT
    #cloud-config
    package_update: true
    packages: [docker.io]
    runcmd:
      - systemctl enable --now docker
      - |
        FWD_IP=$(curl -sf http://169.254.169.254/metadata/v1/interfaces/${var.edge_forward_iface}/0/ipv4/address)
        docker run -d --restart=always --name trqsh-edge --network host \
          -e TRQSH_BASE_DOMAIN=${var.domain} \
          -e TRQSH_REGION=${n.region} \
          -e TRQSH_ENTITLEMENTS=api \
          -e TRQSH_API_URL=https://api.${var.domain} \
          -e TRQSH_INTERNAL_TOKEN=${var.internal_token} \
          -e TRQSH_REDIS_URL=${digitalocean_database_cluster.redis.uri} \
          -e TRQSH_ACME_STAGING=0 \
          -e TRQSH_ACME_EMAIL=${var.acme_email} \
          -e TRQSH_METRICS_ADDR=:9090 \
          -e TRQSH_FORWARD_ADDR=:${var.edge_forward_port} \
          -e TRQSH_FORWARD_ADVERTISE_ADDR=$${FWD_IP}:${var.edge_forward_port} \
          ${var.edge_image}
  EOT
  }
}

# A managed tag identifying edge droplets. Used as the firewall source for the
# inter-edge forwarding port so ONLY edges (not the public internet) can reach it —
# DO tag firewalls match the source droplet's identity, so this also works between
# edges in different regions (DO VPCs are single-region and can't).
resource "digitalocean_tag" "edge" {
  name = "${var.project_name}-edge"
}

resource "digitalocean_droplet" "edge" {
  for_each = local.edge_nodes

  name      = "${var.project_name}-edge-${each.key}"
  region    = each.value.region
  size      = var.edge_node_size
  image     = "ubuntu-24-04-x64"
  ssh_keys  = var.ssh_key_fingerprints
  user_data = local.edge_user_data[each.key]
  tags      = [digitalocean_tag.edge.name, "trqsh", "region:${each.value.region}"]
}

# Stable public IP per edge droplet.
resource "digitalocean_reserved_ip" "edge" {
  for_each = digitalocean_droplet.edge
  region   = each.value.region
}

resource "digitalocean_reserved_ip_assignment" "edge" {
  for_each   = digitalocean_droplet.edge
  ip_address = digitalocean_reserved_ip.edge[each.key].ip_address
  droplet_id = each.value.id
}

# Cloud firewall: allow public ingress on the product ports only. SSH is NOT in
# this public set — it's a separate, opt-in rule below gated by
# ssh_allowed_cidrs, so a fresh apply ships with no SSH ingress at all rather
# than defaulting open. The inter-edge forwarding port is also deliberately NOT
# in the public set — it gets its own edge-only rule.
resource "digitalocean_firewall" "edge" {
  name        = "${var.project_name}-edge"
  droplet_ids = [for d in digitalocean_droplet.edge : d.id]

  dynamic "inbound_rule" {
    for_each = [
      { proto = "tcp", ports = "80" },
      { proto = "tcp", ports = "443" },
      # HTTP/3 (QUIC) ingress shares :443 with HTTPS, over UDP.
      { proto = "udp", ports = "443" },
      { proto = "tcp", ports = "4443" },
      { proto = "udp", ports = "4443" },
    ]
    content {
      protocol         = inbound_rule.value.proto
      port_range       = inbound_rule.value.ports
      source_addresses = ["0.0.0.0/0", "::/0"]
    }
  }

  # SSH: only opened when ssh_allowed_cidrs is set (e.g. to a bastion or admin
  # IP/VPN range). Leave it unset to ship edges with no SSH ingress rule at
  # all; use the DO web console / recovery mode for out-of-band access.
  dynamic "inbound_rule" {
    for_each = length(var.ssh_allowed_cidrs) > 0 ? [1] : []
    content {
      protocol         = "tcp"
      port_range       = "22"
      source_addresses = var.ssh_allowed_cidrs
    }
  }

  # Stage D inter-edge forwarding: token-authenticated internal hop. Restricted to
  # droplets carrying the edge tag (NOT 0.0.0.0/0) — defense-in-depth on top of the
  # shared-token auth, so the port is never reachable from the public internet even
  # though the listener binds the public interface under host networking.
  inbound_rule {
    protocol    = "tcp"
    port_range  = tostring(var.edge_forward_port)
    source_tags = [digitalocean_tag.edge.name]
  }

  # Egress restricted to what an edge actually needs to originate outbound:
  # HTTP(S) for apt package installs, the GHCR edge-image pull, calls to the
  # control API, ACME/Let's Encrypt, and the Cloudflare API (DNS-01 + zone
  # lookups); DNS resolution; and the managed Postgres/Redis ports (DO managed
  # databases always use 25060/25061, see
  # https://docs.digitalocean.com/products/databases — edges outside
  # primary_region reach them over the public endpoint since DO VPCs are
  # single-region). Tunneled traffic itself flows back down the
  # already-established agent session, not a fresh outbound connection, so it
  # needs no broader allowance. Review against your actual egress needs before
  # applying to real infrastructure.
  dynamic "outbound_rule" {
    for_each = [
      { proto = "tcp", ports = "80" },
      { proto = "tcp", ports = "443" },
      { proto = "tcp", ports = "53" },
      { proto = "udp", ports = "53" },
      { proto = "tcp", ports = "25060-25061" },
    ]
    content {
      protocol              = outbound_rule.value.proto
      port_range            = outbound_rule.value.ports
      destination_addresses = ["0.0.0.0/0", "::/0"]
    }
  }
}

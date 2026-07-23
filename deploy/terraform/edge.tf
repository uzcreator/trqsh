# Per-region edge droplets. Each runs the edge container on the host network so
# it owns :80/:443 (TCP+UDP/QUIC). A reserved IP per droplet gives GeoDNS/anycast
# stable targets (dns.tf round-robins the wildcard across them).

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
locals {
  edge_user_data = { for k, n in local.edge_nodes : k => <<-EOT
    #cloud-config
    package_update: true
    packages: [docker.io]
    runcmd:
      - systemctl enable --now docker
      - |
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
          ${var.edge_image}
  EOT
  }
}

resource "digitalocean_droplet" "edge" {
  for_each = local.edge_nodes

  name      = "${var.project_name}-edge-${each.key}"
  region    = each.value.region
  size      = var.edge_node_size
  image     = "ubuntu-24-04-x64"
  ssh_keys  = var.ssh_key_fingerprints
  user_data = local.edge_user_data[each.key]
  tags      = ["trqsh", "edge", "region:${each.value.region}"]
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

# Cloud firewall: allow public ingress + agent ports; SSH from anywhere (tighten
# to a bastion in prod).
resource "digitalocean_firewall" "edge" {
  name        = "${var.project_name}-edge"
  droplet_ids = [for d in digitalocean_droplet.edge : d.id]

  dynamic "inbound_rule" {
    for_each = [
      { proto = "tcp", ports = "80" },
      { proto = "tcp", ports = "443" },
      { proto = "tcp", ports = "4443" },
      { proto = "udp", ports = "4443" },
      { proto = "tcp", ports = "22" },
    ]
    content {
      protocol         = inbound_rule.value.proto
      port_range       = inbound_rule.value.ports
      source_addresses = ["0.0.0.0/0", "::/0"]
    }
  }

  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}

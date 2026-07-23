# Managed Postgres (control-plane state) and Redis (routing registry + pub/sub).
resource "digitalocean_database_cluster" "postgres" {
  name       = "${var.project_name}-pg"
  engine     = "pg"
  version    = "16"
  size       = var.postgres_size
  region     = var.primary_region
  node_count = 1
  private_network_uuid = digitalocean_vpc.control.id
}

resource "digitalocean_database_db" "trqsh" {
  cluster_id = digitalocean_database_cluster.postgres.id
  name       = "trqsh"
}

resource "digitalocean_database_cluster" "redis" {
  name       = "${var.project_name}-redis"
  engine     = "redis"
  version    = "7"
  size       = var.redis_size
  region     = var.primary_region
  node_count = 1
  private_network_uuid = digitalocean_vpc.control.id
}

# Lock the databases to the VPC (control plane) + edge droplets by IP.
resource "digitalocean_database_firewall" "postgres" {
  cluster_id = digitalocean_database_cluster.postgres.id
  rule {
    type  = "k8s"
    value = digitalocean_kubernetes_cluster.control.id
  }
}

resource "digitalocean_database_firewall" "redis" {
  cluster_id = digitalocean_database_cluster.redis.id
  rule {
    type  = "k8s"
    value = digitalocean_kubernetes_cluster.control.id
  }
  dynamic "rule" {
    for_each = digitalocean_droplet.edge
    content {
      type  = "droplet"
      value = rule.value.id
    }
  }
}

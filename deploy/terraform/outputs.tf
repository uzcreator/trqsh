output "cluster_name" {
  value = digitalocean_kubernetes_cluster.control.name
}

output "cluster_endpoint" {
  value     = digitalocean_kubernetes_cluster.control.endpoint
  sensitive = true
}

output "kubeconfig" {
  description = "Raw kubeconfig for the control cluster."
  value       = digitalocean_kubernetes_cluster.control.kube_config[0].raw_config
  sensitive   = true
}

output "postgres_url" {
  description = "Set as the API's TRQSH_DATABASE_URL (in the k8s Secret)."
  value       = digitalocean_database_cluster.postgres.private_uri
  sensitive   = true
}

output "redis_url" {
  description = "Set as TRQSH_REDIS_URL for the API and edges."
  value       = digitalocean_database_cluster.redis.uri
  sensitive   = true
}

output "edge_ips" {
  description = "Reserved IP per edge node (feed to GeoDNS in prod)."
  value       = { for k, ip in digitalocean_reserved_ip.edge : k => ip.ip_address }
}

output "certs_bucket" {
  value = digitalocean_spaces_bucket.certs.name
}

output "releases_bucket" {
  value = digitalocean_spaces_bucket.releases.name
}

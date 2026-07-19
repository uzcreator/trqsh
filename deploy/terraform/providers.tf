provider "digitalocean" {
  token             = var.do_token
  spaces_access_id  = var.spaces_access_id
  spaces_secret_key = var.spaces_secret_key
}

provider "random" {}

# Group all resources under one DO project for tidy billing/console.
resource "digitalocean_project" "rift" {
  name        = var.project_name
  description = "Rift tunneling platform"
  purpose     = "Web Application"
  environment = "Production"
  resources = concat(
    [digitalocean_kubernetes_cluster.control.urn],
    [digitalocean_database_cluster.postgres.urn],
    [digitalocean_database_cluster.redis.urn],
    [digitalocean_domain.rift.urn],
    [for ip in digitalocean_reserved_ip.edge : ip.urn],
  )
}

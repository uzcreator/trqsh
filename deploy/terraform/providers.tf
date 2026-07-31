provider "digitalocean" {
  token             = var.do_token
  spaces_access_id  = var.spaces_access_id
  spaces_secret_key = var.spaces_secret_key
}

provider "random" {}

# Cloudflare is only used when enable_cloudflare_lb = true (dns_cloudflare.tf).
# The token defaults to "" so the provider stays inert (no resources reference it)
# in the default DO-only topology; `terraform validate`/`plan` do not contact
# Cloudflare unless a cloudflare_* resource is instantiated.
provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

# Group all resources under one DO project for tidy billing/console.
resource "digitalocean_project" "trqsh" {
  name        = var.project_name
  description = "trqsh tunneling platform"
  purpose     = "Web Application"
  environment = "Production"
  resources = concat(
    [digitalocean_kubernetes_cluster.control.urn],
    [digitalocean_database_cluster.postgres.urn],
    [digitalocean_database_cluster.redis.urn],
    [digitalocean_domain.trqsh.urn],
    [for ip in digitalocean_reserved_ip.edge : ip.urn],
  )
}

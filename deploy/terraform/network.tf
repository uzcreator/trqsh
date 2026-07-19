# One VPC per region for the control plane + edge pools.
resource "digitalocean_vpc" "control" {
  name   = "${var.project_name}-${var.primary_region}"
  region = var.primary_region
}

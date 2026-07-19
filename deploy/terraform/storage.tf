# Object storage (S3-compatible Spaces): CertMagic cert cache shared across edges
# and release artifacts / the GUI+CLI auto-update feed.
resource "digitalocean_spaces_bucket" "certs" {
  name   = "${var.project_name}-certs"
  region = var.primary_region
  acl    = "private"
}

resource "digitalocean_spaces_bucket" "releases" {
  name   = "${var.project_name}-releases"
  region = var.primary_region
  acl    = "public-read" # installers + update feed are public downloads
}

# Object storage (S3-compatible Spaces): CertMagic cert cache shared across edges
# and release artifacts / the GUI+CLI auto-update feed.
resource "digitalocean_spaces_bucket" "certs" {
  name   = "${var.project_name}-certs"
  region = var.primary_region
  acl    = "private"

  # Recovery path if a bug ever overwrites the shared cert cache with garbage.
  versioning {
    enabled = true
  }
}

# trivy:ignore:AVD-DIG-0006
# trivy:ignore:DIG-0006
# Public-read is intentional: this bucket serves installers + the desktop
# auto-update feed to anonymous clients, same as any public release CDN.
# Nothing sensitive is ever written here — secrets/certs live in the
# separate "certs" bucket above, which stays private.
resource "digitalocean_spaces_bucket" "releases" {
  name   = "${var.project_name}-releases"
  region = var.primary_region
  acl    = "public-read"

  # Recovery path if a release job ever overwrites an artifact under the same key.
  versioning {
    enabled = true
  }
}

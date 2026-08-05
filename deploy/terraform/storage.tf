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

# Bucket ACL is private — no public LIST/READ of the bucket as a whole (which
# would also expose internal naming/versioning of unreleased builds). Nothing
# currently uploads here yet; when the release job is wired up, it must set
# `--acl public-read` (or the SDK/doctl equivalent) on each object it PUTs,
# which is the standard, correct way to serve public installers/the desktop
# auto-update feed without a public bucket-level grant. Object-level ACLs are
# independent of the bucket ACL, so this doesn't require anything else here.
resource "digitalocean_spaces_bucket" "releases" {
  name   = "${var.project_name}-releases"
  region = var.primary_region
  acl    = "private"

  # Recovery path if a release job ever overwrites an artifact under the same key.
  versioning {
    enabled = true
  }
}

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.43"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    # Optional: only exercised when enable_cloudflare_lb = true (dns_cloudflare.tf).
    # Pinned to the v4 line — v5 reworked the load-balancer schema into nested
    # attributes, so upgrading requires rewriting dns_cloudflare.tf.
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
  }

  # Remote state (DigitalOcean Spaces, S3-compatible). Configure via
  # `terraform init -backend-config=...` or a backend.hcl. Commented so `plan`
  # works locally before a bucket exists.
  # backend "s3" {
  #   endpoints                   = { s3 = "https://fra1.digitaloceanspaces.com" }
  #   bucket                      = "trqsh-tfstate"
  #   key                         = "infra/terraform.tfstate"
  #   region                      = "us-east-1"
  #   skip_credentials_validation = true
  #   skip_metadata_api_check     = true
  #   skip_region_validation      = true
  #   use_path_style              = true
  # }
}

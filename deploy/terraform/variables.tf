variable "do_token" {
  description = "DigitalOcean API token (env: TF_VAR_do_token)."
  type        = string
  sensitive   = true
}

variable "spaces_access_id" {
  description = "Spaces access key for object storage (cert cache + releases)."
  type        = string
  sensitive   = true
  default     = ""
}

variable "spaces_secret_key" {
  description = "Spaces secret key."
  type        = string
  sensitive   = true
  default     = ""
}

variable "project_name" {
  description = "DigitalOcean project name."
  type        = string
  default     = "trqsh"
}

variable "internal_token" {
  description = "Shared edge<->API token (must match the API's TRQSH_INTERNAL_TOKEN)."
  type        = string
  sensitive   = true
}

variable "acme_email" {
  description = "Contact email for Let's Encrypt (edge cert issuance)."
  type        = string
  default     = "ops@trqsh.uz"
}

variable "control_ingress_ip" {
  description = "Public IP of the control-cluster ingress LB (for api/app records). Read it after `helm install` provisions the nginx LB, or manage these records with external-dns and leave blank."
  type        = string
  default     = ""
}

variable "edge_image" {
  description = "Edge container image the droplets run."
  type        = string
  default     = "ghcr.io/trqsh/edge:latest"
}

variable "ssh_key_fingerprints" {
  description = "SSH key fingerprints to place on edge droplets."
  type        = list(string)
  default     = []
}

variable "domain" {
  description = "Public apex domain managed here (wildcard tunnels live under it)."
  type        = string
  default     = "trqsh.uz"
}

variable "primary_region" {
  description = "Region for the control plane (K8s, Postgres, Redis)."
  type        = string
  default     = "fra1"
}

variable "edge_regions" {
  description = "Regions to place edge node pools + reserved IPs (latency spread)."
  type        = list(string)
  default     = ["fra1", "nyc3", "sgp1"]
}

variable "k8s_version" {
  description = "DOKS Kubernetes version prefix."
  type        = string
  default     = "1.31."
}

variable "control_node_size" {
  description = "Droplet size for the control-plane node pool (api/dashboard)."
  type        = string
  default     = "s-2vcpu-4gb"
}

variable "edge_node_size" {
  description = "Droplet size for edge node pools."
  type        = string
  default     = "s-2vcpu-4gb"
}

variable "edge_nodes_per_region" {
  description = "Edge nodes per region."
  type        = number
  default     = 2
}

variable "postgres_size" {
  description = "Managed Postgres node size."
  type        = string
  default     = "db-s-1vcpu-2gb"
}

variable "redis_size" {
  description = "Managed Redis node size."
  type        = string
  default     = "db-s-1vcpu-1gb"
}

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
  default     = "ghcr.io/uzcreator/edge:latest"
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

# --- Inter-edge forwarding (Stage D) ---------------------------------------

variable "edge_forward_port" {
  description = "Internal port edges use for the cross-edge forwarding hop (Stage D). Token-authenticated and firewalled to edge droplets only — never opened to the public internet."
  type        = number
  default     = 4444
}

variable "edge_forward_iface" {
  description = "Which droplet interface edges advertise for the forwarding hop. \"public\" (default) is the only choice that works ACROSS regions, because DO VPCs are single-region; \"private\" gives a free, VPC-internal hop but only between edges in the same region/VPC."
  type        = string
  default     = "public"
  validation {
    condition     = contains(["public", "private"], var.edge_forward_iface)
    error_message = "edge_forward_iface must be \"public\" or \"private\"."
  }
}

# --- Cloudflare Load Balancing (optional, DNS-only steering) ----------------
# All inert unless enable_cloudflare_lb = true. See dns_cloudflare.tf.

variable "enable_cloudflare_lb" {
  description = "Steer the wildcard tunnel record with Cloudflare Load Balancing (health-checked failover/latency) instead of the DigitalOcean round-robin. Requires the zone hosted at Cloudflare and a plan that includes Load Balancing. Defaults off so the module stays DO-only until deliberately turned on."
  type        = bool
  default     = false
}

variable "cloudflare_api_token" {
  description = "Cloudflare API token with Load Balancing + DNS edit on the zone. Only used when enable_cloudflare_lb = true."
  type        = string
  sensitive   = true
  default     = ""
}

variable "cloudflare_account_id" {
  description = "Cloudflare account ID owning the LB monitor + pool (enable_cloudflare_lb = true)."
  type        = string
  default     = ""
}

variable "cloudflare_zone_id" {
  description = "Cloudflare zone ID for var.domain (enable_cloudflare_lb = true)."
  type        = string
  default     = ""
}

variable "cloudflare_lb_steering_policy" {
  description = "Cloudflare LB steering policy: \"dynamic_latency\" (fastest healthy edge per resolver), \"off\" (strict failover order), \"geo\", \"proximity\", or \"random\"."
  type        = string
  default     = "dynamic_latency"
}

variable "cloudflare_lb_monitor_type" {
  description = "Edge health-check type. \"tcp\" (default) probes :443 liveness and needs NO new firewall exposure; \"http\"/\"https\" probe a path (see cloudflare_lb_monitor_port/path) — to hit the edge /healthz you must additionally open that port to Cloudflare's health-check IP ranges."
  type        = string
  default     = "tcp"
  validation {
    condition     = contains(["tcp", "http", "https"], var.cloudflare_lb_monitor_type)
    error_message = "cloudflare_lb_monitor_type must be \"tcp\", \"http\", or \"https\"."
  }
}

variable "cloudflare_lb_monitor_port" {
  description = "Port the Cloudflare monitor probes on each edge. 443 for the default TCP liveness check; 9090 (the edge metrics/health port) if you switch to an HTTP /healthz check and open that port to Cloudflare."
  type        = number
  default     = 443
}

variable "cloudflare_lb_monitor_path" {
  description = "Path for an http/https monitor (ignored for tcp). The edge serves /healthz on its metrics port."
  type        = string
  default     = "/healthz"
}

variable "cloudflare_lb_monitor_expected_codes" {
  description = "Expected HTTP status for an http/https monitor (ignored for tcp)."
  type        = string
  default     = "200"
}

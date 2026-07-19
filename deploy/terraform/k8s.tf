# DOKS cluster for the control plane (api + dashboard, ingress, cert-manager).
# Edges do NOT run here — they are per-region droplets (edge.tf) so public
# traffic hits the nearest region. Deploy the chart with edge.enabled=false here;
# the Helm edge DaemonSet remains available for single-region / self-hosted k8s.
data "digitalocean_kubernetes_versions" "current" {
  version_prefix = var.k8s_version
}

resource "digitalocean_kubernetes_cluster" "control" {
  name     = "${var.project_name}-control"
  region   = var.primary_region
  version  = data.digitalocean_kubernetes_versions.current.latest_version
  vpc_uuid = digitalocean_vpc.control.id

  node_pool {
    name       = "control"
    size       = var.control_node_size
    node_count = 2
    labels = {
      "rift.dev/role" = "control"
    }
  }
}

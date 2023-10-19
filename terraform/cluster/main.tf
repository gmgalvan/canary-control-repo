provider "google" {
  credentials = file("./credentials.json")
  project     = var.gcp_project
  region      = var.gcp_region
}

resource "google_container_cluster" "gke_cluster" {
  name     = "demo"
  location = "us-central1-a"
  initial_node_count = 1

  master_auth {
    client_certificate_config {
      issue_client_certificate = false
    }
  }

  node_config {
    oauth_scopes = [
      "https://www.googleapis.com/auth/logging.write",
      "https://www.googleapis.com/auth/monitoring",
    ]
  }
}

resource "helm_release" "istio" {
  name       = "istio"
  repository = "https://istio-release.storage.googleapis.com/charts"
  chart      = "istio"
  version    = "1.19.3"
  namespace  = "istio-system"

  set {
    name  = "global.controlPlaneSecurityEnabled"
    value = "true"
  }

  depends_on = [google_container_cluster.gke_cluster]
}


output "cluster_endpoint" {
  value = google_container_cluster.gke_cluster.endpoint
}


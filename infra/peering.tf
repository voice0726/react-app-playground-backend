# shared-infra 側も逆向きの peering を作ったときに ACTIVE になります
resource "google_compute_network_peering" "shared_db" {
  count = var.shared_db_network == null ? 0 : 1

  name         = "react-app-playground-backend-to-shared-db"
  network      = google_compute_network.app.self_link
  peer_network = "https://www.googleapis.com/compute/v1/${var.shared_db_network}"
}

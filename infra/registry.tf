resource "google_artifact_registry_repository" "api" {
  repository_id = "react-app-playground-backend"
  location      = var.artifact_region
  format        = "DOCKER"

  depends_on = [google_project_service.this]
}

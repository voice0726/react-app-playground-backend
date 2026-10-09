output "db_host" {
  value = google_compute_address.db.address
}

output "image_repository" {
  value = "${var.artifact_region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.api.repository_id}"
}

output "workload_identity_provider" {
  value = google_iam_workload_identity_pool_provider.github.name
}

output "deploy_service_account" {
  value = google_service_account.deploy.email
}

output "api_url" {
  value = var.deploy_workloads ? google_cloud_run_v2_service.api[0].uri : null
}

output "migrate_job" {
  value = var.deploy_workloads ? google_cloud_run_v2_job.migrate[0].name : null
}

output "database_url_secret" {
  value = google_secret_manager_secret.database_url.secret_id
}

output "gateway_token_secret" {
  value = google_secret_manager_secret.gateway_token.secret_id
}

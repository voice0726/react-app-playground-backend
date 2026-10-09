resource "google_service_account" "workload" {
  for_each = toset(["api", "migrate"])

  account_id   = "playground-${each.key}"
  display_name = "react-app-playground-backend ${each.key}"

  depends_on = [google_project_service.this]
}

resource "google_secret_manager_secret" "database_url" {
  secret_id = "playground-database-url"

  replication {
    auto {}
  }

  depends_on = [google_project_service.this]
}

resource "google_secret_manager_secret_version" "database_url" {
  secret      = google_secret_manager_secret.database_url.id
  secret_data = "postgres://playground:${random_password.db.result}@${google_compute_address.db.address}:5432/playground?sslmode=disable"
}

resource "google_secret_manager_secret_iam_member" "database_url" {
  for_each = google_service_account.workload

  secret_id = google_secret_manager_secret.database_url.id
  role      = "roles/secretmanager.secretAccessor"
  member    = each.value.member
}

resource "google_secret_manager_secret" "gateway_token" {
  secret_id = "playground-auth-gateway-internal-token"

  replication {
    auto {}
  }

  depends_on = [google_project_service.this]
}

resource "google_secret_manager_secret_iam_member" "gateway_token" {
  secret_id = google_secret_manager_secret.gateway_token.id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.workload["api"].member
}

resource "google_service_account" "deploy" {
  account_id   = "playground-deploy"
  display_name = "react-app-playground-backend GitHub Actions deploy"

  depends_on = [google_project_service.this]
}

resource "google_project_iam_member" "deploy_run" {
  project = var.project_id
  role    = "roles/run.developer"
  member  = google_service_account.deploy.member
}

resource "google_artifact_registry_repository_iam_member" "deploy_push" {
  repository = google_artifact_registry_repository.api.name
  location   = var.artifact_region
  role       = "roles/artifactregistry.writer"
  member     = google_service_account.deploy.member
}

resource "google_service_account_iam_member" "deploy_act_as" {
  for_each = google_service_account.workload

  service_account_id = each.value.name
  role               = "roles/iam.serviceAccountUser"
  member             = google_service_account.deploy.member
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github"

  depends_on = [google_project_service.this]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }
  attribute_condition = join(" && ", [
    "assertion.repository == \"${var.github_repository}\"",
    "assertion.repository_id == \"1410098688\"",
    "assertion.repository_owner_id == \"42563149\"",
    "assertion.ref == \"refs/heads/main\"",
    "assertion.workflow_ref == \"${var.github_repository}/.github/workflows/deploy.yml@refs/heads/main\"",
  ])

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "deploy_wif" {
  service_account_id = google_service_account.deploy.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repository}"
}

locals {
  api_env = {
    SERVER_ADDR          = ":8080"
    DB_CONNECT_TIMEOUT   = "180s"
    AUTH_GATEWAY_URL     = var.auth_gateway_url
    PUBLIC_ORIGIN        = var.public_origin
    AUTH_GATEWAY_TIMEOUT = "2s"
    SHUTDOWN_TIMEOUT     = "5s"
    LOG_LEVEL            = "info"
    LOG_FORMAT           = "json"
  }
}

resource "google_cloud_run_v2_service" "api" {
  count = var.deploy_workloads ? 1 : 0

  name                 = "react-app-playground-backend"
  location             = var.region
  ingress              = "INGRESS_TRAFFIC_ALL"
  deletion_protection  = true
  invoker_iam_disabled = true

  template {
    service_account = google_service_account.workload["api"].email

    scaling {
      min_instance_count = 0
      max_instance_count = var.max_instances
    }

    containers {
      image = var.image

      ports {
        container_port = 8080
      }

      dynamic "env" {
        for_each = local.api_env
        content {
          name  = env.key
          value = env.value
        }
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url.secret_id
            version = google_secret_manager_secret_version.database_url.version
          }
        }
      }

      env {
        name = "AUTH_GATEWAY_INTERNAL_TOKEN"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.gateway_token.secret_id
            version = var.auth_gateway_token_version
          }
        }
      }

      startup_probe {
        initial_delay_seconds = 0
        timeout_seconds       = 2
        period_seconds        = 5
        failure_threshold     = 48
        http_get {
          path = "/readyz"
          port = 8080
        }
      }

      liveness_probe {
        http_get {
          path = "/healthz"
          port = 8080
        }
      }
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.app.id
        subnetwork = google_compute_subnetwork.cloud_run.id
      }
    }
  }

  lifecycle {
    ignore_changes = [template[0].containers[0].image, client, client_version]
  }

  depends_on = [
    google_compute_instance.db,
    google_secret_manager_secret_iam_member.database_url,
    google_secret_manager_secret_iam_member.gateway_token,
  ]
}

resource "google_cloud_run_v2_job" "migrate" {
  count = var.deploy_workloads ? 1 : 0

  name                = "react-app-playground-backend-migrate"
  location            = var.region
  deletion_protection = true

  template {
    task_count  = 1
    parallelism = 1
    template {
      service_account = google_service_account.workload["migrate"].email
      max_retries     = 0
      timeout         = "600s"

      containers {
        image   = var.image
        command = ["/app/migrate"]
        args    = ["up"]

        env {
          name  = "DB_CONNECT_TIMEOUT"
          value = "180s"
        }

        env {
          name = "DATABASE_URL"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.database_url.secret_id
              version = google_secret_manager_secret_version.database_url.version
            }
          }
        }
      }

      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = google_compute_network.app.id
          subnetwork = google_compute_subnetwork.cloud_run.id
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [template[0].template[0].containers[0].image, client, client_version]
  }

  depends_on = [
    google_compute_instance.db,
    google_secret_manager_secret_iam_member.database_url,
  ]
}

variable "project_id" {
  type    = string
  default = "react-app-playground-backend"
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "artifact_region" {
  type    = string
  default = "us-central1"
}

variable "cloud_run_subnet_cidr" {
  description = "Direct VPC egress 専用。/26 以上の範囲を使います"
  type        = string
  default     = "10.31.0.0/26"
}

variable "github_repository" {
  type    = string
  default = "voice0726/react-app-playground-backend"
}

variable "deploy_workloads" {
  description = "共有 DB との peering と内部トークンの登録後に true にします"
  type        = bool
  default     = false
}

variable "image" {
  description = "初回に API と migrate Job に使う linux/amd64 イメージ。以後は CI が更新します"
  type        = string
  default     = ""

  validation {
    condition     = !var.deploy_workloads || length(var.image) > 0
    error_message = "deploy_workloads=true には image が必要です。"
  }
}

variable "auth_gateway_url" {
  description = "gateway の HTTPS origin。パスや末尾の / は付けません"
  type        = string
  default     = ""

  validation {
    condition     = !var.deploy_workloads || can(regex("^https://[^/?#@]+$", var.auth_gateway_url))
    error_message = "deploy_workloads=true には gateway の HTTPS origin が必要です。"
  }
}

variable "public_origin" {
  description = "Workers 側フロントの HTTPS origin。末尾の / は付けません"
  type        = string
  default     = ""

  validation {
    condition     = !var.deploy_workloads || can(regex("^https://[^/?#@]+$", var.public_origin))
    error_message = "deploy_workloads=true にはフロントの HTTPS origin が必要です。"
  }
}

variable "auth_gateway_token_version" {
  description = "登録済み内部トークンの Secret Manager バージョン番号"
  type        = string
  default     = ""

  validation {
    condition     = !var.deploy_workloads || can(regex("^[1-9][0-9]*$", var.auth_gateway_token_version))
    error_message = "deploy_workloads=true には内部トークンのバージョン番号が必要です。"
  }
}

variable "max_instances" {
  type    = number
  default = 2

  validation {
    condition     = var.max_instances >= 1 && floor(var.max_instances) == var.max_instances
    error_message = "max_instances は 1 以上の整数にしてください。"
  }
}

variable "shared_db_network" {
  description = "shared-infra の DB 用 VPC（projects/<id>/global/networks/<name>）。null なら peering を作りません"
  type        = string
  default     = null

  validation {
    condition     = var.shared_db_network == null || can(regex("^projects/[a-z][a-z0-9-]+/global/networks/[a-z][a-z0-9-]*$", var.shared_db_network))
    error_message = "shared_db_network は projects/<id>/global/networks/<name> の形式にしてください。"
  }
}

variable "shared_database_url_secret" {
  description = "shared-infra の DATABASE_URL の Secret（projects/<番号>/secrets/<ID>）"
  type        = string
  default     = ""

  validation {
    condition     = !var.deploy_workloads || can(regex("^projects/[0-9]+/secrets/[A-Za-z0-9_-]+$", var.shared_database_url_secret))
    error_message = "deploy_workloads=true には projects/<番号>/secrets/<ID> の形式の shared_database_url_secret が必要です。"
  }
}

variable "shared_database_url_secret_version" {
  description = "shared_database_url_secret のバージョン番号"
  type        = string
  default     = ""

  validation {
    condition     = !var.deploy_workloads || can(regex("^[1-9][0-9]*$", var.shared_database_url_secret_version))
    error_message = "deploy_workloads=true には shared_database_url_secret のバージョン番号が必要です。"
  }
}

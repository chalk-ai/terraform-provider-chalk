# main.tf
terraform {
  required_providers {
    chalk = {
      source  = "registry.terraform.io/chalk-ai/chalk"
      version = "0.1.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.5.1"
    }
    google = {
      source  = "hashicorp/google"
      version = "5.0.0"
    }
  }
}
data "google_client_openid_userinfo" "me" {}

locals {
  sanitized_email = replace(data.google_client_openid_userinfo.me.email, "/[^a-zA-Z0-9-]/", "-")
}

# Fixed token
provider "chalk" {
  client_id     = "token-environment-fixed"
  client_secret = "ts-d2c87cbb1dd742c666d547d393a5341e011683206891fcc6dc2780ffd5cdf67e"
  api_server    = "http://localhost:8080"
}

resource "chalk_project" "test" {
  name = "project-${random_pet.pet.id}"
}

resource "random_pet" "pet" {}

resource "chalk_cloud_credentials" "creds" {
  kind                    = "aws"
  name                    = "creds-${random_pet.pet.id}"
  aws_account_id          = "009160067517"
  aws_management_role_arn = "arn:aws:iam::009160067517:role/chalk-cicd-test-Chalk-Api-Management"
  aws_region              = "us-east-1"

  gcp_workload_identity {
    pool_id         = "cicd-009160067517-pool"
    provider_id     = "cicd-009160067517-provider"
    service_account = "aws-workload-009160067517@chalk-infra.iam.gserviceaccount.com"
    project_number  = "610611181724"
  }

  docker_build_config {
    builder            = "argo"
    notification_topic = "arn:aws:sqs:us-east-1:009160067517:argo-build-queue-${local.sanitized_email}"
  }
}

resource "chalk_kubernetes_cluster" "cluster" {
  kind                = "EKS_STANDARD"
  kubernetes_version  = "1.32"
  managed             = false
  name                = "chalk-cicd-test-eks-cluster"
  cloud_credential_id = chalk_cloud_credentials.creds.id
}

resource "chalk_environment" "test" {
  name                      = "env-${random_pet.pet.id}"
  project_id                = chalk_project.test.id
  kube_cluster_id           = chalk_kubernetes_cluster.cluster.id
  kube_job_namespace        = "ns-${local.sanitized_email}"
  kube_service_account_name = "env-${local.sanitized_email}-workload-identity"
  service_url               = "https://${local.sanitized_email}.cicd.chalk.dev/"
  worker_url                = "https://${local.sanitized_email}.cicd.chalk.dev/"
  source_bundle_bucket      = "s3://chalk-cicd-test-source-bucket"
  additional_env_vars = {
    "CHALK_INITIALIZE_NATIVE_BUS_PUBLISHER" : "1", "CHALK_PERSIST_TO_OFFLINE_STORE_QUERY_LOG" : "1", "CHALK_PLANNER_ENABLE_NATIVE_RESULT_BUS_PERSISTENCE" : "1", "CHALK_PLANNER_PERSIST_VALUES_OFFLINE_STORE" : "0", "CHALK_PLANNER_PERSIST_VALUES_PARQUET" : "0", "CHALK_PLANNER_SKIP_RELATIONSHIP_DISTINCT" : "1", "CHALK_PLANNER_USE_FILTERED_JOINS" : "0", "CHALK_PLANNER_USE_NATIVE_SQL_OPERATORS" : "1", "CHALK_PLANNER_USE_NATIVE_STATISTICS_OPERATOR" : "0", "CHALK_PLANNER_VELOX_USE_ZERO_COPY_HASH_JOIN" : "1", "CHALK_SKIP_USAGE_PERSISTENCE" : "1", "CHALK_STATIC_UNDERSCORE_EXPRESSIONS" : "1", "CHECK_DUPLICATE_ROWS" : "0", "DD_TRACE_ENABLED" : "1", "GRPC_QUERY_SERVER_NO_TLS" : "1", "PYTHONOPTIMIZE" : "1"
  }
}

output "env_info" {
  value = chalk_environment.test.name
}

output "project_info" {
  value = chalk_project.test.name
}

output "cluster_info" {
  value = chalk_kubernetes_cluster.cluster.name
}

resource "chalk_cluster_timescale" "timescale" {
  environment_ids = [chalk_environment.test.id]
  timescale_image                 = "ghcr.io/imusmanmalik/timescaledb-postgis:16-3.4-54"
  database_name                   = "${local.sanitized_email}-chalk-metrics"
  database_replicas               = 1
  storage                         = "30Gi"
  namespace                       = "ns-${local.sanitized_email}"
  connection_pool_replicas        = 1
  connection_pool_max_connections = "500"
  connection_pool_size            = "50"
  connection_pool_mode            = "transaction"
  backup_bucket                   = "s3://chalk-cicd-test-timescale-backups"
  backup_iam_role_arn             = "arn:aws:iam::009160067517:role/chalk-timescale-backup-role"
  include_chalk_node_selector     = true
  instance_type                   = "c5.large"
  request = {
    cpu    = "500m"
    memory = "1Gi"
  }
  service_type = "load-balancer"

  postgres_parameters = {
    max_connections = "200"
  }
}

resource "chalk_cluster_background_persistence" "persistence" {
  environment_ids = [chalk_environment.test.id]
  namespace                                = "ns-${local.sanitized_email}"
  service_account_name                     = "${local.sanitized_email}-persistence-workload-identity"
  bus_backend                              = "KAFKA"
  secret_client                            = "AWS"
  bigquery_parquet_upload_subscription_id  = "${local.sanitized_email}-offline-store-bulk-insert-bus-1"
  bigquery_streaming_write_subscription_id = "${local.sanitized_email}-offline-store-streaming-insert-bus-1"
  bigquery_streaming_write_topic           = "${local.sanitized_email}-offline-store-streaming-insert-bus-1"
  bq_upload_bucket                         = "s3://chalk-cicd-test-data-bucket"
  bq_upload_topic                          = "${local.sanitized_email}-offline-store-bulk-insert-bus-1"
  kafka_dlq_topic                          = "${local.sanitized_email}-dlq-1"
  metrics_bus_subscription_id              = "${local.sanitized_email}-metrics-bus-1"
  metrics_bus_topic_id                     = "${local.sanitized_email}-metrics-bus-1"
  operation_subscription_id                = "${local.sanitized_email}-operation-bus-1"
  query_log_result_topic                   = "${local.sanitized_email}-query-log"
  query_log_subscription_id                = "${local.sanitized_email}-query-log"
  result_bus_metrics_subscription_id       = "${local.sanitized_email}-result-bus-1"
  result_bus_offline_store_subscription_id = "${local.sanitized_email}-result-bus-1"
  result_bus_online_store_subscription_id  = "${local.sanitized_email}-result-bus-1"
  result_bus_topic_id                      = "${local.sanitized_email}-result-bus-1"
  usage_bus_topic_id                       = "${local.sanitized_email}-usage-bus"
  usage_events_subscription_id             = "${local.sanitized_email}-usage-events"
  api_server_host                          = "http://${local.sanitized_email}-api-proxy-service.ns-${local.sanitized_email}.svc.cluster.local"
  kafka_sasl_secret                        = "AmazonMSK_chalk-cicd-test_chalk"
  metadata_provider                        = "grpc_server"
  kafka_bootstrap_servers                  = "b-2.chalkcicdtestkafkaclus.446fhd.c4.kafka.us-east-1.amazonaws.com:9096,b-1.chalkcicdtestkafkaclus.446fhd.c4.kafka.us-east-1.amazonaws.com:9096,b-3.chalkcicdtestkafkaclus.446fhd.c4.kafka.us-east-1.amazonaws.com:9096"
  kafka_security_protocol                  = "SASL_SSL"
  kafka_sasl_mechanism                     = "SCRAM-SHA-512"
  redis_is_clustered                       = "1"
  redis_lightning_supports_has_many        = false
  insecure                                 = true

  writers = [
    {
      name                  = "go-metrics-bus-writer"
      bus_subscriber_type   = "GO_METRICS_BUS_WRITER"
      default_replica_count = 1
      request = {
        cpu    = "500m"
        memory = "1Gi"
      }

    }, {
      name                  = "cluster-manager"
      bus_subscriber_type   = "CLUSTER_MANAGER"
      default_replica_count = 1
      request = {
        cpu    = "500m"
        memory = "1Gi"
      }
    }
  ]
}

output "creds_info" {
  value = chalk_cloud_credentials.creds.name
}

output "timescale_info" {
  value = {
    id         = chalk_cluster_timescale.timescale.id
    created_at = chalk_cluster_timescale.timescale.created_at
  }
}

output "persistence_info" {
  value = {
    id         = chalk_cluster_background_persistence.persistence.id
    created_at = chalk_cluster_background_persistence.persistence.created_at
  }
}
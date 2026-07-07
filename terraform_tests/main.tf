# main.tf
terraform {
  required_providers {
    chalk = {
      source  = "registry.terraform.io/chalk-ai/chalk"
      version = "0.1.0"
    }
  }
}

# Fixed token
provider "chalk" {
  client_id     = "token-environment-fixed"
  client_secret = "ts-d2c87cbb1dd742c666d547d393a5341e011683206891fcc6dc2780ffd5cdf67e"
  api_server    = "http://localhost:8080"
}

resource "chalk_aws_cloud_credentials" "creds" {
  name                    = "creds-remote-dev-ari-chalk-ai"
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
    notification_topic = "arn:aws:sqs:us-east-1:009160067517:argo-build-queue-ari-chalk-ai"
  }
}

# resource "chalk_kubernetes_cluster" "cluster" {
#   kind                = "EKS_STANDARD"
#   kubernetes_version  = "1.32"
#   managed             = false
#   name                = "chalk-cicd-test-eks-cluster"
#   cloud_credential_id = chalk_aws_cloud_credentials.creds.id
# }

resource "chalk_managed_aws_vpc" "vpc" {
  cidr_block          = "10.100.0.0/16"
  cloud_credential_id = chalk_aws_cloud_credentials.creds.id
  subnets = [
    {
      name               = "subnet-1"
      private_cidr_block = "10.100.1.0/8"
      public_cidr_block  = "10.100.2.0/8"
      availability_zone  = "a"
      }, {
      name               = "subnet-2"
      private_cidr_block = "10.100.3.0/8"
      public_cidr_block  = "10.100.4.0/8"
      availability_zone  = "b"
      }, {
      name               = "subnet-3"
      private_cidr_block = "10.100.5.0/8"
      public_cidr_block  = "10.100.6.0/8"
      availability_zone  = "c"
    }
  ]
}

resource "chalk_managed_cluster" "cluster" {
  cloud_credential_id = chalk_aws_cloud_credentials.creds.id
  vpc_id              = "foobar"
}
#
# resource "chalk_cluster_gateway_binding" "cgwb" {
#   cluster_gateway_id = chalk_cluster_gateway.test.id
#   cluster_id         = chalk_kubernetes_cluster.cluster.id
# }
#
# resource "chalk_cluster_background_persistence_deployment_binding" "cbpb" {
#   background_persistence_deployment_id = chalk_unmanaged_cluster_background_persistence.persistence.id
#   cluster_id                           = chalk_kubernetes_cluster.cluster.id
# }
#
# resource "chalk_telemetry_binding" "telemetry_binding" {
#   cluster_id              = chalk_kubernetes_cluster.cluster.id
#   telemetry_deployment_id = chalk_telemetry.test.id
# }
#
# resource "chalk_unmanaged_environment" "test" {
#   id                          = local.sanitized_email
#   name                        = local.sanitized_email
#   project_id                  = chalk_project.test.id
#   kube_cluster_id             = chalk_kubernetes_cluster.cluster.id
#   kube_job_namespace          = "ns-${local.sanitized_email}"
#   kube_service_account_name   = "env-${local.sanitized_email}-workload-identity"
#   service_url                 = "https://${local.sanitized_email}.remote.internal.aws.chalk.dev/"
#   engine_docker_registry_path = "engines/engine-${local.sanitized_email}"
#   additional_env_vars = {
#     "CHALK_INITIALIZE_NATIVE_BUS_PUBLISHER" : "1", "CHALK_PERSIST_TO_OFFLINE_STORE_QUERY_LOG" : "1", "CHALK_PLANNER_ENABLE_NATIVE_RESULT_BUS_PERSISTENCE" : "1", "CHALK_PLANNER_PERSIST_VALUES_OFFLINE_STORE" : "0", "CHALK_PLANNER_PERSIST_VALUES_PARQUET" : "0", "CHALK_PLANNER_SKIP_RELATIONSHIP_DISTINCT" : "1", "CHALK_PLANNER_USE_FILTERED_JOINS" : "0", "CHALK_PLANNER_USE_NATIVE_SQL_OPERATORS" : "1", "CHALK_PLANNER_USE_NATIVE_STATISTICS_OPERATOR" : "0", "CHALK_PLANNER_VELOX_USE_ZERO_COPY_HASH_JOIN" : "1", "CHALK_SKIP_USAGE_PERSISTENCE" : "1", "CHALK_STATIC_UNDERSCORE_EXPRESSIONS" : "1", "CHECK_DUPLICATE_ROWS" : "0", "DD_TRACE_ENABLED" : "1", "GRPC_QUERY_SERVER_NO_TLS" : "1", "PYTHONOPTIMIZE" : "1"
#   }
# }
#
# # Buckets are registered as standalone cloud storage resources, then bound to
# # the environment per role - this replaces the deprecated `environment_buckets`
# # field that used to live directly on the environment resource.
# resource "chalk_unmanaged_cloud_storage" "dataset" {
#   uri                 = "s3://chalk-cicd-test-dataset-bucket"
#   cloud_credential_id = chalk_aws_cloud_credentials.creds.id
# }
#
# resource "chalk_unmanaged_cloud_storage" "plan_stages" {
#   uri                 = "s3://chalk-cicd-test-stages-bucket"
#   cloud_credential_id = chalk_aws_cloud_credentials.creds.id
# }
#
# resource "chalk_unmanaged_cloud_storage" "source_bundle" {
#   uri                 = "s3://chalk-cicd-test-source-bucket"
#   cloud_credential_id = chalk_aws_cloud_credentials.creds.id
# }
#
# resource "chalk_unmanaged_cloud_storage" "model_registry" {
#   uri                 = "s3://chalk-cicd-test-model-registry-bucket"
#   cloud_credential_id = chalk_aws_cloud_credentials.creds.id
# }
#
# resource "chalk_environment_dataset_cloud_storage_binding" "dataset" {
#   environment_id   = chalk_unmanaged_environment.test.id
#   cloud_storage_id = chalk_unmanaged_cloud_storage.dataset.id
# }
#
# resource "chalk_environment_plan_stages_cloud_storage_binding" "plan_stages" {
#   environment_id   = chalk_unmanaged_environment.test.id
#   cloud_storage_id = chalk_unmanaged_cloud_storage.plan_stages.id
# }
#
# resource "chalk_environment_source_bundle_cloud_storage_binding" "source_bundle" {
#   environment_id   = chalk_unmanaged_environment.test.id
#   cloud_storage_id = chalk_unmanaged_cloud_storage.source_bundle.id
# }
#
# resource "chalk_environment_model_registry_cloud_storage_binding" "model_registry" {
#   environment_id   = chalk_unmanaged_environment.test.id
#   cloud_storage_id = chalk_unmanaged_cloud_storage.model_registry.id
# }
#
# resource "chalk_cluster_timescale" "timescale" {
#   environment_id                  = chalk_unmanaged_environment.test.id
#   timescale_image                 = "ghcr.io/imusmanmalik/timescaledb-postgis:16-3.4-54"
#   database_name                   = "${local.sanitized_email}-chalk-metrics"
#   database_replicas               = 1
#   storage                         = "30Gi"
#   namespace                       = "ns-${local.sanitized_email}"
#   connection_pool_replicas        = 1
#   connection_pool_max_connections = "500"
#   connection_pool_size            = "50"
#   instance_type                   = "c5.large"
#   request = {
#     cpu    = "500m"
#     memory = "1Gi"
#   }
#   service_type = "load-balancer"
#
#   postgres_parameters = {
#     max_connections = "200"
#   }
#   dns_hostname = "${local.sanitized_email}.metrics.remote.internal.aws.chalk.dev"
# }
#
# resource "chalk_unmanaged_cluster_background_persistence" "persistence" {
#   kube_cluster_id       = chalk_kubernetes_cluster.cluster.id
#   namespace             = "ns-${local.sanitized_email}"
#   service_account_name  = "env-${local.sanitized_email}-workload-identity"
#   api_server_host       = "http://${local.sanitized_email}-api-proxy-service.ns-${local.sanitized_email}.svc.cluster.local:80"
#
#   offline_store_upload_bucket_name = "s3://chalk-cicd-test-data-bucket"
#
#   kafka = {
#     dlq_topic                                  = "${local.sanitized_email}-dlq-1"
#     metrics_bus_topic_id                       = "${local.sanitized_email}-metrics-bus-1"
#     offline_store_bus_streaming_write_topic_id = "${local.sanitized_email}-offline-store-streaming-insert-bus-1"
#     offline_store_bus_upload_topic_id          = "${local.sanitized_email}-offline-store-bulk-insert-bus-1"
#     result_bus_topic_id                        = "${local.sanitized_email}-result-bus-1"
#     bootstrap_servers                          = "b-2.chalkcicdtestkafkaclus.446fhd.c4.kafka.us-east-1.amazonaws.com:9096,b-1.chalkcicdtestkafkaclus.446fhd.c4.kafka.us-east-1.amazonaws.com:9096,b-3.chalkcicdtestkafkaclus.446fhd.c4.kafka.us-east-1.amazonaws.com:9096"
#     sasl_secret                                = "AmazonMSK_chalk-cicd-test_chalk"
#     security_protocol                          = "SASL_SSL"
#     sasl_mechanism                              = "SCRAM-SHA-512"
#   }
#
#   writers = [
#     {
#       bus_subscriber_type   = "GO_METRICS_BUS_WRITER"
#       default_replica_count = 1
#       request = {
#         cpu    = "500m"
#         memory = "1Gi"
#       }
#     }, {
#       bus_subscriber_type   = "CLUSTER_MANAGER"
#       default_replica_count = 1
#       request = {
#         cpu    = "500m"
#         memory = "1Gi"
#       }
#     }
#   ]
# }
#
# resource "chalk_cluster_gateway" "test" {
#   kube_cluster_id    = chalk_kubernetes_cluster.cluster.id
#   namespace          = "chalk-envoy"
#   gateway_name       = "chalk-gw"
#   gateway_class_name = "chalk-gw-class"
#
#   config = {
#     timeout_duration           = "300s"
#     dns_hostname               = "remote.internal.aws.chalk.dev"
#     letsencrypt_cluster_issuer = "chalk-letsencrypt-issuer"
#   }
#
#   service_annotations = {
#     "service.beta.kubernetes.io/aws-load-balancer-scheme"     = "internet-facing"
#     "service.beta.kubernetes.io/aws-load-balancer-type"       = "nlb"
#     "service.beta.kubernetes.io/aws-load-balancer-attributes" = "load_balancing.cross_zone.enabled=true"
#   }
# }
#
# resource "chalk_telemetry" "test" {
#   kube_cluster_id = chalk_kubernetes_cluster.cluster.id
#
#   depends_on = [chalk_cluster_gateway.test]
# }
#
# # FOR CROSS CLUSTER RESOURCES
# # resource "chalk_aws_cloud_credentials" "creds2" {
# #   name                    = "creds-staging-${local.sanitized_email}"
# #   aws_account_id          = "742213191973"
# #   aws_management_role_arn = "arn:aws:iam::742213191973:role/chalk-stag-stage-scoped-api-management"
# #   aws_region              = "us-east-1"
# #
# #   gcp_workload_identity {
# #     pool_id         = "stag-742213191973-pool"
# #     provider_id     = "stag-742213191973-provider"
# #     service_account = "aws-workload-742213191973@chalk-infra.iam.gserviceaccount.com"
# #     project_number  = "610611181724"
# #   }
# # }
# #
# # resource "chalk_kubernetes_cluster" "cluster2" {
# #   kind                = "EKS_STANDARD"
# #   kubernetes_version  = "1.32"
# #   managed             = false
# #   name                = "chalk-stag-stage-eks-cluster"
# #   cloud_credential_id = chalk_aws_cloud_credentials.creds2.id
# # }
# #
# # output "stag_id" {
# #   value = chalk_kubernetes_cluster.cluster2.id
# # }
#

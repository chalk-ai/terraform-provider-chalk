package provider

import (
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// bgpUnmanagedWritersDataSourceSchemaAttribute mirrors bgpUnmanagedWritersSchemaAttribute
// but uses the datasource schema package (all attributes Computed).
func bgpUnmanagedWritersDataSourceSchemaAttribute() dsschema.ListNestedAttribute {
	kubeResource := map[string]dsschema.Attribute{
		"cpu":               dsschema.StringAttribute{Computed: true},
		"memory":            dsschema.StringAttribute{Computed: true},
		"ephemeral_storage": dsschema.StringAttribute{Computed: true},
		"storage":           dsschema.StringAttribute{Computed: true},
	}

	return dsschema.ListNestedAttribute{
		MarkdownDescription: "Background persistence writers.",
		Computed:            true,
		NestedObject: dsschema.NestedAttributeObject{
			Attributes: map[string]dsschema.Attribute{
				"name":           dsschema.StringAttribute{Computed: true},
				"image_override": dsschema.StringAttribute{Computed: true},
				"hpa_specs": dsschema.SingleNestedAttribute{
					Computed: true,
					Attributes: map[string]dsschema.Attribute{
						"hpa_pubsub_subscription_id": dsschema.StringAttribute{Computed: true},
						"hpa_min_replicas":           dsschema.Int64Attribute{Computed: true},
						"hpa_max_replicas":           dsschema.Int64Attribute{Computed: true},
						"hpa_target_average_value":   dsschema.Int64Attribute{Computed: true},
					},
				},
				"gke_spot":                                      dsschema.BoolAttribute{Computed: true},
				"load_writer_configmap":                         dsschema.BoolAttribute{Computed: true},
				"version":                                       dsschema.StringAttribute{Computed: true},
				"request":                                       dsschema.SingleNestedAttribute{Computed: true, Attributes: kubeResource},
				"limit":                                         dsschema.SingleNestedAttribute{Computed: true, Attributes: kubeResource},
				"bus_subscriber_type":                           dsschema.StringAttribute{Computed: true},
				"default_replica_count":                         dsschema.Int64Attribute{Computed: true},
				"kafka_consumer_group_override":                 dsschema.StringAttribute{Computed: true},
				"max_batch_size":                                dsschema.Int64Attribute{Computed: true},
				"message_processing_concurrency":                dsschema.Int64Attribute{Computed: true},
				"metadata_sql_ssl_ca_cert_secret":               dsschema.StringAttribute{Computed: true},
				"metadata_sql_ssl_client_cert_secret":           dsschema.StringAttribute{Computed: true},
				"metadata_sql_ssl_client_key_secret":            dsschema.StringAttribute{Computed: true},
				"metadata_sql_uri_secret":                       dsschema.StringAttribute{Computed: true},
				"offline_store_inserter_db_type":                dsschema.StringAttribute{Computed: true},
				"storage_cache_prefix":                          dsschema.StringAttribute{Computed: true},
				"results_writer_skip_producing_feature_metrics": dsschema.BoolAttribute{Computed: true},
				"query_table_write_drop_ratio":                  dsschema.StringAttribute{Computed: true},
				"additional_env_vars":                           dsschema.MapAttribute{Computed: true, ElementType: types.StringType},
			},
		},
	}
}

package provider

import (
	"context"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"golang.org/x/exp/maps"
)

var kubeResourceConfigSchemaAttrs = map[string]schema.Attribute{
	"cpu": schema.StringAttribute{
		MarkdownDescription: "CPU resource specification",
		Optional:            true,
	},
	"memory": schema.StringAttribute{
		MarkdownDescription: "Memory resource specification",
		Optional:            true,
	},
	"ephemeral_storage": schema.StringAttribute{
		MarkdownDescription: "Ephemeral storage resource specification",
		Optional:            true,
	},
	"storage": schema.StringAttribute{
		MarkdownDescription: "Storage resource specification",
		Optional:            true,
	},
}

var bgpWritersNestedAttrs = map[string]schema.Attribute{
	"name": schema.StringAttribute{
		MarkdownDescription: "Writer name",
		Required:            true,
	},
	"image_override": schema.StringAttribute{
		MarkdownDescription: "Image override",
		Optional:            true,
	},
	"hpa_specs": schema.SingleNestedAttribute{
		MarkdownDescription: "HPA specifications",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"hpa_pubsub_subscription_id": schema.StringAttribute{
				MarkdownDescription: "HPA pubsub subscription ID",
				Optional:            true,
			},
			"hpa_min_replicas": schema.Int64Attribute{
				MarkdownDescription: "HPA minimum replicas",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"hpa_max_replicas": schema.Int64Attribute{
				MarkdownDescription: "HPA maximum replicas",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(10),
			},
			"hpa_target_average_value": schema.Int64Attribute{
				MarkdownDescription: "HPA target average value",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(5),
			},
		},
	},
	"gke_spot": schema.BoolAttribute{
		MarkdownDescription: "GKE spot instances",
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(false),
	},
	"load_writer_configmap": schema.BoolAttribute{
		MarkdownDescription: "Load writer configmap",
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(false),
	},
	"version": schema.StringAttribute{
		MarkdownDescription: "Writer version",
		Optional:            true,
	},
	"request": schema.SingleNestedAttribute{
		MarkdownDescription: "Resource requests",
		Optional:            true,
		Attributes:          kubeResourceConfigSchemaAttrs,
	},
	"limit": schema.SingleNestedAttribute{
		MarkdownDescription: "Resource limits",
		Optional:            true,
		Attributes:          kubeResourceConfigSchemaAttrs,
	},
	"bus_subscriber_type": schema.StringAttribute{
		MarkdownDescription: "Bus subscriber type",
		Required:            true,
	},
	"default_replica_count": schema.Int64Attribute{
		MarkdownDescription: "Default replica count",
		Optional:            true,
		Computed:            true,
		Default:             int64default.StaticInt64(1),
	},
	"kafka_consumer_group_override": schema.StringAttribute{
		MarkdownDescription: "Kafka consumer group override",
		Optional:            true,
	},
	"max_batch_size": schema.Int64Attribute{
		MarkdownDescription: "Maximum batch size",
		Optional:            true,
	},
	"message_processing_concurrency": schema.Int64Attribute{
		MarkdownDescription: "Message processing concurrency",
		Optional:            true,
	},
	"metadata_sql_ssl_ca_cert_secret": schema.StringAttribute{
		MarkdownDescription: "Metadata SQL SSL CA cert secret",
		Optional:            true,
	},
	"metadata_sql_ssl_client_cert_secret": schema.StringAttribute{
		MarkdownDescription: "Metadata SQL SSL client cert secret",
		Optional:            true,
	},
	"metadata_sql_ssl_client_key_secret": schema.StringAttribute{
		MarkdownDescription: "Metadata SQL SSL client key secret",
		Optional:            true,
	},
	"metadata_sql_uri_secret": schema.StringAttribute{
		MarkdownDescription: "Metadata SQL URI secret",
		Optional:            true,
	},
	"offline_store_inserter_db_type": schema.StringAttribute{
		MarkdownDescription: "Offline store inserter DB type",
		Optional:            true,
	},
	"storage_cache_prefix": schema.StringAttribute{
		MarkdownDescription: "Storage cache prefix",
		Optional:            true,
	},
	"results_writer_skip_producing_feature_metrics": schema.BoolAttribute{
		MarkdownDescription: "Results writer skip producing feature metrics",
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(false),
	},
	"query_table_write_drop_ratio": schema.StringAttribute{
		MarkdownDescription: "Query table write drop ratio",
		Optional:            true,
		Computed:            true,
		Default:             stringdefault.StaticString("0.0"),
	},
	"additional_env_vars": schema.MapAttribute{
		MarkdownDescription: "Additional environment variables to set for the writer",
		Optional:            true,
		ElementType:         types.StringType,
	},
}

var bgpWritersSchemaAttribute = schema.ListNestedAttribute{
	MarkdownDescription: "Background persistence writers",
	Required:            true,
	NestedObject: schema.NestedAttributeObject{
		Attributes: bgpWritersNestedAttrs,
	},
}

var bgpUnmanagedWritersSchemaAttribute = schema.ListNestedAttribute{
	MarkdownDescription: "Background persistence writers",
	Required:            true,
	NestedObject: schema.NestedAttributeObject{
		Attributes: func() map[string]schema.Attribute {
			attrs := maps.Clone(bgpWritersNestedAttrs)
			// overrides name attribute and makes it computed, schema is identical otherwise
			attrs["name"] = schema.StringAttribute{
				MarkdownDescription: "Writer name (derived from bus_subscriber_type)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			}
			return attrs
		}(),
	},
}

var bgpWriterObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"name":                                types.StringType,
		"image_override":                      types.StringType,
		"hpa_specs":                           types.ObjectType{AttrTypes: map[string]attr.Type{"hpa_pubsub_subscription_id": types.StringType, "hpa_min_replicas": types.Int64Type, "hpa_max_replicas": types.Int64Type, "hpa_target_average_value": types.Int64Type}},
		"gke_spot":                            types.BoolType,
		"load_writer_configmap":               types.BoolType,
		"version":                             types.StringType,
		"request":                             types.ObjectType{AttrTypes: map[string]attr.Type{"cpu": types.StringType, "memory": types.StringType, "ephemeral_storage": types.StringType, "storage": types.StringType}},
		"limit":                               types.ObjectType{AttrTypes: map[string]attr.Type{"cpu": types.StringType, "memory": types.StringType, "ephemeral_storage": types.StringType, "storage": types.StringType}},
		"bus_subscriber_type":                 types.StringType,
		"default_replica_count":               types.Int64Type,
		"kafka_consumer_group_override":       types.StringType,
		"max_batch_size":                      types.Int64Type,
		"message_processing_concurrency":      types.Int64Type,
		"metadata_sql_ssl_ca_cert_secret":     types.StringType,
		"metadata_sql_ssl_client_cert_secret": types.StringType,
		"metadata_sql_ssl_client_key_secret":  types.StringType,
		"metadata_sql_uri_secret":             types.StringType,
		"offline_store_inserter_db_type":      types.StringType,
		"storage_cache_prefix":                types.StringType,
		"results_writer_skip_producing_feature_metrics": types.BoolType,
		"query_table_write_drop_ratio":                  types.StringType,
		"additional_env_vars":                           types.MapType{ElemType: types.StringType},
	},
}

// bgpWritersTFToProto converts a TF writers list to a proto slice.
func bgpWritersTFToProto(ctx context.Context, writersList types.List) ([]*serverv1.BackgroundPersistenceWriterSpecs, diag.Diagnostics) {
	var writers []BackgroundPersistenceWriterModel
	diags := writersList.ElementsAs(ctx, &writers, false)
	if diags.HasError() {
		return nil, diags
	}

	var protoWriters []*serverv1.BackgroundPersistenceWriterSpecs
	for _, writer := range writers {
		protoWriter := &serverv1.BackgroundPersistenceWriterSpecs{
			Name:              writer.Name.ValueString(),
			BusSubscriberType: writer.BusSubscriberType.ValueString(),
		}

		if !writer.ImageOverride.IsNull() {
			protoWriter.ImageOverride = writer.ImageOverride.ValueString()
		}
		if !writer.Version.IsNull() {
			protoWriter.Version = writer.Version.ValueString()
		}
		if !writer.DefaultReplicaCount.IsNull() {
			protoWriter.DefaultReplicaCount = int32(writer.DefaultReplicaCount.ValueInt64())
		}
		if !writer.MetadataSqlSslCaCertSecret.IsNull() {
			protoWriter.MetadataSqlSslCaCertSecret = writer.MetadataSqlSslCaCertSecret.ValueString()
		}
		if !writer.MetadataSqlSslClientCertSecret.IsNull() {
			protoWriter.MetadataSqlSslClientCertSecret = writer.MetadataSqlSslClientCertSecret.ValueString()
		}
		if !writer.MetadataSqlSslClientKeySecret.IsNull() {
			protoWriter.MetadataSqlSslClientKeySecret = writer.MetadataSqlSslClientKeySecret.ValueString()
		}
		if !writer.MetadataSqlUriSecret.IsNull() {
			protoWriter.MetadataSqlUriSecret = writer.MetadataSqlUriSecret.ValueString()
		}
		if !writer.OfflineStoreInserterDbType.IsNull() {
			protoWriter.OfflineStoreInserterDbType = writer.OfflineStoreInserterDbType.ValueString()
		}
		if !writer.StorageCachePrefix.IsNull() {
			protoWriter.StorageCachePrefix = writer.StorageCachePrefix.ValueString()
		}
		if !writer.QueryTableWriteDropRatio.IsNull() {
			protoWriter.QueryTableWriteDropRatio = writer.QueryTableWriteDropRatio.ValueString()
		}
		if !writer.GkeSpot.IsNull() {
			val := writer.GkeSpot.ValueBool()
			protoWriter.GkeSpot = &val
		}
		if !writer.LoadWriterConfigmap.IsNull() {
			val := writer.LoadWriterConfigmap.ValueBool()
			protoWriter.LoadWriterConfigmap = &val
		}
		if !writer.KafkaConsumerGroupOverride.IsNull() {
			protoWriter.KafkaConsumerGroupOverride = writer.KafkaConsumerGroupOverride.ValueString()
		}
		if !writer.MaxBatchSize.IsNull() {
			val := int32(writer.MaxBatchSize.ValueInt64())
			protoWriter.MaxBatchSize = &val
		}
		if !writer.MessageProcessingConcurrency.IsNull() {
			val := int32(writer.MessageProcessingConcurrency.ValueInt64())
			protoWriter.MessageProcessingConcurrency = &val
		}
		if !writer.ResultsWriterSkipProducingFeatureMetrics.IsNull() {
			val := writer.ResultsWriterSkipProducingFeatureMetrics.ValueBool()
			protoWriter.ResultsWriterSkipProducingFeatureMetrics = &val
		}

		if !writer.AdditionalEnvVars.IsNull() && !writer.AdditionalEnvVars.IsUnknown() {
			var envVars map[string]string
			diags.Append(writer.AdditionalEnvVars.ElementsAs(ctx, &envVars, false)...)
			if !diags.HasError() {
				protoWriter.AdditionalEnvVars = envVars
			}
		}

		if writer.HpaSpecs != nil {
			protoWriter.HpaSpecs = &serverv1.BackgroundPersistenceWriterHpaSpecs{
				HpaPubsubSubscriptionId: writer.HpaSpecs.HpaPubsubSubscriptionId.ValueString(),
			}
			if !writer.HpaSpecs.HpaMinReplicas.IsNull() {
				val := int32(writer.HpaSpecs.HpaMinReplicas.ValueInt64())
				protoWriter.HpaSpecs.HpaMinReplicas = &val
			}
			if !writer.HpaSpecs.HpaMaxReplicas.IsNull() {
				val := int32(writer.HpaSpecs.HpaMaxReplicas.ValueInt64())
				protoWriter.HpaSpecs.HpaMaxReplicas = &val
			}
			if !writer.HpaSpecs.HpaTargetAverageValue.IsNull() {
				val := int32(writer.HpaSpecs.HpaTargetAverageValue.ValueInt64())
				protoWriter.HpaSpecs.HpaTargetAverageValue = &val
			}
		}

		if writer.Request != nil {
			protoWriter.Request = &serverv1.KubeResourceConfig{}
			if !writer.Request.CPU.IsNull() {
				protoWriter.Request.Cpu = writer.Request.CPU.ValueString()
			}
			if !writer.Request.Memory.IsNull() {
				protoWriter.Request.Memory = writer.Request.Memory.ValueString()
			}
			if !writer.Request.EphemeralStorage.IsNull() {
				protoWriter.Request.EphemeralStorage = writer.Request.EphemeralStorage.ValueString()
			}
			if !writer.Request.Storage.IsNull() {
				protoWriter.Request.Storage = writer.Request.Storage.ValueString()
			}
		}
		if writer.Limit != nil {
			protoWriter.Limit = &serverv1.KubeResourceConfig{}
			if !writer.Limit.CPU.IsNull() {
				protoWriter.Limit.Cpu = writer.Limit.CPU.ValueString()
			}
			if !writer.Limit.Memory.IsNull() {
				protoWriter.Limit.Memory = writer.Limit.Memory.ValueString()
			}
			if !writer.Limit.EphemeralStorage.IsNull() {
				protoWriter.Limit.EphemeralStorage = writer.Limit.EphemeralStorage.ValueString()
			}
			if !writer.Limit.Storage.IsNull() {
				protoWriter.Limit.Storage = writer.Limit.Storage.ValueString()
			}
		}

		protoWriters = append(protoWriters, protoWriter)
	}
	return protoWriters, diags
}

// bgpWritersProtoToTF converts a proto writers slice to a TF list.
func bgpWritersProtoToTF(ctx context.Context, protoWriters []*serverv1.BackgroundPersistenceWriterSpecs) (types.List, diag.Diagnostics) {
	if len(protoWriters) == 0 {
		return types.ListValueMust(bgpWriterObjectType, []attr.Value{}), nil
	}

	var (
		diags     diag.Diagnostics
		tfWriters []BackgroundPersistenceWriterModel
	)
	for _, protoWriter := range protoWriters {
		tfWriter := BackgroundPersistenceWriterModel{
			Name:              types.StringValue(protoWriter.Name),
			BusSubscriberType: types.StringValue(protoWriter.BusSubscriberType),
		}

		if protoWriter.ImageOverride != "" {
			tfWriter.ImageOverride = types.StringValue(protoWriter.ImageOverride)
		} else {
			tfWriter.ImageOverride = types.StringNull()
		}
		if protoWriter.Version != "" {
			tfWriter.Version = types.StringValue(protoWriter.Version)
		} else {
			tfWriter.Version = types.StringNull()
		}
		if protoWriter.DefaultReplicaCount != 0 {
			tfWriter.DefaultReplicaCount = types.Int64Value(int64(protoWriter.DefaultReplicaCount))
		} else {
			tfWriter.DefaultReplicaCount = types.Int64Null()
		}
		if protoWriter.MetadataSqlSslCaCertSecret != "" {
			tfWriter.MetadataSqlSslCaCertSecret = types.StringValue(protoWriter.MetadataSqlSslCaCertSecret)
		} else {
			tfWriter.MetadataSqlSslCaCertSecret = types.StringNull()
		}
		if protoWriter.MetadataSqlSslClientCertSecret != "" {
			tfWriter.MetadataSqlSslClientCertSecret = types.StringValue(protoWriter.MetadataSqlSslClientCertSecret)
		} else {
			tfWriter.MetadataSqlSslClientCertSecret = types.StringNull()
		}
		if protoWriter.MetadataSqlSslClientKeySecret != "" {
			tfWriter.MetadataSqlSslClientKeySecret = types.StringValue(protoWriter.MetadataSqlSslClientKeySecret)
		} else {
			tfWriter.MetadataSqlSslClientKeySecret = types.StringNull()
		}
		if protoWriter.MetadataSqlUriSecret != "" {
			tfWriter.MetadataSqlUriSecret = types.StringValue(protoWriter.MetadataSqlUriSecret)
		} else {
			tfWriter.MetadataSqlUriSecret = types.StringNull()
		}
		if protoWriter.OfflineStoreInserterDbType != "" {
			tfWriter.OfflineStoreInserterDbType = types.StringValue(protoWriter.OfflineStoreInserterDbType)
		} else {
			tfWriter.OfflineStoreInserterDbType = types.StringNull()
		}
		if protoWriter.StorageCachePrefix != "" {
			tfWriter.StorageCachePrefix = types.StringValue(protoWriter.StorageCachePrefix)
		} else {
			tfWriter.StorageCachePrefix = types.StringNull()
		}
		// Optional+Computed+Default("0.0"): fall back to the schema default when
		// the server omits the field, otherwise the framework reapplies the
		// default on every plan and shows "+" noise on imported resources.
		if protoWriter.QueryTableWriteDropRatio != "" {
			tfWriter.QueryTableWriteDropRatio = types.StringValue(protoWriter.QueryTableWriteDropRatio)
		} else {
			tfWriter.QueryTableWriteDropRatio = types.StringValue("0.0")
		}
		if protoWriter.KafkaConsumerGroupOverride != "" {
			tfWriter.KafkaConsumerGroupOverride = types.StringValue(protoWriter.KafkaConsumerGroupOverride)
		} else {
			tfWriter.KafkaConsumerGroupOverride = types.StringNull()
		}
		// Optional+Computed+Default(false) — see note above.
		if protoWriter.GkeSpot != nil {
			tfWriter.GkeSpot = types.BoolValue(*protoWriter.GkeSpot)
		} else {
			tfWriter.GkeSpot = types.BoolValue(false)
		}
		if protoWriter.LoadWriterConfigmap != nil {
			tfWriter.LoadWriterConfigmap = types.BoolValue(*protoWriter.LoadWriterConfigmap)
		} else {
			tfWriter.LoadWriterConfigmap = types.BoolValue(false)
		}
		if protoWriter.MaxBatchSize != nil {
			tfWriter.MaxBatchSize = types.Int64Value(int64(*protoWriter.MaxBatchSize))
		} else {
			tfWriter.MaxBatchSize = types.Int64Null()
		}
		if protoWriter.MessageProcessingConcurrency != nil {
			tfWriter.MessageProcessingConcurrency = types.Int64Value(int64(*protoWriter.MessageProcessingConcurrency))
		} else {
			tfWriter.MessageProcessingConcurrency = types.Int64Null()
		}
		if protoWriter.ResultsWriterSkipProducingFeatureMetrics != nil {
			tfWriter.ResultsWriterSkipProducingFeatureMetrics = types.BoolValue(*protoWriter.ResultsWriterSkipProducingFeatureMetrics)
		} else {
			tfWriter.ResultsWriterSkipProducingFeatureMetrics = types.BoolValue(false)
		}

		if len(protoWriter.AdditionalEnvVars) > 0 {
			mapVal, mapDiags := types.MapValueFrom(ctx, types.StringType, protoWriter.AdditionalEnvVars)
			diags.Append(mapDiags...)
			tfWriter.AdditionalEnvVars = mapVal
		} else {
			tfWriter.AdditionalEnvVars = types.MapNull(types.StringType)
		}

		// When the server returns a non-nil HpaSpecs — including the common empty
		// `{}` shape — populate every Computed+Default sub-field so the framework
		// does not reapply defaults on the next plan. HpaPubsubSubscriptionId is
		// Optional on the schema: treat empty string from the server as null.
		if protoWriter.HpaSpecs != nil {
			tfWriter.HpaSpecs = &BackgroundPersistenceWriterHpaModel{}
			if protoWriter.HpaSpecs.HpaPubsubSubscriptionId != "" {
				tfWriter.HpaSpecs.HpaPubsubSubscriptionId = types.StringValue(protoWriter.HpaSpecs.HpaPubsubSubscriptionId)
			} else {
				tfWriter.HpaSpecs.HpaPubsubSubscriptionId = types.StringNull()
			}
			if protoWriter.HpaSpecs.HpaMinReplicas != nil {
				tfWriter.HpaSpecs.HpaMinReplicas = types.Int64Value(int64(*protoWriter.HpaSpecs.HpaMinReplicas))
			} else {
				tfWriter.HpaSpecs.HpaMinReplicas = types.Int64Value(1)
			}
			if protoWriter.HpaSpecs.HpaMaxReplicas != nil {
				tfWriter.HpaSpecs.HpaMaxReplicas = types.Int64Value(int64(*protoWriter.HpaSpecs.HpaMaxReplicas))
			} else {
				tfWriter.HpaSpecs.HpaMaxReplicas = types.Int64Value(10)
			}
			if protoWriter.HpaSpecs.HpaTargetAverageValue != nil {
				tfWriter.HpaSpecs.HpaTargetAverageValue = types.Int64Value(int64(*protoWriter.HpaSpecs.HpaTargetAverageValue))
			} else {
				tfWriter.HpaSpecs.HpaTargetAverageValue = types.Int64Value(5)
			}
		}

		if protoWriter.Request != nil {
			tfWriter.Request = &KubeResourceConfigModel{}
			if protoWriter.Request.Cpu != "" {
				tfWriter.Request.CPU = types.StringValue(protoWriter.Request.Cpu)
			} else {
				tfWriter.Request.CPU = types.StringNull()
			}
			if protoWriter.Request.Memory != "" {
				tfWriter.Request.Memory = types.StringValue(protoWriter.Request.Memory)
			} else {
				tfWriter.Request.Memory = types.StringNull()
			}
			if protoWriter.Request.EphemeralStorage != "" {
				tfWriter.Request.EphemeralStorage = types.StringValue(protoWriter.Request.EphemeralStorage)
			} else {
				tfWriter.Request.EphemeralStorage = types.StringNull()
			}
			if protoWriter.Request.Storage != "" {
				tfWriter.Request.Storage = types.StringValue(protoWriter.Request.Storage)
			} else {
				tfWriter.Request.Storage = types.StringNull()
			}
		}
		if protoWriter.Limit != nil {
			tfWriter.Limit = &KubeResourceConfigModel{}
			if protoWriter.Limit.Cpu != "" {
				tfWriter.Limit.CPU = types.StringValue(protoWriter.Limit.Cpu)
			} else {
				tfWriter.Limit.CPU = types.StringNull()
			}
			if protoWriter.Limit.Memory != "" {
				tfWriter.Limit.Memory = types.StringValue(protoWriter.Limit.Memory)
			} else {
				tfWriter.Limit.Memory = types.StringNull()
			}
			if protoWriter.Limit.EphemeralStorage != "" {
				tfWriter.Limit.EphemeralStorage = types.StringValue(protoWriter.Limit.EphemeralStorage)
			} else {
				tfWriter.Limit.EphemeralStorage = types.StringNull()
			}
			if protoWriter.Limit.Storage != "" {
				tfWriter.Limit.Storage = types.StringValue(protoWriter.Limit.Storage)
			} else {
				tfWriter.Limit.Storage = types.StringNull()
			}
		}

		tfWriters = append(tfWriters, tfWriter)
	}
	if diags.HasError() {
		return types.ListNull(bgpWriterObjectType), diags
	}
	listVal, listDiags := types.ListValueFrom(ctx, bgpWriterObjectType, tfWriters)
	diags.Append(listDiags...)
	return listVal, diags
}

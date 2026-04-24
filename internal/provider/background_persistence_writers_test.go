package provider

import (
	"context"
	"fmt"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeSingleWriter runs bgpWritersProtoToTF for a single writer and returns
// the decoded Go model plus any diagnostics.
func decodeSingleWriter(t *testing.T, w *serverv1.BackgroundPersistenceWriterSpecs) BackgroundPersistenceWriterModel {
	t.Helper()
	ctx := context.Background()
	list, diags := bgpWritersProtoToTF(ctx, []*serverv1.BackgroundPersistenceWriterSpecs{w})
	require.False(t, diags.HasError(), "unexpected diagnostics: %v", diags)
	require.False(t, list.IsNull(), "decoded list should not be null")
	var models []BackgroundPersistenceWriterModel
	diags = list.ElementsAs(ctx, &models, false)
	require.False(t, diags.HasError(), "ElementsAs diagnostics: %v", diags)
	require.Len(t, models, 1)
	return models[0]
}

// TestBgpWritersProtoToTF_MinimalServerResponse: server returns only name,
// bus_subscriber_type, request, default_replica_count. All other attributes,
// including the formerly-Optional+Computed+Default ones, should decode to
// null so HCL omission matches state.
func TestBgpWritersProtoToTF_MinimalServerResponse(t *testing.T) {
	t.Parallel()
	w := &serverv1.BackgroundPersistenceWriterSpecs{
		Name:                "go-metrics-bus-writer",
		BusSubscriberType:   "GO_METRICS_BUS_WRITER",
		DefaultReplicaCount: 1,
		Request:             &serverv1.KubeResourceConfig{Cpu: "500m", Memory: "1Gi"},
	}
	m := decodeSingleWriter(t, w)

	assert.Equal(t, types.BoolNull(), m.GkeSpot)
	assert.Equal(t, types.BoolNull(), m.LoadWriterConfigmap)
	assert.Equal(t, types.BoolNull(), m.ResultsWriterSkipProducingFeatureMetrics)
	assert.Equal(t, types.StringNull(), m.QueryTableWriteDropRatio)
	assert.Nil(t, m.HpaSpecs, "hpa_specs should remain nil when server didn't send it")
}

// TestBgpWritersProtoToTF_HpaSpecsMaxReplicasOnly covers the most common
// populated hpa_specs shape in production (~120/150 populated writers): only
// hpa_max_replicas is set alongside the required pubsub id.
func TestBgpWritersProtoToTF_HpaSpecsMaxReplicasOnly(t *testing.T) {
	t.Parallel()
	w := &serverv1.BackgroundPersistenceWriterSpecs{
		Name:              "online-writer",
		BusSubscriberType: "ONLINE_WRITER",
		HpaSpecs: &serverv1.BackgroundPersistenceWriterHpaSpecs{
			HpaPubsubSubscriptionId: "chalk-port-prod-result-bus-sub-onlinewriter",
			HpaMaxReplicas:          new(int32(1)),
		},
	}
	m := decodeSingleWriter(t, w)

	require.NotNil(t, m.HpaSpecs)
	assert.Equal(t, types.StringValue("chalk-port-prod-result-bus-sub-onlinewriter"), m.HpaSpecs.HpaPubsubSubscriptionId)
	assert.Equal(t, types.Int64Value(1), m.HpaSpecs.HpaMaxReplicas)
	assert.Equal(t, types.Int64Null(), m.HpaSpecs.HpaMinReplicas)
	assert.Equal(t, types.Int64Null(), m.HpaSpecs.HpaTargetAverageValue)
}

// TestBgpWritersProtoToTF_HpaMinReplicasZero ensures a real non-default value of 0
// (observed in 14 real-world configs) is preserved, not coerced to null.
func TestBgpWritersProtoToTF_HpaMinReplicasZero(t *testing.T) {
	t.Parallel()
	w := &serverv1.BackgroundPersistenceWriterSpecs{
		Name:              "rust-offline-writer",
		BusSubscriberType: "RUST_OFFLINE_WRITER",
		HpaSpecs: &serverv1.BackgroundPersistenceWriterHpaSpecs{
			HpaPubsubSubscriptionId: "sub-id",
			HpaMinReplicas:          new(int32(0)),
			HpaMaxReplicas:          new(int32(4)),
		},
	}
	m := decodeSingleWriter(t, w)

	require.NotNil(t, m.HpaSpecs)
	assert.Equal(t, types.Int64Value(0), m.HpaSpecs.HpaMinReplicas, "0 must be preserved")
	assert.Equal(t, types.Int64Value(4), m.HpaSpecs.HpaMaxReplicas)
	assert.Equal(t, types.Int64Null(), m.HpaSpecs.HpaTargetAverageValue)
}

// TestBgpWritersProtoToTF_HpaMaxReplicasZero is a symmetry guard for the zero case
// on max_replicas (observed in 2 real-world configs).
func TestBgpWritersProtoToTF_HpaMaxReplicasZero(t *testing.T) {
	t.Parallel()
	w := &serverv1.BackgroundPersistenceWriterSpecs{
		Name:              "offline-writer",
		BusSubscriberType: "OFFLINE_WRITER",
		HpaSpecs: &serverv1.BackgroundPersistenceWriterHpaSpecs{
			HpaPubsubSubscriptionId: "sub-id",
			HpaMaxReplicas:          new(int32(0)),
		},
	}
	m := decodeSingleWriter(t, w)

	require.NotNil(t, m.HpaSpecs)
	assert.Equal(t, types.Int64Value(0), m.HpaSpecs.HpaMaxReplicas, "0 must be preserved")
	assert.Equal(t, types.Int64Null(), m.HpaSpecs.HpaMinReplicas)
	assert.Equal(t, types.Int64Null(), m.HpaSpecs.HpaTargetAverageValue)
}

// TestBgpWritersProtoToTF_FullyPopulatedHpaSpecs guards the happy path — a writer
// with all four hpa sub-fields set.
func TestBgpWritersProtoToTF_FullyPopulatedHpaSpecs(t *testing.T) {
	t.Parallel()
	w := &serverv1.BackgroundPersistenceWriterSpecs{
		Name:              "online-writer",
		BusSubscriberType: "ONLINE_WRITER",
		HpaSpecs: &serverv1.BackgroundPersistenceWriterHpaSpecs{
			HpaPubsubSubscriptionId: "chalk-port-prod-result-bus-sub-onlinewriter",
			HpaMinReplicas:          new(int32(2)),
			HpaMaxReplicas:          new(int32(64)),
			HpaTargetAverageValue:   new(int32(1000)),
		},
	}
	m := decodeSingleWriter(t, w)

	require.NotNil(t, m.HpaSpecs)
	assert.Equal(t, types.StringValue("chalk-port-prod-result-bus-sub-onlinewriter"), m.HpaSpecs.HpaPubsubSubscriptionId)
	assert.Equal(t, types.Int64Value(2), m.HpaSpecs.HpaMinReplicas)
	assert.Equal(t, types.Int64Value(64), m.HpaSpecs.HpaMaxReplicas)
	assert.Equal(t, types.Int64Value(1000), m.HpaSpecs.HpaTargetAverageValue)
}

// TestBgpWritersProtoToTF_GkeSpotExplicit covers both truthy and falsy server values
// (both appear in production).
func TestBgpWritersProtoToTF_GkeSpotExplicit(t *testing.T) {
	t.Parallel()
	for _, v := range []bool{true, false} {
		t.Run(fmt.Sprintf("gke_spot=%v", v), func(t *testing.T) {
			t.Parallel()
			w := &serverv1.BackgroundPersistenceWriterSpecs{
				Name:              "writer",
				BusSubscriberType: "ONLINE_WRITER",
				GkeSpot:           new(v),
			}
			m := decodeSingleWriter(t, w)
			assert.Equal(t, types.BoolValue(v), m.GkeSpot)
		})
	}
}

// TestBgpWritersProtoToTF_ExplicitServerValues guards that concrete server values
// round-trip untouched — e.g. load_writer_configmap=false and
// query_table_write_drop_ratio="0.0" appear in 32 real cases each.
func TestBgpWritersProtoToTF_ExplicitServerValues(t *testing.T) {
	t.Parallel()
	w := &serverv1.BackgroundPersistenceWriterSpecs{
		Name:                                     "go-metrics-bus-writer",
		BusSubscriberType:                        "GO_METRICS_BUS_WRITER",
		LoadWriterConfigmap:                      new(false),
		QueryTableWriteDropRatio:                 "0.0",
		ResultsWriterSkipProducingFeatureMetrics: new(false),
	}
	m := decodeSingleWriter(t, w)

	assert.Equal(t, types.BoolValue(false), m.LoadWriterConfigmap)
	assert.Equal(t, types.StringValue("0.0"), m.QueryTableWriteDropRatio)
	assert.Equal(t, types.BoolValue(false), m.ResultsWriterSkipProducingFeatureMetrics)
}

// TestBgpWritersProtoToTF_ProtoRoundTrip converts proto -> TF -> proto and asserts
// semantic equality for a fully-populated writer. Guards the decoder from drifting
// away from the encoder.
func TestBgpWritersProtoToTF_ProtoRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	original := &serverv1.BackgroundPersistenceWriterSpecs{
		Name:                                     "online-writer",
		BusSubscriberType:                        "ONLINE_WRITER",
		DefaultReplicaCount:                      3,
		ImageOverride:                            "custom-image:v1",
		Version:                                  "v1.2.3",
		QueryTableWriteDropRatio:                 "0.5",
		GkeSpot:                                  new(true),
		LoadWriterConfigmap:                      new(true),
		ResultsWriterSkipProducingFeatureMetrics: new(true),
		MaxBatchSize:                             new(int32(100)),
		MessageProcessingConcurrency:             new(int32(8)),
		KafkaConsumerGroupOverride:               "group-override",
		OfflineStoreInserterDbType:               "bigquery",
		StorageCachePrefix:                       "cache/",
		MetadataSqlUriSecret:                     "secret-uri",
		MetadataSqlSslCaCertSecret:               "secret-ca",
		MetadataSqlSslClientCertSecret:           "secret-client-cert",
		MetadataSqlSslClientKeySecret:            "secret-client-key",
		Request:                                  &serverv1.KubeResourceConfig{Cpu: "500m", Memory: "1Gi"},
		Limit:                                    &serverv1.KubeResourceConfig{Cpu: "1", Memory: "2Gi"},
		HpaSpecs: &serverv1.BackgroundPersistenceWriterHpaSpecs{
			HpaPubsubSubscriptionId: "sub-id",
			HpaMinReplicas:          new(int32(2)),
			HpaMaxReplicas:          new(int32(10)),
			HpaTargetAverageValue:   new(int32(100)),
		},
		AdditionalEnvVars: map[string]string{"FOO": "bar"},
	}

	tfList, diags := bgpWritersProtoToTF(ctx, []*serverv1.BackgroundPersistenceWriterSpecs{original})
	require.False(t, diags.HasError(), "decode diagnostics: %v", diags)

	out, diags := bgpWritersTFToProto(ctx, tfList)
	require.False(t, diags.HasError(), "encode diagnostics: %v", diags)
	require.Len(t, out, 1)
	roundTripped := out[0]

	assert.Equal(t, original.Name, roundTripped.Name)
	assert.Equal(t, original.BusSubscriberType, roundTripped.BusSubscriberType)
	assert.Equal(t, original.DefaultReplicaCount, roundTripped.DefaultReplicaCount)
	assert.Equal(t, original.ImageOverride, roundTripped.ImageOverride)
	assert.Equal(t, original.Version, roundTripped.Version)
	assert.Equal(t, original.QueryTableWriteDropRatio, roundTripped.QueryTableWriteDropRatio)
	assert.Equal(t, original.GkeSpot, roundTripped.GkeSpot)
	assert.Equal(t, original.LoadWriterConfigmap, roundTripped.LoadWriterConfigmap)
	assert.Equal(t, original.ResultsWriterSkipProducingFeatureMetrics, roundTripped.ResultsWriterSkipProducingFeatureMetrics)
	assert.Equal(t, original.MaxBatchSize, roundTripped.MaxBatchSize)
	assert.Equal(t, original.MessageProcessingConcurrency, roundTripped.MessageProcessingConcurrency)
	assert.Equal(t, original.KafkaConsumerGroupOverride, roundTripped.KafkaConsumerGroupOverride)
	assert.Equal(t, original.OfflineStoreInserterDbType, roundTripped.OfflineStoreInserterDbType)
	assert.Equal(t, original.StorageCachePrefix, roundTripped.StorageCachePrefix)
	assert.Equal(t, original.MetadataSqlUriSecret, roundTripped.MetadataSqlUriSecret)
	assert.Equal(t, original.MetadataSqlSslCaCertSecret, roundTripped.MetadataSqlSslCaCertSecret)
	assert.Equal(t, original.MetadataSqlSslClientCertSecret, roundTripped.MetadataSqlSslClientCertSecret)
	assert.Equal(t, original.MetadataSqlSslClientKeySecret, roundTripped.MetadataSqlSslClientKeySecret)
	assert.Equal(t, original.Request.Cpu, roundTripped.Request.Cpu)
	assert.Equal(t, original.Request.Memory, roundTripped.Request.Memory)
	assert.Equal(t, original.Limit.Cpu, roundTripped.Limit.Cpu)
	assert.Equal(t, original.Limit.Memory, roundTripped.Limit.Memory)
	require.NotNil(t, roundTripped.HpaSpecs)
	assert.Equal(t, original.HpaSpecs.HpaPubsubSubscriptionId, roundTripped.HpaSpecs.HpaPubsubSubscriptionId)
	assert.Equal(t, original.HpaSpecs.HpaMinReplicas, roundTripped.HpaSpecs.HpaMinReplicas)
	assert.Equal(t, original.HpaSpecs.HpaMaxReplicas, roundTripped.HpaSpecs.HpaMaxReplicas)
	assert.Equal(t, original.HpaSpecs.HpaTargetAverageValue, roundTripped.HpaSpecs.HpaTargetAverageValue)
	assert.Equal(t, original.AdditionalEnvVars, roundTripped.AdditionalEnvVars)
}

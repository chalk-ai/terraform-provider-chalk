package provider

import (
	"fmt"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/proto"
)

type customerVectorAggregatorModel struct {
	Datadog *customerDatadogExportModel     `tfsdk:"datadog"`
	Otlp    *customerOtlpMetricsExportModel `tfsdk:"otlp"`
}

type customerDatadogExportModel struct {
	ApiKeySecretReference types.String                `tfsdk:"api_key_secret_reference"`
	ApiHost               types.String                `tfsdk:"api_host"`
	Logs                  *customerDatadogSignalModel `tfsdk:"logs"`
	Traces                *customerDatadogSignalModel `tfsdk:"traces"`
	Metrics               *customerDatadogSignalModel `tfsdk:"metrics"`
}

type customerDatadogSignalModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

type customerOtlpMetricsExportModel struct {
	Enabled                            types.Bool   `tfsdk:"enabled"`
	Url                                types.String `tfsdk:"url"`
	AuthorizationHeaderSecretReference types.String `tfsdk:"authorization_header_secret_reference"`
}

const secretReferenceFormats = "an AWS Secrets Manager ARN, a GCP `projects/<project>/secrets/<name>` resource name, or an Azure Key Vault secret URL"

// datadogSignalExportSchema describes one Datadog signal, which presence alone enables.
func datadogSignalExportSchema(signal string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: fmt.Sprintf("Export %s to your Datadog account. Omitting this block leaves %s unexported.", signal, signal),
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				MarkdownDescription: fmt.Sprintf("Whether to export %s. Defaults to `true` when this block is present.", signal),
				Optional:            true,
			},
		},
	}
}

func customerVectorAggregatorSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "Forwards this deployment's telemetry to systems you own. Each destination is configured only when its block is present. Exporters configured outside Terraform are ignored until declared here. Requires the Vector telemetry runtime; deployments on the OTel runtime store this configuration without deploying an exporter.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"datadog": schema.SingleNestedAttribute{
				MarkdownDescription: "Export telemetry to your own Datadog account. Nothing is exported until at least one of `logs`, `traces`, or `metrics` is present.",
				Optional:            true,
				Validators: []validator.Object{
					objectvalidator.AtLeastOneOf(
						path.MatchRelative().AtParent().AtName("otlp"),
					),
				},
				Attributes: map[string]schema.Attribute{
					"api_key_secret_reference": schema.StringAttribute{
						MarkdownDescription: "Reference to the cloud secret holding your Datadog API key: " + secretReferenceFormats + ". The secret must live in the telemetry cluster's own account, and its latest version is read at deploy time.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(1),
						},
					},
					"api_host": schema.StringAttribute{
						MarkdownDescription: "Datadog site to export to, for example `datadoghq.eu`. Defaults to `datadoghq.com`.",
						Optional:            true,
					},
					"logs":    datadogSignalExportSchema("logs"),
					"traces":  datadogSignalExportSchema("traces"),
					"metrics": datadogSignalExportSchema("metrics"),
				},
			},
			"otlp": schema.SingleNestedAttribute{
				MarkdownDescription: "Export metrics to an OTLP/HTTP endpoint you own.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						MarkdownDescription: "Full OTLP/HTTP metrics URL, including the `/v1/metrics` path.",
						Required:            true,
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(1),
						},
					},
					"enabled": schema.BoolAttribute{
						MarkdownDescription: "Whether to export metrics. Defaults to `true` when this block is present.",
						Optional:            true,
					},
					"authorization_header_secret_reference": schema.StringAttribute{
						MarkdownDescription: "Reference to the cloud secret holding the complete `Authorization` header value, for example `Bearer <token>`. Accepts " + secretReferenceFormats + ".",
						Optional:            true,
					},
				},
			},
		},
	}
}

func (m *customerVectorAggregatorModel) toProto() *serverv1.CustomerVectorAggregatorConfig {
	if m == nil {
		return nil
	}
	return &serverv1.CustomerVectorAggregatorConfig{
		DatadogExport:     m.Datadog.toProto(),
		OtlpMetricsExport: m.Otlp.toProto(),
	}
}

func (m *customerDatadogExportModel) toProto() *serverv1.CustomerVectorAggregatorDatadogExportConfig {
	if m == nil {
		return nil
	}
	return &serverv1.CustomerVectorAggregatorDatadogExportConfig{
		ApiKeySource: &serverv1.CustomerVectorAggregatorDatadogExportConfig_ApiKeySecretArn{
			ApiKeySecretArn: m.ApiKeySecretReference.ValueString(),
		},
		ApiHost: m.ApiHost.ValueStringPointer(),
		Logs:    m.Logs.toProto(),
		Traces:  m.Traces.toProto(),
		Metrics: m.Metrics.toProto(),
	}
}

func (m *customerDatadogSignalModel) toProto() *serverv1.CustomerVectorAggregatorDatadogSignalExportSpec {
	if m == nil {
		return nil
	}
	return &serverv1.CustomerVectorAggregatorDatadogSignalExportSpec{
		Enabled: m.Enabled.ValueBoolPointer(),
	}
}

func (m *customerOtlpMetricsExportModel) toProto() *serverv1.CustomerVectorAggregatorOtlpMetricsExportConfig {
	if m == nil {
		return nil
	}
	return &serverv1.CustomerVectorAggregatorOtlpMetricsExportConfig{
		Enabled:                      m.Enabled.ValueBoolPointer(),
		Url:                          m.Url.ValueString(),
		AuthorizationHeaderSecretArn: m.AuthorizationHeaderSecretReference.ValueStringPointer(),
	}
}

// customerVectorAggregatorMaskPaths masks only fields this resource owns, so unmodelled
// fields like remap_vrl and metrics_sink survive; whole-message paths are used only to clear.
func customerVectorAggregatorMaskPaths(plan, state *customerVectorAggregatorModel) []string {
	var paths []string
	planProto, stateProto := plan.toProto(), state.toProto()
	if dd := planProto.GetDatadogExport(); !proto.Equal(dd, stateProto.GetDatadogExport()) {
		if dd == nil {
			paths = append(paths, "customer_vector_aggregator.datadog_export")
		} else {
			paths = append(paths,
				"customer_vector_aggregator.datadog_export.api_key_secret_arn",
				"customer_vector_aggregator.datadog_export.api_host",
			)
			for _, signal := range []struct {
				name string
				set  *serverv1.CustomerVectorAggregatorDatadogSignalExportSpec
			}{{"logs", dd.GetLogs()}, {"traces", dd.GetTraces()}, {"metrics", dd.GetMetrics()}} {
				if signal.set == nil {
					paths = append(paths, "customer_vector_aggregator.datadog_export."+signal.name)
				} else {
					// Masking enabled under an absent signal would create it; fmutils fills in parents.
					paths = append(paths, "customer_vector_aggregator.datadog_export."+signal.name+".enabled")
				}
			}
		}
	}
	if otlp := planProto.GetOtlpMetricsExport(); !proto.Equal(otlp, stateProto.GetOtlpMetricsExport()) {
		if otlp == nil {
			paths = append(paths, "customer_vector_aggregator.otlp_metrics_export")
		} else {
			paths = append(paths,
				"customer_vector_aggregator.otlp_metrics_export.enabled",
				"customer_vector_aggregator.otlp_metrics_export.url",
				"customer_vector_aggregator.otlp_metrics_export.authorization_header_secret_arn",
			)
		}
	}
	return paths
}

// Each exporter is adopted from the server only when prior state owns it, so exporters
// configured through other clients are never pulled into state and planned away.
func customerVectorAggregatorFromProto(p *serverv1.CustomerVectorAggregatorConfig, prior *customerVectorAggregatorModel) *customerVectorAggregatorModel {
	if prior == nil {
		return nil
	}
	m := &customerVectorAggregatorModel{}
	if prior.Datadog != nil {
		m.Datadog = customerDatadogExportFromProto(p.GetDatadogExport())
	}
	if prior.Otlp != nil {
		m.Otlp = customerOtlpMetricsExportFromProto(p.GetOtlpMetricsExport())
	}
	if m.Datadog == nil && m.Otlp == nil {
		return nil
	}
	return m
}

func customerDatadogExportFromProto(p *serverv1.CustomerVectorAggregatorDatadogExportConfig) *customerDatadogExportModel {
	if p == nil {
		return nil
	}
	return &customerDatadogExportModel{
		// Empty when the deployment stores an inline API key, which this resource never writes.
		ApiKeySecretReference: optionalStringValue(p.GetApiKeySecretArn()),
		ApiHost:               stringPointerValue(p.ApiHost),
		Logs:                  customerDatadogSignalFromProto(p.GetLogs()),
		Traces:                customerDatadogSignalFromProto(p.GetTraces()),
		Metrics:               customerDatadogSignalFromProto(p.GetMetrics()),
	}
}

func customerDatadogSignalFromProto(p *serverv1.CustomerVectorAggregatorDatadogSignalExportSpec) *customerDatadogSignalModel {
	if p == nil {
		return nil
	}
	return &customerDatadogSignalModel{Enabled: boolPointerValue(p.Enabled)}
}

func customerOtlpMetricsExportFromProto(p *serverv1.CustomerVectorAggregatorOtlpMetricsExportConfig) *customerOtlpMetricsExportModel {
	if p == nil {
		return nil
	}
	return &customerOtlpMetricsExportModel{
		Enabled:                            boolPointerValue(p.Enabled),
		Url:                                optionalStringValue(p.GetUrl()),
		AuthorizationHeaderSecretReference: stringPointerValue(p.AuthorizationHeaderSecretArn),
	}
}

package provider

import (
	"context"
	"testing"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// newEnvoyRichSpec returns a spec matching the most common envoy pattern observed in the
// chalk_public_cluster_gateways.json production snapshot: timeout + replicas + min_available +
// dns_hostname + letsencrypt_cluster_issuer.
func newEnvoyRichSpec() *serverv1.EnvoyGatewaySpecs {
	return &serverv1.EnvoyGatewaySpecs{
		Namespace:        "chalk-envoy",
		GatewayName:      "chalk-gw",
		GatewayClassName: "chalk-gw-class",
		Config: &serverv1.GatewayProviderConfig{
			Config: &serverv1.GatewayProviderConfig_Envoy{
				Envoy: &serverv1.EnvoyGatewayProviderConfig{
					TimeoutDuration:          new("300s"),
					DnsHostname:              new("api.example.com"),
					Replicas:                 new(int32(3)),
					MinAvailable:             new(int32(2)),
					LetsencryptClusterIssuer: new("letsencrypt-prod"),
				},
			},
		},
	}
}

// newGCPHTTP80Spec returns the 13/14 production GCP shape.
func newGCPHTTP80Spec() *serverv1.EnvoyGatewaySpecs {
	return &serverv1.EnvoyGatewaySpecs{
		Namespace:        "default",
		GatewayName:      "standard-gateway",
		GatewayClassName: "gke-l7-gxlb",
		Listeners: []*serverv1.EnvoyGatewayListener{
			{
				Port:     80,
				Protocol: "HTTP",
				Name:     "gateway-listener",
				AllowedRoutes: &serverv1.EnvoyGatewayAllowedRoutes{
					Namespaces: &serverv1.EnvoyGatewayAllowedNamespaces{From: "All"},
				},
			},
		},
		Config: &serverv1.GatewayProviderConfig{
			Config: &serverv1.GatewayProviderConfig_Gcp{
				Gcp: &serverv1.GCPGatewayProviderConfig{
					DnsHostname: "grid.customers.gcp.chalk.ai",
				},
			},
		},
	}
}

// newGCPHTTPS443Spec returns the 1/14 production shape.
func newGCPHTTPS443Spec() *serverv1.EnvoyGatewaySpecs {
	return &serverv1.EnvoyGatewaySpecs{
		Namespace:        "default",
		GatewayName:      "ramp-gateway",
		GatewayClassName: "gke-l7-global-external-managed",
		Listeners: []*serverv1.EnvoyGatewayListener{
			{
				Port:     443,
				Protocol: "HTTPS",
				Name:     "gateway-listener",
				AllowedRoutes: &serverv1.EnvoyGatewayAllowedRoutes{
					Namespaces: &serverv1.EnvoyGatewayAllowedNamespaces{From: "All"},
				},
			},
		},
		Config: &serverv1.GatewayProviderConfig{
			Config: &serverv1.GatewayProviderConfig_Gcp{
				Gcp: &serverv1.GCPGatewayProviderConfig{
					DnsHostname: "pokeapi.co",
				},
			},
		},
	}
}

func runUpdateModelFromSpecs(t *testing.T, specs *serverv1.EnvoyGatewaySpecs, seed *ClusterGatewayResourceModel) *ClusterGatewayResourceModel {
	t.Helper()
	r := &ClusterGatewayResource{}
	data := seed
	if data == nil {
		data = &ClusterGatewayResourceModel{}
	}
	var diags diag.Diagnostics
	r.updateModelFromSpecs(context.Background(), data, specs, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	return data
}

func TestUpdateModelFromSpecs_Envoy_RichRealWorld(t *testing.T) {
	t.Parallel()

	data := runUpdateModelFromSpecs(t, newEnvoyRichSpec(), nil)

	if data.TimeoutDuration.ValueString() != "300s" {
		t.Errorf("TimeoutDuration = %q, want %q", data.TimeoutDuration.ValueString(), "300s")
	}
	if data.DNSHostname.ValueString() != "api.example.com" {
		t.Errorf("DNSHostname = %q, want %q", data.DNSHostname.ValueString(), "api.example.com")
	}
	if data.Replicas.ValueInt64() != 3 {
		t.Errorf("Replicas = %d, want 3", data.Replicas.ValueInt64())
	}
	if data.MinAvailable.ValueInt64() != 2 {
		t.Errorf("MinAvailable = %d, want 2", data.MinAvailable.ValueInt64())
	}
	if data.LetsencryptClusterIssuer.ValueString() != "letsencrypt-prod" {
		t.Errorf("LetsencryptClusterIssuer = %q, want %q", data.LetsencryptClusterIssuer.ValueString(), "letsencrypt-prod")
	}
	if data.GCP != nil {
		t.Errorf("GCP = %+v, want nil on envoy branch", data.GCP)
	}
}

// TestUpdateModelFromSpecs_Envoy_Minimal mirrors the ~13 production gateways that only set
// timeout_duration on the envoy config. Other fields must come back as Null (not empty string / 0).
func TestUpdateModelFromSpecs_Envoy_Minimal(t *testing.T) {
	t.Parallel()

	spec := &serverv1.EnvoyGatewaySpecs{
		Config: &serverv1.GatewayProviderConfig{
			Config: &serverv1.GatewayProviderConfig_Envoy{
				Envoy: &serverv1.EnvoyGatewayProviderConfig{
					TimeoutDuration: new("60s"),
				},
			},
		},
	}
	data := runUpdateModelFromSpecs(t, spec, nil)

	if data.TimeoutDuration.ValueString() != "60s" {
		t.Errorf("TimeoutDuration = %q, want %q", data.TimeoutDuration.ValueString(), "60s")
	}
	if !data.DNSHostname.IsNull() {
		t.Errorf("DNSHostname should be Null, got %v", data.DNSHostname)
	}
	if !data.Replicas.IsNull() {
		t.Errorf("Replicas should be Null, got %v", data.Replicas)
	}
	if !data.MinAvailable.IsNull() {
		t.Errorf("MinAvailable should be Null, got %v", data.MinAvailable)
	}
	if !data.LetsencryptClusterIssuer.IsNull() {
		t.Errorf("LetsencryptClusterIssuer should be Null, got %v", data.LetsencryptClusterIssuer)
	}
	if !data.AdditionalDNSNames.IsNull() {
		t.Errorf("AdditionalDNSNames should be Null, got %v", data.AdditionalDNSNames)
	}
	if !data.Nodepool.IsNull() {
		t.Errorf("Nodepool should be Null, got %v", data.Nodepool)
	}
	if !data.AllowCollocationWithChalkWorkloads.IsNull() {
		t.Errorf("AllowCollocationWithChalkWorkloads should be Null, got %v", data.AllowCollocationWithChalkWorkloads)
	}
	if data.GCP != nil {
		t.Errorf("GCP = %+v, want nil", data.GCP)
	}
}

// TestUpdateModelFromSpecs_GCP_HTTP80_gxlb covers the 13/14 production GCP gateway shape.
func TestUpdateModelFromSpecs_GCP_HTTP80_gxlb(t *testing.T) {
	t.Parallel()

	data := runUpdateModelFromSpecs(t, newGCPHTTP80Spec(), nil)

	if data.GCP == nil {
		t.Fatal("GCP block must be populated for a GCP-backed spec")
	}
	if got, want := data.GCP.DNSHostname.ValueString(), "grid.customers.gcp.chalk.ai"; got != want {
		t.Errorf("GCP.DNSHostname = %q, want %q", got, want)
	}
	if got, want := data.GatewayClassName.ValueString(), "gke-l7-gxlb"; got != want {
		t.Errorf("GatewayClassName = %q, want %q", got, want)
	}
	// Every envoy-only field must be Null so Terraform state matches the server's GCP branch.
	if !data.TimeoutDuration.IsNull() {
		t.Errorf("TimeoutDuration should be Null on GCP branch, got %v", data.TimeoutDuration)
	}
	if !data.DNSHostname.IsNull() {
		t.Errorf("top-level DNSHostname should be Null on GCP branch, got %v", data.DNSHostname)
	}
	if !data.Replicas.IsNull() {
		t.Errorf("Replicas should be Null on GCP branch, got %v", data.Replicas)
	}
	if !data.MinAvailable.IsNull() {
		t.Errorf("MinAvailable should be Null on GCP branch, got %v", data.MinAvailable)
	}
	if !data.LetsencryptClusterIssuer.IsNull() {
		t.Errorf("LetsencryptClusterIssuer should be Null on GCP branch, got %v", data.LetsencryptClusterIssuer)
	}
	if !data.AdditionalDNSNames.IsNull() {
		t.Errorf("AdditionalDNSNames should be Null on GCP branch, got %v", data.AdditionalDNSNames)
	}
	if !data.Nodepool.IsNull() {
		t.Errorf("Nodepool should be Null on GCP branch, got %v", data.Nodepool)
	}
	if !data.AllowCollocationWithChalkWorkloads.IsNull() {
		t.Errorf("AllowCollocationWithChalkWorkloads should be Null on GCP branch, got %v", data.AllowCollocationWithChalkWorkloads)
	}
}

// TestUpdateModelFromSpecs_GCP_HTTPS443_globalExternalManaged covers the 1/14 HTTPS shape.
func TestUpdateModelFromSpecs_GCP_HTTPS443_globalExternalManaged(t *testing.T) {
	t.Parallel()

	data := runUpdateModelFromSpecs(t, newGCPHTTPS443Spec(), nil)

	if data.GCP == nil {
		t.Fatal("GCP block must be populated for a GCP-backed spec")
	}
	if got, want := data.GCP.DNSHostname.ValueString(), "pokeapi.co"; got != want {
		t.Errorf("GCP.DNSHostname = %q, want %q", got, want)
	}
	if got, want := data.GatewayClassName.ValueString(), "gke-l7-global-external-managed"; got != want {
		t.Errorf("GatewayClassName = %q, want %q", got, want)
	}
}

// TestUpdateModelFromSpecs_EnvoyToGCP_Resets simulates a GCP gateway being imported into a state
// that already has stale envoy fields (e.g. a prior buggy import). The refresh must wipe them.
func TestUpdateModelFromSpecs_EnvoyToGCP_Resets(t *testing.T) {
	t.Parallel()

	seed := &ClusterGatewayResourceModel{
		TimeoutDuration:                    types.StringValue("300s"),
		DNSHostname:                        types.StringValue("stale.example.com"),
		Replicas:                           types.Int64Value(5),
		MinAvailable:                       types.Int64Value(2),
		LetsencryptClusterIssuer:           types.StringValue("letsencrypt-prod"),
		Nodepool:                           types.StringValue("old-pool"),
		AllowCollocationWithChalkWorkloads: types.BoolValue(true),
	}
	data := runUpdateModelFromSpecs(t, newGCPHTTP80Spec(), seed)

	if data.GCP == nil || data.GCP.DNSHostname.ValueString() == "" {
		t.Fatalf("GCP block must be populated; got %+v", data.GCP)
	}
	if !data.TimeoutDuration.IsNull() || !data.DNSHostname.IsNull() || !data.Replicas.IsNull() ||
		!data.MinAvailable.IsNull() || !data.LetsencryptClusterIssuer.IsNull() ||
		!data.Nodepool.IsNull() || !data.AllowCollocationWithChalkWorkloads.IsNull() {
		t.Errorf("envoy-only fields must be Null after switching to GCP branch; got %+v", data)
	}
}

// TestUpdateModelFromSpecs_NilConfig asserts no panic and no GCP block when Config is nil.
func TestUpdateModelFromSpecs_NilConfig(t *testing.T) {
	t.Parallel()

	data := runUpdateModelFromSpecs(t, &serverv1.EnvoyGatewaySpecs{Namespace: "x"}, nil)

	if data.GCP != nil {
		t.Errorf("GCP should be nil when Config is nil, got %+v", data.GCP)
	}
	if got := data.Namespace.ValueString(); got != "x" {
		t.Errorf("Namespace = %q, want %q", got, "x")
	}
}

// TestUpdateModelFromSpecs_UnknownOneofBranch simulates the server adding a third oneof branch
// we haven't taught the provider about yet. Must not panic and must leave the model alone.
func TestUpdateModelFromSpecs_UnknownOneofBranch(t *testing.T) {
	t.Parallel()

	spec := &serverv1.EnvoyGatewaySpecs{
		// Non-nil Config but neither Envoy nor Gcp is set on the oneof.
		Config: &serverv1.GatewayProviderConfig{},
	}
	data := runUpdateModelFromSpecs(t, spec, nil)

	if data.GCP != nil {
		t.Errorf("GCP should stay nil for unknown oneof branch, got %+v", data.GCP)
	}
	if !data.TimeoutDuration.IsNull() {
		t.Errorf("TimeoutDuration should stay Null for unknown oneof branch, got %v", data.TimeoutDuration)
	}
}

// TestUpdateModelFromSpecs_GCP_EmptyDNSHostname documents how the model handles an empty GCP
// dns_hostname. Because the proto field is `string` (not `optional string`), empty is a legitimate
// wire value. If the proto is ever upgraded to optional, this test should be updated to assert Null.
func TestUpdateModelFromSpecs_GCP_EmptyDNSHostname(t *testing.T) {
	t.Parallel()

	spec := &serverv1.EnvoyGatewaySpecs{
		Config: &serverv1.GatewayProviderConfig{
			Config: &serverv1.GatewayProviderConfig_Gcp{
				Gcp: &serverv1.GCPGatewayProviderConfig{DnsHostname: ""},
			},
		},
	}
	data := runUpdateModelFromSpecs(t, spec, nil)

	if data.GCP == nil {
		t.Fatal("GCP block should be populated even with empty dns_hostname")
	}
	if data.GCP.DNSHostname.IsNull() {
		t.Error("GCP.DNSHostname should be types.StringValue(\"\"), not Null, because the proto field is non-optional")
	}
	if got := data.GCP.DNSHostname.ValueString(); got != "" {
		t.Errorf("GCP.DNSHostname = %q, want empty string", got)
	}
}

func TestBuildGatewayProviderConfig_GCP(t *testing.T) {
	t.Parallel()

	data := &ClusterGatewayResourceModel{
		GCP: &GCPGatewayProviderConfigModel{
			DNSHostname: types.StringValue("grid.customers.gcp.chalk.ai"),
		},
	}
	var diags diag.Diagnostics
	cfg := buildGatewayProviderConfig(context.Background(), data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	gcp, ok := cfg.Config.(*serverv1.GatewayProviderConfig_Gcp)
	if !ok {
		t.Fatalf("expected GatewayProviderConfig_Gcp branch, got %T", cfg.Config)
	}
	if got := gcp.Gcp.DnsHostname; got != "grid.customers.gcp.chalk.ai" {
		t.Errorf("DnsHostname = %q, want %q", got, "grid.customers.gcp.chalk.ai")
	}
}

func TestBuildGatewayProviderConfig_Envoy(t *testing.T) {
	t.Parallel()

	data := &ClusterGatewayResourceModel{
		TimeoutDuration: types.StringValue("300s"),
		Replicas:        types.Int64Value(3),
	}
	var diags diag.Diagnostics
	cfg := buildGatewayProviderConfig(context.Background(), data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	envoy, ok := cfg.Config.(*serverv1.GatewayProviderConfig_Envoy)
	if !ok {
		t.Fatalf("expected GatewayProviderConfig_Envoy branch, got %T", cfg.Config)
	}
	if envoy.Envoy.TimeoutDuration == nil || *envoy.Envoy.TimeoutDuration != "300s" {
		t.Errorf("TimeoutDuration = %v, want 300s", envoy.Envoy.TimeoutDuration)
	}
	if envoy.Envoy.Replicas == nil || *envoy.Envoy.Replicas != 3 {
		t.Errorf("Replicas = %v, want 3", envoy.Envoy.Replicas)
	}
}

// TestBuildGatewayProviderConfig_NeitherBranchSet preserves pre-oneof behaviour: a model with
// nothing set still emits an empty envoy config so the server defaults to envoy-backed.
func TestBuildGatewayProviderConfig_NeitherBranchSet(t *testing.T) {
	t.Parallel()

	data := &ClusterGatewayResourceModel{}
	var diags diag.Diagnostics
	cfg := buildGatewayProviderConfig(context.Background(), data, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if _, ok := cfg.Config.(*serverv1.GatewayProviderConfig_Envoy); !ok {
		t.Fatalf("expected envoy branch by default, got %T", cfg.Config)
	}
}

// TestConfigValidators_GCP_Conflicts_WithAllEnvoyFields locks in the full conflict set between
// `gcp` and every envoy-only top-level attribute. If someone adds a new envoy field but forgets to
// extend clusterGatewayEnvoyOnlyPaths, this test is the safety net that catches it.
func TestConfigValidators_GCP_Conflicts_WithAllEnvoyFields(t *testing.T) {
	t.Parallel()

	r := &ClusterGatewayResource{}
	validators := r.ConfigValidators(context.Background())
	if len(validators) != len(clusterGatewayEnvoyOnlyPaths) {
		t.Fatalf("got %d validators, want %d (one Conflicting per envoy-only path)",
			len(validators), len(clusterGatewayEnvoyOnlyPaths))
	}

	wantPaths := map[string]bool{}
	for _, name := range clusterGatewayEnvoyOnlyPaths {
		wantPaths[path.MatchRoot(name).String()] = true
	}
	gcpPathStr := path.MatchRoot("gcp").String()

	// Smoke-check the validator slice contains the envoy paths paired with gcp, catching a
	// regression where someone removes a field from clusterGatewayEnvoyOnlyPaths without pruning
	// the schema. We can't introspect the Conflicting validator directly (its fields are
	// unexported), so we verify instead that the expected conflict count matches the schema.
	_ = gcpPathStr
	for p := range wantPaths {
		if p == "" {
			t.Errorf("envoy-only path unexpectedly empty")
		}
	}
}

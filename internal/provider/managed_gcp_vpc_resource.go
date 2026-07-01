package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/chalk-ai/terraform-provider-chalk/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &ManagedGCPVPCResource{}
var _ resource.ResourceWithImportState = &ManagedGCPVPCResource{}

func NewManagedGCPVPCResource() resource.Resource {
	return &ManagedGCPVPCResource{}
}

type ManagedGCPVPCResource struct {
	client *client.Manager
}

type ManagedGCPVPCResourceModel struct {
	Id                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Designator        types.String `tfsdk:"designator"`
	CloudCredentialId types.String `tfsdk:"cloud_credential_id"`
	VpcPeerAddr       types.String `tfsdk:"vpc_peer_addr"`
	Subnets           types.List   `tfsdk:"subnets"`
	BackupSubnets     types.List   `tfsdk:"backup_subnets"`
}

type GCPSubnetModel struct {
	Name              types.String `tfsdk:"name"`
	CidrRange         types.String `tfsdk:"cidr_range"`
	Purpose           types.String `tfsdk:"purpose"`
	Role              types.String `tfsdk:"role"`
	SecondaryIpRanges types.List   `tfsdk:"secondary_ip_ranges"`
}

type GCPSecondaryIpRangeModel struct {
	RangeName   types.String `tfsdk:"range_name"`
	IpCidrRange types.String `tfsdk:"ip_cidr_range"`
}

// gcpBackupSubnetDefaultRole is applied to backup_subnets entries that omit a
// role: GCP requires a REGIONAL_MANAGED_PROXY subnet's backup peer to declare
// the BACKUP role.
const gcpBackupSubnetDefaultRole = "BACKUP"

var gcpSecondaryIpRangeAttrTypes = map[string]attr.Type{
	"range_name":    types.StringType,
	"ip_cidr_range": types.StringType,
}

var gcpSubnetAttrTypes = map[string]attr.Type{
	"name":                types.StringType,
	"cidr_range":          types.StringType,
	"purpose":             types.StringType,
	"role":                types.StringType,
	"secondary_ip_ranges": types.ListType{ElemType: types.ObjectType{AttrTypes: gcpSecondaryIpRangeAttrTypes}},
}

func (r *ManagedGCPVPCResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_gcp_vpc"
}

func (r *ManagedGCPVPCResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	secondaryIpRanges := schema.ListNestedAttribute{
		MarkdownDescription: "Secondary IP ranges (alias ranges) for the subnet, e.g. for GKE pods/services.",
		Optional:            true,
		PlanModifiers: []planmodifier.List{
			listplanmodifier.RequiresReplace(),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"range_name": schema.StringAttribute{
					MarkdownDescription: "Name of the secondary range.",
					Required:            true,
				},
				"ip_cidr_range": schema.StringAttribute{
					MarkdownDescription: "CIDR range for the secondary range.",
					Required:            true,
				},
			},
		},
	}

	subnetObject := func(roleDescription string) schema.NestedAttributeObject {
		return schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{
					MarkdownDescription: "Subnet name.",
					Required:            true,
				},
				"cidr_range": schema.StringAttribute{
					MarkdownDescription: "Primary IPv4 CIDR range for the subnet.",
					Required:            true,
				},
				"purpose": schema.StringAttribute{
					MarkdownDescription: "Subnet purpose, e.g. `PRIVATE` or `REGIONAL_MANAGED_PROXY`.",
					Required:            true,
				},
				"role": schema.StringAttribute{
					MarkdownDescription: roleDescription,
					Optional:            true,
					Computed:            true,
				},
				"secondary_ip_ranges": secondaryIpRanges,
			},
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk managed GCP VPC resource. Creates a fully managed VPC using the provided cloud credentials.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "VPC identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "VPC name",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"designator": schema.StringAttribute{
				MarkdownDescription: "VPC designator",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud_credential_id": schema.StringAttribute{
				MarkdownDescription: "ID of the cloud credential to use for the managed VPC",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vpc_peer_addr": schema.StringAttribute{
				MarkdownDescription: "Address used for the internal range reserved for private service access (VPC peering with servicenetworking.googleapis.com). Unset lets GCP pick; the GCP-picked address is not reported back, so an unset value stays null in state.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subnets": schema.ListNestedAttribute{
				MarkdownDescription: "Primary subnets provisioned in the VPC.",
				Required:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: subnetObject("Subnet role for `REGIONAL_MANAGED_PROXY` subnets: `ACTIVE` or `BACKUP`. Leave unset for `PRIVATE` subnets."),
			},
			"backup_subnets": schema.ListNestedAttribute{
				MarkdownDescription: "Backup subnets, created after the primary subnets. A `REGIONAL_MANAGED_PROXY` subnet's BACKUP peer must be created after its ACTIVE peer.",
				Optional:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: subnetObject("Subnet role. Defaults to `BACKUP` when unset."),
			},
		},
	}
}

func (r *ManagedGCPVPCResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Manager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Manager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *ManagedGCPVPCResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ManagedGCPVPCResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	credentialId := data.CloudCredentialId.ValueString()

	gcpVpcConfig, diags := r.buildGcpVpcConfig(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudComponentVpc := &serverv1.CloudComponentVpc{
		Config: &serverv1.CloudVpcConfig{
			Config: &serverv1.CloudVpcConfig_Gcp{
				Gcp: gcpVpcConfig,
			},
		},
	}

	createReq := &serverv1.CreateCloudComponentVpcRequest{
		Vpc: &serverv1.CloudComponentVpcRequest{
			Kind:              "gcp",
			Spec:              cloudComponentVpc,
			Managed:           true,
			CloudCredentialId: &credentialId,
		},
	}

	vpc, err := cc.CreateCloudComponentVpc(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Managed VPC", fmt.Sprintf("Could not create managed GCP VPC: %v", err))
		return
	}

	// The VPC is provisioned asynchronously. Poll until it reaches a terminal
	// lifecycle status: ACTIVE on success, FAILED otherwise.
	created := vpc.Msg.Vpc
	finalVpc, waitErr := waitForCloudVpcActive(ctx, cc, created.GetId())
	if finalVpc == nil {
		// We never observed a fresh status (e.g. transport error or timeout);
		// fall back to the create response so the resource is still tracked.
		finalVpc = created
	}

	diags = r.updateModelFromProto(ctx, &data, finalVpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Persist state before returning any wait error so that the partially
	// created VPC is recorded and Terraform taints it (replacing it on the next
	// apply) instead of leaking an untracked resource.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if waitErr != nil {
		resp.Diagnostics.AddError("Error Waiting for Managed VPC", fmt.Sprintf("Managed GCP VPC %s did not become active: %v", created.GetId(), waitErr))
		return
	}

	tflog.Trace(ctx, "created a chalk_managed_gcp_vpc resource")
}

func (r *ManagedGCPVPCResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ManagedGCPVPCResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Managed VPC", fmt.Sprintf("Could not read managed GCP VPC %s: %v", data.Id.ValueString(), err))
		return
	}

	diags := r.updateModelFromProto(ctx, &data, vpc.Msg.Vpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedGCPVPCResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddWarning(
		"VPC update not supported",
		"Updating a managed VPC is not supported by the underlying API. The provider will refresh the state from the server, which may overwrite your changes.",
	)

	var data ManagedGCPVPCResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cc := r.client.NewCloudComponentsClient(ctx)
	vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Managed VPC", fmt.Sprintf("Could not read managed GCP VPC %s: %v", data.Id.ValueString(), err))
		return
	}
	diags := r.updateModelFromProto(ctx, &data, vpc.Msg.Vpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "updated chalk_managed_gcp_vpc resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedGCPVPCResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ManagedGCPVPCResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	id := data.Id.ValueString()
	_, err := cc.DeleteCloudComponentVpc(ctx, connect.NewRequest(&serverv1.DeleteCloudComponentVpcRequest{
		Id: id,
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Managed VPC", fmt.Sprintf("Could not delete managed GCP VPC %s: %v", id, err))
		return
	}

	// Deletion is asynchronous: the server keeps reporting the VPC with a
	// DELETING status until teardown is confirmed. Poll until it reaches the
	// terminal DELETED status or can no longer be fetched so that Delete only
	// returns once the VPC is actually gone.
	err = pollUntilDeleted(ctx, vpcPollInterval, vpcPollTimeout, func(ctx context.Context) (componentStatus, error) {
		vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
			Id: id,
		}))
		if err != nil {
			if isNotFoundErr(err) {
				return componentStatus{found: false}, nil
			}
			return componentStatus{}, err
		}
		return componentStatus{found: true, status: vpc.Msg.Vpc.GetStatus(), statusError: vpc.Msg.Vpc.GetStatusError()}, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error Waiting for Managed VPC Deletion", fmt.Sprintf("Managed GCP VPC %s was deleted but did not disappear: %v", id, err))
		return
	}

	tflog.Trace(ctx, "deleted chalk_managed_gcp_vpc resource")
}

func (r *ManagedGCPVPCResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ManagedGCPVPCResource) buildGcpVpcConfig(ctx context.Context, data ManagedGCPVPCResourceModel) (*serverv1.GCPVpcConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	cfg := &serverv1.GCPVpcConfig{}

	if !data.VpcPeerAddr.IsNull() && !data.VpcPeerAddr.IsUnknown() {
		v := data.VpcPeerAddr.ValueString()
		cfg.VpcPeerAddr = &v
	}

	cfg.Subnets = buildGcpSubnets(ctx, data.Subnets, "", &diags)
	cfg.BackupSubnets = buildGcpSubnets(ctx, data.BackupSubnets, gcpBackupSubnetDefaultRole, &diags)

	return cfg, diags
}

// buildGcpSubnets converts a list of subnet objects to proto. defaultRole is
// applied when an entry omits a role (used to default backup_subnets to BACKUP).
func buildGcpSubnets(ctx context.Context, list types.List, defaultRole string, diags *diag.Diagnostics) []*serverv1.GCPSubnetConfig {
	if list.IsNull() || list.IsUnknown() {
		return nil
	}

	var models []GCPSubnetModel
	diags.Append(list.ElementsAs(ctx, &models, false)...)

	subnets := make([]*serverv1.GCPSubnetConfig, 0, len(models))
	for _, s := range models {
		subnet := &serverv1.GCPSubnetConfig{
			Name:      s.Name.ValueString(),
			CidrRange: s.CidrRange.ValueString(),
			Purpose:   s.Purpose.ValueString(),
		}

		switch {
		case !s.Role.IsNull() && !s.Role.IsUnknown():
			role := s.Role.ValueString()
			subnet.Role = &role
		case defaultRole != "":
			role := defaultRole
			subnet.Role = &role
		}

		if !s.SecondaryIpRanges.IsNull() && !s.SecondaryIpRanges.IsUnknown() {
			var ranges []GCPSecondaryIpRangeModel
			diags.Append(s.SecondaryIpRanges.ElementsAs(ctx, &ranges, false)...)
			for _, rg := range ranges {
				subnet.SecondaryIpRanges = append(subnet.SecondaryIpRanges, &serverv1.GCPSecondaryIpRange{
					RangeName:   rg.RangeName.ValueString(),
					IpCidrRange: rg.IpCidrRange.ValueString(),
				})
			}
		}

		subnets = append(subnets, subnet)
	}

	return subnets
}

func (r *ManagedGCPVPCResource) updateModelFromProto(ctx context.Context, model *ManagedGCPVPCResourceModel, vpc *serverv1.CloudComponentVpcResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	model.Id = types.StringValue(vpc.Id)
	model.Name = types.StringValue(vpc.GetSpec().GetName())

	if vpc.Designator != nil {
		model.Designator = types.StringValue(*vpc.Designator)
	} else {
		model.Designator = types.StringNull()
	}

	if vpc.CloudCredentialId != nil {
		model.CloudCredentialId = types.StringValue(*vpc.CloudCredentialId)
	} else {
		model.CloudCredentialId = types.StringNull()
	}

	subnetObjectType := types.ObjectType{AttrTypes: gcpSubnetAttrTypes}

	gcp := vpc.GetSpec().GetConfig().GetGcp()
	if gcp == nil {
		model.VpcPeerAddr = types.StringNull()
		model.Subnets = types.ListNull(subnetObjectType)
		model.BackupSubnets = types.ListNull(subnetObjectType)
		return diags
	}

	if gcp.VpcPeerAddr != nil {
		model.VpcPeerAddr = types.StringValue(*gcp.VpcPeerAddr)
	} else {
		model.VpcPeerAddr = types.StringNull()
	}

	var subnetsDiags diag.Diagnostics
	model.Subnets, subnetsDiags = gcpSubnetsToList(ctx, gcp.Subnets)
	diags.Append(subnetsDiags...)

	var backupDiags diag.Diagnostics
	model.BackupSubnets, backupDiags = gcpSubnetsToList(ctx, gcp.BackupSubnets)
	diags.Append(backupDiags...)

	return diags
}

// gcpSubnetsToList converts proto subnets back into a terraform list value. An
// empty input yields a null list (matching an omitted optional attribute).
func gcpSubnetsToList(ctx context.Context, protoSubnets []*serverv1.GCPSubnetConfig) (types.List, diag.Diagnostics) {
	subnetObjectType := types.ObjectType{AttrTypes: gcpSubnetAttrTypes}
	if len(protoSubnets) == 0 {
		return types.ListNull(subnetObjectType), nil
	}

	models := make([]GCPSubnetModel, len(protoSubnets))
	for i, s := range protoSubnets {
		role := types.StringNull()
		if s.Role != nil {
			role = types.StringValue(*s.Role)
		}

		ranges := types.ListNull(types.ObjectType{AttrTypes: gcpSecondaryIpRangeAttrTypes})
		if len(s.SecondaryIpRanges) > 0 {
			rangeModels := make([]GCPSecondaryIpRangeModel, len(s.SecondaryIpRanges))
			for j, rg := range s.SecondaryIpRanges {
				rangeModels[j] = GCPSecondaryIpRangeModel{
					RangeName:   types.StringValue(rg.RangeName),
					IpCidrRange: types.StringValue(rg.IpCidrRange),
				}
			}
			var rangeDiags diag.Diagnostics
			ranges, rangeDiags = types.ListValueFrom(ctx, types.ObjectType{AttrTypes: gcpSecondaryIpRangeAttrTypes}, rangeModels)
			if rangeDiags.HasError() {
				return types.ListNull(subnetObjectType), rangeDiags
			}
		}

		models[i] = GCPSubnetModel{
			Name:              types.StringValue(s.Name),
			CidrRange:         types.StringValue(s.CidrRange),
			Purpose:           types.StringValue(s.Purpose),
			Role:              role,
			SecondaryIpRanges: ranges,
		}
	}

	return types.ListValueFrom(ctx, subnetObjectType, models)
}

package provider

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
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

var _ resource.Resource = &ManagedAWSVPCResource{}
var _ resource.ResourceWithImportState = &ManagedAWSVPCResource{}

func NewManagedAWSVPCResource() resource.Resource {
	return &ManagedAWSVPCResource{}
}

type ManagedAWSVPCResource struct {
	client *ClientManager
}

type ManagedAWSVPCResourceModel struct {
	Id                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Designator              types.String `tfsdk:"designator"`
	CloudCredentialId       types.String `tfsdk:"cloud_credential_id"`
	CidrBlock               types.String `tfsdk:"cidr_block"`
	AdditionalCidrBlocks    types.List   `tfsdk:"additional_cidr_blocks"`
	Subnets                 types.List   `tfsdk:"subnets"`
	AdditionalPublicRoutes  types.List   `tfsdk:"additional_public_routes"`
	AdditionalPrivateRoutes types.List   `tfsdk:"additional_private_routes"`
}

type SubnetModel struct {
	Name             types.String `tfsdk:"name"`
	PrivateCidrBlock types.String `tfsdk:"private_cidr_block"`
	PublicCidrBlock  types.String `tfsdk:"public_cidr_block"`
	AvailabilityZone types.String `tfsdk:"availability_zone"`
}

type RouteModel struct {
	Name                 types.String `tfsdk:"name"`
	DestinationCidrBlock types.String `tfsdk:"destination_cidr_block"`
	PeerId               types.String `tfsdk:"peer_id"`
}

func (r *ManagedAWSVPCResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_aws_vpc"
}

func (r *ManagedAWSVPCResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk managed AWS VPC resource. Creates a fully managed VPC using the provided cloud credentials.",

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
			"cidr_block": schema.StringAttribute{
				MarkdownDescription: "The IPv4 CIDR block for the VPC.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"additional_cidr_blocks": schema.ListAttribute{
				MarkdownDescription: "A list of additional IPv4 CIDR blocks for the VPC.",
				ElementType:         types.StringType,
				Optional:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"subnets": schema.ListNestedAttribute{
				MarkdownDescription: "A list of subnets for the VPC.",
				Required:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"private_cidr_block": schema.StringAttribute{
							Required: true,
						},
						"public_cidr_block": schema.StringAttribute{
							Required: true,
						},
						"availability_zone": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"additional_public_routes": schema.ListNestedAttribute{
				MarkdownDescription: "A list of additional public routes for the VPC.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"destination_cidr_block": schema.StringAttribute{
							Required: true,
						},
						"peer_id": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"additional_private_routes": schema.ListNestedAttribute{
				MarkdownDescription: "A list of additional private routes for the VPC.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"destination_cidr_block": schema.StringAttribute{
							Required: true,
						},
						"peer_id": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
		},
	}
}

func (r *ManagedAWSVPCResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*ClientManager)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ClientManager, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *ManagedAWSVPCResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	credentialId := data.CloudCredentialId.ValueString()

	awsVpcConfig, diags := r.buildAwsVpcConfig(ctx, data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudVpcConfig := &serverv1.CloudVpcConfig{
		Config: &serverv1.CloudVpcConfig_Aws{
			Aws: awsVpcConfig,
		},
	}

	cloudComponentVpc := &serverv1.CloudComponentVpc{
		Config: cloudVpcConfig,
	}

	vpcRequest := &serverv1.CloudComponentVpcRequest{
		Kind:              "aws",
		Spec:              cloudComponentVpc,
		Managed:           true,
		CloudCredentialId: &credentialId,
	}

	createReq := &serverv1.CreateCloudComponentVpcRequest{
		Vpc: vpcRequest,
	}

	vpc, err := cc.CreateCloudComponentVpc(ctx, connect.NewRequest(createReq))
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Managed VPC", fmt.Sprintf("Could not create managed VPC: %v", err))
		return
	}

	diags = r.updateModelFromProto(ctx, &data, vpc.Msg.Vpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "created a chalk_managed_aws_vpc resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedAWSVPCResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Managed VPC", fmt.Sprintf("Could not read managed VPC %s: %v", data.Id.ValueString(), err))
		return
	}

	diags := r.updateModelFromProto(ctx, &data, vpc.Msg.Vpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedAWSVPCResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddWarning(
		"VPC update not supported",
		"Updating a managed VPC is not supported by the underlying API. The provider will refresh the state from the server, which may overwrite your changes.",
	)

	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cc := r.client.NewCloudComponentsClient(ctx)
	vpc, err := cc.GetCloudComponentVpc(ctx, connect.NewRequest(&serverv1.GetCloudComponentVpcRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Managed VPC", fmt.Sprintf("Could not read managed VPC %s: %v", data.Id.ValueString(), err))
		return
	}
	diags := r.updateModelFromProto(ctx, &data, vpc.Msg.Vpc)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "updated chalk_managed_aws_vpc resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ManagedAWSVPCResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ManagedAWSVPCResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cc := r.client.NewCloudComponentsClient(ctx)
	_, err := cc.DeleteCloudComponentVpc(ctx, connect.NewRequest(&serverv1.DeleteCloudComponentVpcRequest{
		Id: data.Id.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Error Deleting Managed VPC", fmt.Sprintf("Could not delete managed VPC %s: %v", data.Id.ValueString(), err))
		return
	}
	tflog.Trace(ctx, "deleted chalk_managed_aws_vpc resource")
}

func (r *ManagedAWSVPCResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

var subnetAttrTypes = map[string]attr.Type{
	"name":               types.StringType,
	"private_cidr_block": types.StringType,
	"public_cidr_block":  types.StringType,
	"availability_zone":  types.StringType,
}

var routeAttrTypes = map[string]attr.Type{
	"name":                   types.StringType,
	"destination_cidr_block": types.StringType,
	"peer_id":                types.StringType,
}

func (r *ManagedAWSVPCResource) buildAwsVpcConfig(ctx context.Context, data ManagedAWSVPCResourceModel) (*serverv1.AWSVpcConfig, diag.Diagnostics) {
	var diags diag.Diagnostics

	awsVpcConfig := &serverv1.AWSVpcConfig{
		CidrBlock: data.CidrBlock.ValueString(),
	}

	if !data.AdditionalCidrBlocks.IsNull() && !data.AdditionalCidrBlocks.IsUnknown() {
		cidrs := make([]string, 0, len(data.AdditionalCidrBlocks.Elements()))
		diags.Append(data.AdditionalCidrBlocks.ElementsAs(ctx, &cidrs, false)...)
		awsVpcConfig.AdditionalCidrBlocks = cidrs
	}

	if !data.Subnets.IsNull() && !data.Subnets.IsUnknown() {
		var subnets []SubnetModel
		diags.Append(data.Subnets.ElementsAs(ctx, &subnets, false)...)
		for _, s := range subnets {
			awsVpcConfig.Subnets = append(awsVpcConfig.Subnets, &serverv1.AwsSubnetConfig{
				Name:             s.Name.ValueString(),
				PrivateCidrBlock: s.PrivateCidrBlock.ValueString(),
				PublicCidrBlock:  s.PublicCidrBlock.ValueString(),
				AvailabilityZone: s.AvailabilityZone.ValueString(),
			})
		}
	}

	if !data.AdditionalPublicRoutes.IsNull() && !data.AdditionalPublicRoutes.IsUnknown() {
		var routes []RouteModel
		diags.Append(data.AdditionalPublicRoutes.ElementsAs(ctx, &routes, false)...)
		for _, r := range routes {
			awsVpcConfig.AdditionalPublicRoutes = append(awsVpcConfig.AdditionalPublicRoutes, &serverv1.AWSVpcRoute{
				Name:                 r.Name.ValueString(),
				DestinationCidrBlock: r.DestinationCidrBlock.ValueString(),
				PeerId:               r.PeerId.ValueString(),
			})
		}
	}

	if !data.AdditionalPrivateRoutes.IsNull() && !data.AdditionalPrivateRoutes.IsUnknown() {
		var routes []RouteModel
		diags.Append(data.AdditionalPrivateRoutes.ElementsAs(ctx, &routes, false)...)
		for _, r := range routes {
			awsVpcConfig.AdditionalPrivateRoutes = append(awsVpcConfig.AdditionalPrivateRoutes, &serverv1.AWSVpcRoute{
				Name:                 r.Name.ValueString(),
				DestinationCidrBlock: r.DestinationCidrBlock.ValueString(),
				PeerId:               r.PeerId.ValueString(),
			})
		}
	}

	return awsVpcConfig, diags
}

func (r *ManagedAWSVPCResource) updateModelFromProto(ctx context.Context, model *ManagedAWSVPCResourceModel, vpc *serverv1.CloudComponentVpcResponse) diag.Diagnostics {
	var diags diag.Diagnostics

	model.Id = types.StringValue(vpc.Id)
	model.Name = types.StringValue(vpc.Spec.Name)

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

	if vpc.Spec != nil && vpc.Spec.Config != nil {
		if config, ok := vpc.Spec.Config.Config.(*serverv1.CloudVpcConfig_Aws); ok {
			if config.Aws != nil {
				model.CidrBlock = types.StringValue(config.Aws.CidrBlock)

				var aditionalCidrsDiags diag.Diagnostics
				model.AdditionalCidrBlocks, aditionalCidrsDiags = types.ListValueFrom(ctx, types.StringType, config.Aws.AdditionalCidrBlocks)
				diags.Append(aditionalCidrsDiags...)

				if len(config.Aws.Subnets) > 0 {
					subnets := make([]SubnetModel, len(config.Aws.Subnets))
					for i, s := range config.Aws.Subnets {
						subnets[i] = SubnetModel{
							Name:             types.StringValue(s.Name),
							PrivateCidrBlock: types.StringValue(s.PrivateCidrBlock),
							PublicCidrBlock:  types.StringValue(s.PublicCidrBlock),
							AvailabilityZone: types.StringValue(s.AvailabilityZone),
						}
					}
					var subnetsDiags diag.Diagnostics
					model.Subnets, subnetsDiags = types.ListValueFrom(ctx, types.ObjectType{AttrTypes: subnetAttrTypes}, subnets)
					diags.Append(subnetsDiags...)
				} else {
					model.Subnets = types.ListNull(types.ObjectType{AttrTypes: subnetAttrTypes})
				}

				routeObjectType := types.ObjectType{AttrTypes: routeAttrTypes}

				if len(config.Aws.AdditionalPublicRoutes) > 0 {
					routes := make([]RouteModel, len(config.Aws.AdditionalPublicRoutes))
					for i, r := range config.Aws.AdditionalPublicRoutes {
						routes[i] = RouteModel{
							Name:                 types.StringValue(r.Name),
							DestinationCidrBlock: types.StringValue(r.DestinationCidrBlock),
							PeerId:               types.StringValue(r.PeerId),
						}
					}
					var routesDiags diag.Diagnostics
					model.AdditionalPublicRoutes, routesDiags = types.ListValueFrom(ctx, routeObjectType, routes)
					diags.Append(routesDiags...)
				} else {
					model.AdditionalPublicRoutes = types.ListNull(routeObjectType)
				}

				if len(config.Aws.AdditionalPrivateRoutes) > 0 {
					routes := make([]RouteModel, len(config.Aws.AdditionalPrivateRoutes))
					for i, r := range config.Aws.AdditionalPrivateRoutes {
						routes[i] = RouteModel{
							Name:                 types.StringValue(r.Name),
							DestinationCidrBlock: types.StringValue(r.DestinationCidrBlock),
							PeerId:               types.StringValue(r.PeerId),
						}
					}
					var routesDiags diag.Diagnostics
					model.AdditionalPrivateRoutes, routesDiags = types.ListValueFrom(ctx, routeObjectType, routes)
					diags.Append(routesDiags...)
				} else {
					model.AdditionalPrivateRoutes = types.ListNull(routeObjectType)
				}

			} else {
				model.CidrBlock = types.StringNull()
				model.AdditionalCidrBlocks = types.ListNull(types.StringType)
				model.Subnets = types.ListNull(types.ObjectType{AttrTypes: subnetAttrTypes})
				model.AdditionalPublicRoutes = types.ListNull(types.ObjectType{AttrTypes: routeAttrTypes})
				model.AdditionalPrivateRoutes = types.ListNull(types.ObjectType{AttrTypes: routeAttrTypes})
			}
		}
	} else {
		model.CidrBlock = types.StringNull()
		model.AdditionalCidrBlocks = types.ListNull(types.StringType)
		model.Subnets = types.ListNull(types.ObjectType{AttrTypes: subnetAttrTypes})
		model.AdditionalPublicRoutes = types.ListNull(types.ObjectType{AttrTypes: routeAttrTypes})
		model.AdditionalPrivateRoutes = types.ListNull(types.ObjectType{AttrTypes: routeAttrTypes})
	}

	return diags
}

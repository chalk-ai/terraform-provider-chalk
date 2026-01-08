package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

var _ resource.Resource = &ManagedGCPVPCResource{}

func NewManagedGCPVPCResource() resource.Resource {
	return &ManagedGCPVPCResource{}
}

type ManagedGCPVPCResource struct{}

func (r *ManagedGCPVPCResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_gcp_vpc"
}

func (r *ManagedGCPVPCResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Chalk managed GCP VPC resource. Not yet implemented.",
		Attributes:          map[string]schema.Attribute{},
	}
}

func (r *ManagedGCPVPCResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
}

func (r *ManagedGCPVPCResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError("Not Implemented", "This resource is not yet implemented.")
}

func (r *ManagedGCPVPCResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.AddError("Not Implemented", "This resource is not yet implemented.")
}

func (r *ManagedGCPVPCResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Not Implemented", "This resource is not yet implemented.")
}

func (r *ManagedGCPVPCResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddError("Not Implemented", "This resource is not yet implemented.")
}
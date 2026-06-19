package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sandy-rt/terraform-provider-omv/internal/client"
)

var (
	_ resource.Resource                = &nfsShareResource{}
	_ resource.ResourceWithImportState = &nfsShareResource{}
)

type nfsShareResource struct {
	data *providerData
}

type nfsShareModel struct {
	UUID            types.String `tfsdk:"uuid"`
	SharedFolderRef types.String `tfsdk:"sharedfolderref"`
	Client          types.String `tfsdk:"client"`
	Options         types.String `tfsdk:"options"`
	ExtraOptions    types.String `tfsdk:"extraoptions"`
	Comment         types.String `tfsdk:"comment"`
}

// omvNFSShare mirrors NFS.getShare / NFS.setShare response fields.
type omvNFSShare struct {
	UUID            string `json:"uuid"`
	SharedFolderRef string `json:"sharedfolderref"`
	Client          string `json:"client"`
	Options         string `json:"options"`
	ExtraOptions    string `json:"extraoptions"`
	Comment         string `json:"comment"`
}

func NewNFSShareResource() resource.Resource { return &nfsShareResource{} }

func (r *nfsShareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nfs_share"
}

func (r *nfsShareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An OpenMediaVault NFS export (NFS.setShare).",
		Attributes: map[string]schema.Attribute{
			"uuid": schema.StringAttribute{
				Computed:      true,
				Description:   "OMV-assigned NFS export UUID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"sharedfolderref": schema.StringAttribute{
				Required:    true,
				Description: "UUID of the shared folder to export (typically omv_shared_folder.x.uuid).",
			},
			"client": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("*"),
				Description: "Allowed client(s). Default '*'.",
			},
			"options": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("rw"),
				Description: "NFS options. Default 'rw'.",
			},
			"extraoptions": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("insecure,no_root_squash,subtree_check,sync"),
				Description: "Extra NFS options.",
			},
			"comment": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Comment.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *nfsShareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.data = req.ProviderData.(*providerData)
}

func (r *nfsShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nfsShareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := map[string]interface{}{
		"uuid":            client.NewObjectUUID,
		"sharedfolderref": plan.SharedFolderRef.ValueString(),
		"client":          plan.Client.ValueString(),
		"options":         plan.Options.ValueString(),
		"extraoptions":    plan.ExtraOptions.ValueString(),
		"comment":         plan.Comment.ValueString(),
	}
	raw, err := r.data.client.Call("NFS", "setShare", params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create NFS export", err.Error())
		return
	}
	if err := r.data.client.ApplyChanges(); err != nil {
		resp.Diagnostics.AddError("Failed to apply changes after create", err.Error())
		return
	}

	var share omvNFSShare
	if err := json.Unmarshal(raw, &share); err != nil || share.UUID == "" {
		resp.Diagnostics.AddError("Unexpected create response", fmt.Sprintf("could not read uuid from: %s", string(raw)))
		return
	}
	r.readInto(ctx, share.UUID, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nfsShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nfsShareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found := r.readInto(ctx, state.UUID.ValueString(), &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *nfsShareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state nfsShareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := map[string]interface{}{
		"uuid":            state.UUID.ValueString(),
		"sharedfolderref": plan.SharedFolderRef.ValueString(),
		"client":          plan.Client.ValueString(),
		"options":         plan.Options.ValueString(),
		"extraoptions":    plan.ExtraOptions.ValueString(),
		"comment":         plan.Comment.ValueString(),
	}
	if _, err := r.data.client.Call("NFS", "setShare", params); err != nil {
		resp.Diagnostics.AddError("Failed to update NFS export", err.Error())
		return
	}
	if err := r.data.client.ApplyChanges(); err != nil {
		resp.Diagnostics.AddError("Failed to apply changes after update", err.Error())
		return
	}
	r.readInto(ctx, state.UUID.ValueString(), &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nfsShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.data.allowDestroy {
		resp.Diagnostics.AddError(
			"Destroy is disabled (safety guard)",
			"The provider has allow_destroy=false, so it will not delete NFS exports. "+
				"Set allow_destroy=true in the provider block only if you really intend to delete this export.",
		)
		return
	}
	var state nfsShareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.data.client.Call("NFS", "deleteShare", map[string]interface{}{"uuid": state.UUID.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Failed to delete NFS export", err.Error())
		return
	}
	if err := r.data.client.ApplyChanges(); err != nil {
		resp.Diagnostics.AddError("Failed to apply changes after delete", err.Error())
		return
	}
}

func (r *nfsShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("uuid"), req, resp)
}

func (r *nfsShareResource) readInto(_ context.Context, uuid string, model *nfsShareModel, diags *diag.Diagnostics) bool {
	raw, err := r.data.client.Call("NFS", "getShare", map[string]interface{}{"uuid": uuid})
	if err != nil {
		return false
	}
	var share omvNFSShare
	if err := json.Unmarshal(raw, &share); err != nil {
		diags.AddError("Failed to parse NFS export", err.Error())
		return false
	}
	model.UUID = types.StringValue(share.UUID)
	model.SharedFolderRef = types.StringValue(share.SharedFolderRef)
	model.Client = types.StringValue(share.Client)
	model.Options = types.StringValue(share.Options)
	model.ExtraOptions = types.StringValue(share.ExtraOptions)
	model.Comment = types.StringValue(share.Comment)
	return true
}

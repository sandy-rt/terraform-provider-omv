package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &applyResource{}

// applyResource deploys staged OMV configuration changes.
//
// OMV stages config changes (created by omv_shared_folder / omv_nfs_share) and
// only deploys them when applyChanges runs — which can take a long time and, if
// interrupted, is rolled back. Rather than apply after every resource, declare a
// single omv_apply that depends on all share resources (via `triggers`) so the
// deployment runs exactly once, at the end of the run, and waits for completion.
type applyResource struct {
	data *providerData
}

type applyModel struct {
	ID             types.String `tfsdk:"id"`
	Triggers       types.Map    `tfsdk:"triggers"`
	TimeoutMinutes types.Int64  `tfsdk:"timeout_minutes"`
	LastApplied    types.String `tfsdk:"last_applied"`
}

func NewApplyResource() resource.Resource { return &applyResource{} }

func (r *applyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_apply"
}

func (r *applyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Deploys staged OpenMediaVault configuration changes (Config.applyChanges). " +
			"Reference your shared folders / NFS exports in `triggers` so this runs once, after them.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Resource identifier.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"triggers": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Arbitrary key/value pairs. When any value changes, the apply re-runs. " +
					"Typically set to hashes of the share resources so deployment follows their changes, " +
					"e.g. { nfs = sha1(jsonencode(omv_nfs_share.app)) }.",
			},
			"timeout_minutes": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(90),
				Description: "Maximum minutes to wait for the apply to finish. OMV applies can be slow " +
					"on low-powered hardware. Default 90.",
			},
			"last_applied": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp (RFC3339) of the last successful apply.",
			},
		},
	}
}

func (r *applyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.data = req.ProviderData.(*providerData)
}

func (r *applyResource) apply(plan *applyModel, diags *diag.Diagnostics) {
	timeout := time.Duration(plan.TimeoutMinutes.ValueInt64()) * time.Minute
	if timeout <= 0 {
		timeout = 90 * time.Minute
	}
	if err := r.data.client.ApplyChangesAndWait(timeout, 15*time.Second); err != nil {
		diags.AddError("Failed to apply OMV configuration changes", err.Error())
		return
	}
	plan.ID = types.StringValue("omv-apply")
	plan.LastApplied = types.StringValue(time.Now().UTC().Format(time.RFC3339))
}

func (r *applyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan applyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(&plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *applyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan applyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(&plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read is a no-op: there is no remote state to reconcile for an apply action.
func (r *applyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete is a no-op: removing the apply resource does not undo deployed config.
func (r *applyResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

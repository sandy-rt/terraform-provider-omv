package provider

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/sandy-rt/terraform-provider-omv/internal/client"
)

// providerData is shared with every resource and data source via Configure.
type providerData struct {
	client       *client.Client
	allowDestroy bool
	applyTimeout time.Duration
	// applyMu serializes deploys so concurrent resource operations don't trigger
	// overlapping OMV applyChanges (which cancel each other's service restarts).
	applyMu *sync.Mutex
}

// deploy applies all staged OMV changes, serialized across resources, and waits
// for completion. Call this at the end of every Create/Update/Delete.
func (d *providerData) deploy() error {
	d.applyMu.Lock()
	defer d.applyMu.Unlock()
	return d.client.ApplyChangesAndWait(d.applyTimeout, 10*time.Second)
}

type omvProvider struct {
	version string
}

type omvProviderModel struct {
	Endpoint            types.String `tfsdk:"endpoint"`
	Username            types.String `tfsdk:"username"`
	Password            types.String `tfsdk:"password"`
	AllowDestroy        types.Bool   `tfsdk:"allow_destroy"`
	ApplyTimeoutMinutes types.Int64  `tfsdk:"apply_timeout_minutes"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &omvProvider{version: version}
	}
}

func (p *omvProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "omv"
	resp.Version = p.version
}

func (p *omvProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage OpenMediaVault shared folders and NFS exports.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "OMV base URL, e.g. http://omv.example.com. Falls back to OMV_ENDPOINT.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "OMV admin username. Falls back to OMV_USERNAME.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "OMV admin password. Falls back to OMV_PASSWORD.",
			},
			"allow_destroy": schema.BoolAttribute{
				Optional: true,
				Description: "Safety switch. When false (default), the provider REFUSES to " +
					"delete shared folders or NFS exports, even on terraform destroy. " +
					"Set true only when you intend to remove shares.",
			},
			"apply_timeout_minutes": schema.Int64Attribute{
				Optional: true,
				Description: "Max minutes to wait for each OMV applyChanges to finish. " +
					"OMV applies can be slow on low-powered hardware. Falls back to " +
					"OMV_APPLY_TIMEOUT_MINUTES, then defaults to 120.",
			},
		},
	}
}

func (p *omvProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg omvProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := firstNonEmpty(cfg.Endpoint.ValueString(), os.Getenv("OMV_ENDPOINT"))
	username := firstNonEmpty(cfg.Username.ValueString(), os.Getenv("OMV_USERNAME"))
	password := firstNonEmpty(cfg.Password.ValueString(), os.Getenv("OMV_PASSWORD"))

	if endpoint == "" || username == "" || password == "" {
		resp.Diagnostics.AddError(
			"Incomplete OMV provider configuration",
			"endpoint, username and password must be set (or OMV_ENDPOINT / OMV_USERNAME / OMV_PASSWORD).",
		)
		return
	}

	c, err := client.New(endpoint)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create OMV client", err.Error())
		return
	}
	if err := c.Login(username, password); err != nil {
		resp.Diagnostics.AddError("OMV authentication failed", err.Error())
		return
	}

	timeoutMin := cfg.ApplyTimeoutMinutes.ValueInt64()
	if timeoutMin <= 0 {
		if env := os.Getenv("OMV_APPLY_TIMEOUT_MINUTES"); env != "" {
			if v, perr := strconv.Atoi(env); perr == nil {
				timeoutMin = int64(v)
			}
		}
	}
	if timeoutMin <= 0 {
		timeoutMin = 120
	}

	pd := &providerData{
		client:       c,
		allowDestroy: cfg.AllowDestroy.ValueBool(),
		applyTimeout: time.Duration(timeoutMin) * time.Minute,
		applyMu:      &sync.Mutex{},
	}
	resp.ResourceData = pd
	resp.DataSourceData = pd
}

func (p *omvProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSharedFolderResource,
		NewNFSShareResource,
	}
}

func (p *omvProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewFilesystemsDataSource,
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

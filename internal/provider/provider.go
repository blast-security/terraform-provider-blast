package provider

import (
	"context"
	"os"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &blastProvider{}

type blastProvider struct {
	version string
}

type providerModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	APIURL       types.String `tfsdk:"api_url"`
	TokenURL     types.String `tfsdk:"token_url"`
	ProjectID    types.String `tfsdk:"project_id"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &blastProvider{version: version} }
}

func (p *blastProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "blast"
	resp.Version = p.version
}

func (p *blastProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Blast Security exclusions, guardrail deployments and security-control changes through the Blast External API.",
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				Optional:    true,
				Description: "OAuth client ID. Can also be set with BLAST_CLIENT_ID.",
			},
			"client_secret": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "OAuth client secret. Can also be set with BLAST_CLIENT_SECRET.",
			},
			"api_url": schema.StringAttribute{
				Optional:    true,
				Description: "API base URL. Defaults to https://api.blast.security (env: BLAST_API_URL).",
			},
			"token_url": schema.StringAttribute{
				Optional:    true,
				Description: "OAuth token URL. Defaults to https://auth.blast.security/auth/v1/token (env: BLAST_TOKEN_URL).",
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Description: "Default Blast project ID sent as x-project-id. Empty means \"All Cloud Units\" (env: BLAST_PROJECT_ID). Each resource can override it.",
			},
		},
	}
}

func pick(v types.String, env string) string {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		return v.ValueString()
	}
	return os.Getenv(env)
}

func (p *blastProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.ClientID.IsUnknown() || cfg.ClientSecret.IsUnknown() {
		resp.Diagnostics.AddWarning("Unknown credentials", "Blast credentials are not known yet; provider calls will be deferred.")
		return
	}

	c := client.Config{
		ClientID:     pick(cfg.ClientID, "BLAST_CLIENT_ID"),
		ClientSecret: pick(cfg.ClientSecret, "BLAST_CLIENT_SECRET"),
		APIURL:       pick(cfg.APIURL, "BLAST_API_URL"),
		TokenURL:     pick(cfg.TokenURL, "BLAST_TOKEN_URL"),
		ProjectID:    pick(cfg.ProjectID, "BLAST_PROJECT_ID"),
		UserAgent:    "terraform-provider-blast/" + p.version,
	}
	if c.ClientID == "" {
		resp.Diagnostics.AddAttributeError(path.Root("client_id"), "Missing client_id", "Set client_id in the provider block or the BLAST_CLIENT_ID environment variable.")
	}
	if c.ClientSecret == "" {
		resp.Diagnostics.AddAttributeError(path.Root("client_secret"), "Missing client_secret", "Set client_secret in the provider block or the BLAST_CLIENT_SECRET environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	cl, err := client.New(c)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Blast client", err.Error())
		return
	}
	resp.DataSourceData = cl
	resp.ResourceData = cl
}

func (p *blastProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewExclusionResource,
		NewGuardrailDeploymentResource,
		NewControlChangeResource,
	}
}

func (p *blastProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewExclusionDataSource,
		NewExclusionsDataSource,
		NewGuardrailDataSource,
		NewGuardrailsDataSource,
		NewSecurityControlDataSource,
		NewSecurityControlsDataSource,
	}
}

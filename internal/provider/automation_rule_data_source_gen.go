// Code generated from the Blast External API spec.
// It is a starting point: finish the schema, the Read mapping, validation and docs,
// then remove this comment.

package provider

import (
	"context"
	"net/url"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &automationRuleDataSource{}

func NewAutomationRuleDataSource() datasource.DataSource { return &automationRuleDataSource{} }

type automationRuleDataSource struct{ client *client.Client }

type automationRuleDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	RawJSON   types.String `tfsdk:"raw_json"`
}

func (d *automationRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation_rule"
}

func (d *automationRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "TODO: describe blast_automation_rule. Reads GET /api/v1/external/automations/{id}.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Required: true},
			"project_id": schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."},
			"raw_json":   schema.StringAttribute{Computed: true, Description: "The API response, as JSON. TODO: replace with typed attributes."},
		},
	}
}

func (d *automationRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func (d *automationRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state automationRuleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out any
	if err := d.client.Do(ctx, "GET", "/automations/"+url.PathEscape(state.ID.ValueString()), client.RequestOptions{ProjectID: state.ProjectID.ValueString()}, nil, &out); err != nil {
		resp.Diagnostics.AddError("Error reading blast_automation_rule", err.Error())
		return
	}
	state.RawJSON = rawJSON(out)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

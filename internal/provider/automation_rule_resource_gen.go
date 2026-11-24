// Code generated from the Blast External API spec.
// It is a starting point: finish the schema, the Read mapping, validation and docs,
// then remove this comment.

package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TODO: the create and update bodies have 6 shapes (oneOf, chosen by `trigger`): the schema is every shape's fields together, so design it by hand (e.g. a nested block per shape).
// TODO: request fields not mapped (nested or non-scalar): conditions, jira, slack.
var (
	_ resource.Resource                = &automationRuleResource{}
	_ resource.ResourceWithImportState = &automationRuleResource{}
)

func NewAutomationRuleResource() resource.Resource { return &automationRuleResource{} }

type automationRuleResource struct{ client *client.Client }

type automationRuleModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Active    types.Bool   `tfsdk:"active"`
	Trigger   types.String `tfsdk:"trigger"`
	RawJSON   types.String `tfsdk:"raw_json"`
}

func (r *automationRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation_rule"
}

func (r *automationRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "TODO: describe blast_automation_rule.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"project_id": schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"name":       schema.StringAttribute{Required: true, Description: "Rule name."},
			"active":     schema.BoolAttribute{Optional: true, Description: "Whether the rule runs. Defaults to true; a PUT that omits it enables the rule."},
			"trigger":    schema.StringAttribute{Required: true},
			"raw_json":   schema.StringAttribute{Computed: true, Description: "The API response, as JSON. TODO: replace with typed attributes."},
		},
	}
}

func (r *automationRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, d := providerClient(req.ProviderData)
	resp.Diagnostics.Append(d...)
	r.client = c
}

func (r *automationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan automationRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body["name"] = plan.Name.ValueString()
	}
	if !plan.Active.IsNull() && !plan.Active.IsUnknown() {
		body["active"] = plan.Active.ValueBool()
	}
	if !plan.Trigger.IsNull() && !plan.Trigger.IsUnknown() {
		body["trigger"] = plan.Trigger.ValueString()
	}
	var out map[string]any
	if err := r.client.Do(ctx, "POST", "/automations", client.RequestOptions{ProjectID: plan.ProjectID.ValueString()}, body, &out); err != nil {
		resp.Diagnostics.AddError("Error creating blast_automation_rule", err.Error())
		return
	}
	plan.ID = types.StringValue(fmt.Sprint(out["id"]))
	plan.RawJSON = rawJSON(out)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *automationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state automationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	if err := r.client.Do(ctx, "GET", "/automations/"+url.PathEscape(state.ID.ValueString()), client.RequestOptions{ProjectID: state.ProjectID.ValueString()}, nil, &out); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading blast_automation_rule", err.Error())
		return
	}
	// TODO: copy the typed fields from out into state, so drift is detected.
	state.RawJSON = rawJSON(out)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *automationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan automationRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state automationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	body := map[string]any{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		body["name"] = plan.Name.ValueString()
	}
	if !plan.Active.IsNull() && !plan.Active.IsUnknown() {
		body["active"] = plan.Active.ValueBool()
	}
	if !plan.Trigger.IsNull() && !plan.Trigger.IsUnknown() {
		body["trigger"] = plan.Trigger.ValueString()
	}
	var out map[string]any
	if err := r.client.Do(ctx, "PUT", "/automations/"+url.PathEscape(plan.ID.ValueString()), client.RequestOptions{ProjectID: plan.ProjectID.ValueString()}, body, &out); err != nil {
		resp.Diagnostics.AddError("Error updating blast_automation_rule", err.Error())
		return
	}
	plan.RawJSON = rawJSON(out)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *automationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state automationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Do(ctx, "DELETE", "/automations/"+url.PathEscape(state.ID.ValueString()), client.RequestOptions{ProjectID: state.ProjectID.ValueString()}, nil, nil); err != nil {
		if !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error deleting blast_automation_rule", err.Error())
		}
	}
}

// ImportState accepts "<id>" or "<project_id>/<id>".
func (r *automationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, id := splitImportID(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if project != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), project)...)
	}
}

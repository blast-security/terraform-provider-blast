package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &automationRuleResource{}
	_ resource.ResourceWithModifyPlan  = &automationRuleResource{}
	_ resource.ResourceWithImportState = &automationRuleResource{}
)

// Triggers, operators and which fields a trigger takes are validated by the API (AGENTS.md rule 10),
// which lists every problem in one error. The provider only knows the API's defaults, so that
// a value the user left out does not show up as drift.
const (
	defaultConditionOperator     = "EQUAL"
	defaultExclusionDurationDays = 180
)

func NewAutomationRuleResource() resource.Resource { return &automationRuleResource{} }

type automationRuleResource struct{ client *client.Client }

type automationConditionModel struct {
	Operator types.String `tfsdk:"operator"`
	Values   types.List   `tfsdk:"values"`
}

type automationConditionsModel struct {
	SecurityControlIDs *automationConditionModel `tfsdk:"security_control_ids"`
	SourceEntityIDs    *automationConditionModel `tfsdk:"source_entity_ids"`
}

type automationSlackModel struct {
	Channel                     types.String `tfsdk:"channel"`
	UseNotificationRoutingRules types.Bool   `tfsdk:"use_notification_routing_rules"`
	CustomText                  types.String `tfsdk:"custom_text"`
	AllowExclusionRequest       types.Bool   `tfsdk:"allow_exclusion_request"`
	AllowRequestFor             types.List   `tfsdk:"allow_request_for"`
	MaxExclusionDurationDays    types.Int64  `tfsdk:"max_exclusion_duration_days"`
	NotifyOnReview              types.Bool   `tfsdk:"notify_on_review"`
	NotifyRequesterOnExpiry     types.Bool   `tfsdk:"notify_requester_on_expiry"`
}

type automationJiraModel struct {
	Project        types.String `tfsdk:"project"`
	IssueType      types.String `tfsdk:"issue_type"`
	Summary        types.String `tfsdk:"summary"`
	UpdateOnReview types.Bool   `tfsdk:"update_on_review"`
}

type automationRuleModel struct {
	ID               types.String               `tfsdk:"id"`
	ProjectID        types.String               `tfsdk:"project_id"`
	Name             types.String               `tfsdk:"name"`
	Trigger          types.String               `tfsdk:"trigger"`
	Active           types.Bool                 `tfsdk:"active"`
	Conditions       *automationConditionsModel `tfsdk:"conditions"`
	Slack            *automationSlackModel      `tfsdk:"slack"`
	Jira             *automationJiraModel       `tfsdk:"jira"`
	UnmanagedActions types.List                 `tfsdk:"unmanaged_actions"`
	OtherChannels    types.List                 `tfsdk:"other_slack_channels"`
	OverwriteConsole types.Bool                 `tfsdk:"overwrite_console_changes"`
	CreatedAt        types.String               `tfsdk:"created_at"`
	CreatedBy        types.String               `tfsdk:"created_by"`
	ModifiedAt       types.String               `tfsdk:"modified_at"`
	ModifiedBy       types.String               `tfsdk:"modified_by"`
}

func (r *automationRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation_rule"
}

func conditionSchema(desc string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: desc,
		Attributes: map[string]schema.Attribute{
			"operator": schema.StringAttribute{
				Optional:    true,
				Description: "EQUAL matches the listed ids, NOT_EQUAL matches everything else. The API defaults to EQUAL.",
			},
			"values": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Ids to match.",
				Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
			},
		},
	}
}

func (r *automationRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An automation rule: when a trigger fires (a blocked activity, an exclusion about to expire, …), " +
			"Blast notifies Slack and/or opens a Jira ticket. Which fields apply depends on the trigger; the Blast API " +
			"rejects fields the trigger does not offer, listing every problem at once.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": projectIDAttribute(),
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Rule name.",
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"trigger": schema.StringAttribute{
				Required:      true,
				Description:   "What fires the rule, for example BLOCKED_ACTIVITY or EXCLUSION_REQUEST_CREATED (the API lists the valid triggers in its error). Changing it replaces the rule.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"active": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the rule runs. Defaults to true.",
			},
			"conditions": schema.SingleNestedAttribute{
				Validators:  []validator.Object{notEmptyObject()},
				Optional:    true,
				Description: "BLOCKED_ACTIVITY only: limit the rule to some security controls or sources.",
				Attributes: map[string]schema.Attribute{
					"security_control_ids": conditionSchema("Only blocked activities of these security controls (ids must exist)."),
					"source_entity_ids":    conditionSchema("Only blocked activities from these sources (source entity ids)."),
				},
			},
			"slack": schema.SingleNestedAttribute{
				Validators: []validator.Object{notEmptyObject()},
				Optional:   true,
				Description: "Post to Slack. Required for every trigger except EXCLUSION_REQUEST_CREATED, which takes slack, jira or both. " +
					"Set channel, or (BLOCKED_ACTIVITY only) use_notification_routing_rules.",
				Attributes: map[string]schema.Attribute{
					"channel": schema.StringAttribute{
						Optional:    true,
						Description: "Slack channel name, with or without the leading #.",
					},
					"use_notification_routing_rules": schema.BoolAttribute{
						Optional:    true,
						Description: "BLOCKED_ACTIVITY only: send to the recipients your notification routing rules pick, instead of a channel.",
					},
					"custom_text": schema.StringAttribute{
						Optional:    true,
						Description: "Text added to the Slack message.",
					},
					"allow_exclusion_request": schema.BoolAttribute{
						Optional:    true,
						Description: "BLOCKED_ACTIVITY with use_notification_routing_rules: offer a \"request an exclusion\" button.",
					},
					"allow_request_for": schema.ListAttribute{
						Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
						Optional:    true,
						ElementType: types.StringType,
						Description: "Guardrail catalog ids the button may request exclusions for. Omitted means every guardrail in the activity. Needs allow_exclusion_request.",
					},
					"max_exclusion_duration_days": schema.Int64Attribute{
						Optional:    true,
						Description: fmt.Sprintf("Longest exclusion, in days, a requester can ask for. Needs allow_exclusion_request. The API defaults to %d.", defaultExclusionDurationDays),
					},
					"notify_on_review": schema.BoolAttribute{
						Optional:    true,
						Description: "EXCLUSION_REQUEST_CREATED only: also post the decision when the request is approved or rejected.",
					},
					"notify_requester_on_expiry": schema.BoolAttribute{
						Optional:    true,
						Description: "EXCLUSION_ABOUT_TO_EXPIRE only: also DM each requester about their own expiring exclusions.",
					},
				},
			},
			"jira": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "EXCLUSION_REQUEST_CREATED only: open a Jira ticket.",
				Attributes: map[string]schema.Attribute{
					"project":          schema.StringAttribute{Required: true, Description: "Jira project key (the API returns the key, so a project name shows as a change)."},
					"issue_type":       schema.StringAttribute{Required: true, Description: "Issue type name, as it exists in that project."},
					"summary":          schema.StringAttribute{Required: true, Description: "Ticket summary."},
					"update_on_review": schema.BoolAttribute{Optional: true, Description: "Also comment on the ticket when the request is approved or rejected."},
				},
			},
			"unmanaged_actions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Actions on the rule this resource cannot express (for example webhooks added in the console). An update replaces the whole rule and removes them.",
			},
			"other_slack_channels": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Further Slack channels set in the console. Terraform manages only slack.channel, so an update removes these.",
			},
			"overwrite_console_changes": schema.BoolAttribute{
				Optional: true,
				Description: "An update replaces the whole rule. When the rule has actions or Slack channels set in the console " +
					"(unmanaged_actions, other_slack_channels), the plan fails unless this is true, so they are never deleted silently.",
			},
			"created_at":  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"created_by":  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"modified_at": schema.StringAttribute{Computed: true},
			"modified_by": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *automationRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, d := providerClient(req.ProviderData)
	resp.Diagnostics.Append(d...)
	r.client = c
}

// ---------- model -> API ----------

func optBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

func optInt64(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	i := v.ValueInt64()
	return &i
}

func conditionToAPI(ctx context.Context, m *automationConditionModel) (*client.AutomationCondition, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	vals, d := strList(ctx, m.Values)
	return &client.AutomationCondition{Operator: m.Operator.ValueString(), Values: vals}, d
}

func automationInput(ctx context.Context, m *automationRuleModel) (client.AutomationRuleInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	in := client.AutomationRuleInput{
		Name:    m.Name.ValueString(),
		Trigger: m.Trigger.ValueString(),
		Active:  optBool(m.Active),
	}
	if c := m.Conditions; c != nil {
		sc, d := conditionToAPI(ctx, c.SecurityControlIDs)
		diags.Append(d...)
		se, d := conditionToAPI(ctx, c.SourceEntityIDs)
		diags.Append(d...)
		in.Conditions = &client.AutomationConditions{SecurityControlIDs: sc, SourceEntityIDs: se}
	}
	if s := m.Slack; s != nil {
		allow, d := strList(ctx, s.AllowRequestFor)
		diags.Append(d...)
		in.Slack = &client.AutomationSlack{
			Channel:                     s.Channel.ValueString(),
			UseNotificationRoutingRules: optBool(s.UseNotificationRoutingRules),
			CustomText:                  s.CustomText.ValueString(),
			AllowExclusionRequest:       optBool(s.AllowExclusionRequest),
			AllowRequestFor:             allow,
			MaxExclusionDurationDays:    optInt64(s.MaxExclusionDurationDays),
			NotifyOnReview:              optBool(s.NotifyOnReview),
			NotifyRequesterOnExpiry:     optBool(s.NotifyRequesterOnExpiry),
		}
	}
	if j := m.Jira; j != nil {
		in.Jira = &client.AutomationJira{
			Project:        j.Project.ValueString(),
			IssueType:      j.IssueType.ValueString(),
			Summary:        j.Summary.ValueString(),
			UpdateOnReview: optBool(j.UpdateOnReview),
		}
	}
	return in, diags
}

// ---------- API -> model ----------
//
// The API fills in defaults the user may not have written (operator EQUAL, false booleans,
// 180 days) and returns the Slack channel without "#". Each helper keeps the configured value
// when it means the same thing, so a plan after apply or import is empty.

// keepBool: the API reports false (or nothing) for a flag the user did not set.
func keepBool(prior types.Bool, api *bool) types.Bool {
	v := api != nil && *api
	if prior.IsNull() && !v {
		return types.BoolNull()
	}
	return types.BoolValue(v)
}

// keepDefault: the API reports its default for a value the user did not set.
func keepDefault(prior types.String, api, def string) types.String {
	if api == "" || (prior.IsNull() && api == def) {
		return types.StringNull()
	}
	return types.StringValue(api)
}

func keepChannel(prior types.String, api string) types.String {
	if api == "" {
		return types.StringNull()
	}
	if !prior.IsNull() && strings.TrimPrefix(prior.ValueString(), "#") == strings.TrimPrefix(api, "#") {
		return prior
	}
	return types.StringValue(api)
}

func keepFold(prior types.String, api string) types.String {
	if !prior.IsNull() && strings.EqualFold(prior.ValueString(), api) {
		return prior
	}
	return types.StringValue(api)
}

func optList(vals []string) types.List {
	if len(vals) == 0 {
		return types.ListNull(types.StringType)
	}
	return toList(vals)
}

func conditionFromAPI(prior *automationConditionModel, api *client.AutomationCondition) *automationConditionModel {
	if api == nil {
		return nil
	}
	op := types.StringNull()
	if prior != nil {
		op = prior.Operator
	}
	return &automationConditionModel{Operator: keepDefault(op, api.Operator, defaultConditionOperator), Values: toList(api.Values)}
}

func (r *automationRuleResource) apply(m *automationRuleModel, a *client.AutomationRule) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(a.ID)
	m.Name = types.StringValue(a.Name)
	m.Trigger = types.StringValue(a.Trigger)
	m.Active = types.BoolValue(a.Active)

	if c := a.Conditions; c != nil && (c.SecurityControlIDs != nil || c.SourceEntityIDs != nil) {
		prior := m.Conditions
		if prior == nil {
			prior = &automationConditionsModel{}
		}
		m.Conditions = &automationConditionsModel{
			SecurityControlIDs: conditionFromAPI(prior.SecurityControlIDs, c.SecurityControlIDs),
			SourceEntityIDs:    conditionFromAPI(prior.SourceEntityIDs, c.SourceEntityIDs),
		}
	} else {
		m.Conditions = nil
	}

	if s := a.Slack; s != nil {
		p := m.Slack
		if p == nil {
			p = &automationSlackModel{
				Channel: types.StringNull(), UseNotificationRoutingRules: types.BoolNull(), CustomText: types.StringNull(),
				AllowExclusionRequest: types.BoolNull(), AllowRequestFor: types.ListNull(types.StringType),
				MaxExclusionDurationDays: types.Int64Null(), NotifyOnReview: types.BoolNull(), NotifyRequesterOnExpiry: types.BoolNull(),
			}
		}
		days := types.Int64Null()
		if s.MaxExclusionDurationDays != nil && !(p.MaxExclusionDurationDays.IsNull() && *s.MaxExclusionDurationDays == defaultExclusionDurationDays) {
			days = types.Int64Value(*s.MaxExclusionDurationDays)
		}
		m.Slack = &automationSlackModel{
			Channel:                     keepChannel(p.Channel, s.Channel),
			UseNotificationRoutingRules: keepBool(p.UseNotificationRoutingRules, s.UseNotificationRoutingRules),
			CustomText:                  optString(s.CustomText),
			AllowExclusionRequest:       keepBool(p.AllowExclusionRequest, s.AllowExclusionRequest),
			AllowRequestFor:             optList(s.AllowRequestFor),
			MaxExclusionDurationDays:    days,
			NotifyOnReview:              keepBool(p.NotifyOnReview, s.NotifyOnReview),
			NotifyRequesterOnExpiry:     keepBool(p.NotifyRequesterOnExpiry, s.NotifyRequesterOnExpiry),
		}
		if len(s.OtherChannels) > 0 {
			diags.AddWarning("Automation rule has more Slack channels than Terraform manages",
				fmt.Sprintf("Rule %s also posts to %s (set in the console). The next update replaces the rule and keeps only slack.channel.",
					a.ID, strings.Join(s.OtherChannels, ", ")))
		}
	} else {
		m.Slack = nil
	}

	if j := a.Jira; j != nil {
		p := m.Jira
		if p == nil {
			p = &automationJiraModel{Project: types.StringNull(), IssueType: types.StringNull(), UpdateOnReview: types.BoolNull()}
		}
		m.Jira = &automationJiraModel{
			Project:        keepFold(p.Project, j.Project),
			IssueType:      keepFold(p.IssueType, j.IssueType),
			Summary:        types.StringValue(j.Summary),
			UpdateOnReview: keepBool(p.UpdateOnReview, j.UpdateOnReview),
		}
	} else {
		m.Jira = nil
	}

	m.UnmanagedActions = toList(a.UnmanagedActions)
	var others []string
	if a.Slack != nil {
		others = a.Slack.OtherChannels
	}
	m.OtherChannels = toList(others)
	if len(a.UnmanagedActions) > 0 {
		diags.AddWarning("Automation rule has actions Terraform cannot manage",
			fmt.Sprintf("Rule %s also has: %s. The next update replaces the rule and removes them.", a.ID, strings.Join(a.UnmanagedActions, ", ")))
	}
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.CreatedBy = types.StringValue(a.CreatedBy)
	m.ModifiedAt = types.StringValue(a.ModifiedAt)
	m.ModifiedBy = types.StringValue(a.ModifiedBy)
	return diags
}

// ---------- CRUD ----------

func (r *automationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan automationRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, d := automationInput(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.client.CreateAutomationRule(ctx, plan.ProjectID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating automation rule", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(&plan, a)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	resp.Diagnostics.Append(rememberProject(ctx, resp.Private, plan.ProjectID, r.client, true)...)
}

func (r *automationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state automationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.client.GetAutomationRule(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading automation rule", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(&state, a)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	resp.Diagnostics.Append(rememberProject(ctx, resp.Private, state.ProjectID, r.client, false)...)
}

// Update sends the whole rule (PUT replaces it), so it also removes actions the rule had outside
// this resource; apply() warns about those on every read.
func (r *automationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state automationRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, d := automationInput(ctx, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.client.ReplaceAutomationRule(ctx, plan.ProjectID.ValueString(), state.ID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error updating automation rule", err.Error())
		return
	}
	resp.Diagnostics.Append(r.apply(&plan, a)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *automationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state automationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DeleteAutomationRule(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting automation rule", err.Error())
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

// ModifyPlan replaces the resource only when its effective project changes (see projectIDAttribute).
func (r *automationRuleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	planProjectReplace(ctx, r.client, req, resp)
	refuseSilentOverwrite(ctx, req, resp)
}

// refuseSilentOverwrite fails a plan that would update (PUT, a full replacement) or replace a rule
// that has actions or Slack channels set in the console, unless overwrite_console_changes = true.
// Without changes to the rule, or on create and destroy, it does nothing.
func refuseSilentOverwrite(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || req.Plan.Raw.Equal(req.State.Raw) {
		return
	}
	var unmanaged, others types.List
	var overwrite types.Bool
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("unmanaged_actions"), &unmanaged)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("other_slack_channels"), &others)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("overwrite_console_changes"), &overwrite)...)
	if resp.Diagnostics.HasError() || overwrite.ValueBool() {
		return
	}
	var lost []string
	for _, l := range []types.List{unmanaged, others} {
		if vals, _ := strList(ctx, l); len(vals) > 0 {
			lost = append(lost, vals...)
		}
	}
	if len(lost) == 0 {
		return
	}
	resp.Diagnostics.AddError("This change would delete parts of the rule set in the console",
		fmt.Sprintf("An update replaces the whole automation rule, so it would remove: %s. Remove them in the console, "+
			"add them to the configuration if the resource supports them, or set overwrite_console_changes = true to "+
			"delete them.", strings.Join(lost, ", ")))
}

package provider

import (
	"context"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &automationRuleDataSource{}

func NewAutomationRuleDataSource() datasource.DataSource { return &automationRuleDataSource{} }

type automationRuleDataSource struct{ client *client.Client }

var (
	automationConditionDSAttrTypes = map[string]attr.Type{
		"operator": types.StringType,
		"values":   types.ListType{ElemType: types.StringType},
	}
	automationConditionsDSAttrTypes = map[string]attr.Type{
		"security_control_ids": types.ObjectType{AttrTypes: automationConditionDSAttrTypes},
		"source_entity_ids":    types.ObjectType{AttrTypes: automationConditionDSAttrTypes},
	}
	automationSlackDSAttrTypes = map[string]attr.Type{
		"channel":                        types.StringType,
		"use_notification_routing_rules": types.BoolType,
		"custom_text":                    types.StringType,
		"allow_exclusion_request":        types.BoolType,
		"allow_request_for":              types.ListType{ElemType: types.StringType},
		"max_exclusion_duration_days":    types.Int64Type,
		"notify_on_review":               types.BoolType,
		"notify_requester_on_expiry":     types.BoolType,
	}
	automationJiraDSAttrTypes = map[string]attr.Type{
		"project":          types.StringType,
		"issue_type":       types.StringType,
		"summary":          types.StringType,
		"update_on_review": types.BoolType,
	}
)

type automationRuleDSModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectID        types.String `tfsdk:"project_id"`
	Name             types.String `tfsdk:"name"`
	Trigger          types.String `tfsdk:"trigger"`
	Active           types.Bool   `tfsdk:"active"`
	Conditions       types.Object `tfsdk:"conditions"`
	Slack            types.Object `tfsdk:"slack"`
	Jira             types.Object `tfsdk:"jira"`
	UnmanagedActions types.List   `tfsdk:"unmanaged_actions"`
	CreatedAt        types.String `tfsdk:"created_at"`
	CreatedBy        types.String `tfsdk:"created_by"`
	ModifiedAt       types.String `tfsdk:"modified_at"`
	ModifiedBy       types.String `tfsdk:"modified_by"`
}

func (d *automationRuleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_automation_rule"
}

func (d *automationRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	condAttr := schema.SingleNestedAttribute{
		Computed: true,
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"operator": schema.StringAttribute{Computed: true},
			"values":   schema.ListAttribute{Computed: true, ElementType: types.StringType},
		},
	}
	resp.Schema = schema.Schema{
		Description: "Look up a single automation rule by ID.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Required: true},
			"project_id": schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."},
			"name":       schema.StringAttribute{Computed: true},
			"trigger":    schema.StringAttribute{Computed: true},
			"active":     schema.BoolAttribute{Computed: true},
			"conditions": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"security_control_ids": condAttr,
					"source_entity_ids":    condAttr,
				},
			},
			"slack": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"channel":                        schema.StringAttribute{Computed: true},
					"use_notification_routing_rules": schema.BoolAttribute{Computed: true},
					"custom_text":                    schema.StringAttribute{Computed: true},
					"allow_exclusion_request":        schema.BoolAttribute{Computed: true},
					"allow_request_for":              schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"max_exclusion_duration_days":    schema.Int64Attribute{Computed: true},
					"notify_on_review":               schema.BoolAttribute{Computed: true},
					"notify_requester_on_expiry":     schema.BoolAttribute{Computed: true},
				},
			},
			"jira": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"project":          schema.StringAttribute{Computed: true},
					"issue_type":       schema.StringAttribute{Computed: true},
					"summary":          schema.StringAttribute{Computed: true},
					"update_on_review": schema.BoolAttribute{Computed: true},
				},
			},
			"unmanaged_actions": schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"created_at":        schema.StringAttribute{Computed: true},
			"created_by":        schema.StringAttribute{Computed: true},
			"modified_at":       schema.StringAttribute{Computed: true},
			"modified_by":       schema.StringAttribute{Computed: true},
		},
	}
}

func (d *automationRuleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func conditionDSObj(cond *client.AutomationCondition) types.Object {
	if cond == nil {
		return types.ObjectNull(automationConditionDSAttrTypes)
	}
	op := cond.Operator
	if op == "" {
		op = "EQUAL"
	}
	return types.ObjectValueMust(automationConditionDSAttrTypes, map[string]attr.Value{
		"operator": types.StringValue(op),
		"values":   toList(cond.Values),
	})
}

func (d *automationRuleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state automationRuleDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	a, err := d.client.GetAutomationRule(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading blast_automation_rule", err.Error())
		return
	}

	state.Name = types.StringValue(a.Name)
	state.Trigger = types.StringValue(a.Trigger)
	state.Active = types.BoolValue(a.Active)
	state.UnmanagedActions = toList(a.UnmanagedActions)
	state.CreatedAt = types.StringValue(a.CreatedAt)
	state.CreatedBy = types.StringValue(a.CreatedBy)
	state.ModifiedAt = types.StringValue(a.ModifiedAt)
	state.ModifiedBy = types.StringValue(a.ModifiedBy)

	if c := a.Conditions; c != nil && (c.SecurityControlIDs != nil || c.SourceEntityIDs != nil) {
		state.Conditions = types.ObjectValueMust(automationConditionsDSAttrTypes, map[string]attr.Value{
			"security_control_ids": conditionDSObj(c.SecurityControlIDs),
			"source_entity_ids":    conditionDSObj(c.SourceEntityIDs),
		})
	} else {
		state.Conditions = types.ObjectNull(automationConditionsDSAttrTypes)
	}

	if s := a.Slack; s != nil {
		days := types.Int64Null()
		if s.MaxExclusionDurationDays != nil {
			days = types.Int64Value(*s.MaxExclusionDurationDays)
		}
		routing := s.UseNotificationRoutingRules != nil && *s.UseNotificationRoutingRules
		allowReq := s.AllowExclusionRequest != nil && *s.AllowExclusionRequest
		notifyRev := s.NotifyOnReview != nil && *s.NotifyOnReview
		notifyExp := s.NotifyRequesterOnExpiry != nil && *s.NotifyRequesterOnExpiry
		state.Slack = types.ObjectValueMust(automationSlackDSAttrTypes, map[string]attr.Value{
			"channel":                        types.StringValue(s.Channel),
			"use_notification_routing_rules": types.BoolValue(routing),
			"custom_text":                    types.StringValue(s.CustomText),
			"allow_exclusion_request":        types.BoolValue(allowReq),
			"allow_request_for":              toList(s.AllowRequestFor),
			"max_exclusion_duration_days":    days,
			"notify_on_review":               types.BoolValue(notifyRev),
			"notify_requester_on_expiry":     types.BoolValue(notifyExp),
		})
	} else {
		state.Slack = types.ObjectNull(automationSlackDSAttrTypes)
	}

	if j := a.Jira; j != nil {
		updateRev := j.UpdateOnReview != nil && *j.UpdateOnReview
		state.Jira = types.ObjectValueMust(automationJiraDSAttrTypes, map[string]attr.Value{
			"project":          types.StringValue(j.Project),
			"issue_type":       types.StringValue(j.IssueType),
			"summary":          types.StringValue(j.Summary),
			"update_on_review": types.BoolValue(updateRev),
		})
	} else {
		state.Jira = types.ObjectNull(automationJiraDSAttrTypes)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

package provider

import (
	"context"
	"net/url"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &gapDataSource{}

func NewGapDataSource() datasource.DataSource { return &gapDataSource{} }

type gapDataSource struct{ client *client.Client }

var (
	gapPriorityMetricAttrTypes = map[string]attr.Type{
		"count":      types.Int64Type,
		"weight":     types.Float64Type,
		"max_weight": types.Float64Type,
	}
	gapThreatIntelAttrTypes = map[string]attr.Type{
		"count":      types.Int64Type,
		"weight":     types.Float64Type,
		"max_weight": types.Float64Type,
		"threat_id":  types.StringType,
		"link":       types.StringType,
	}
	gapPriorityIndicatorsAttrTypes = map[string]attr.Type{
		"effective_exposure":    types.ObjectType{AttrTypes: gapPriorityMetricAttrTypes},
		"suspicious_activities": types.ObjectType{AttrTypes: gapPriorityMetricAttrTypes},
		"violations_found":      types.ObjectType{AttrTypes: gapPriorityMetricAttrTypes},
		"threat_intel":          types.ObjectType{AttrTypes: gapThreatIntelAttrTypes},
	}
	gapMitigationAttrTypes = map[string]attr.Type{
		"description":    types.StringType,
		"link":           types.StringType,
		"guardrail_name": types.StringType,
		"guardrail_id":   types.StringType,
		"impact_label":   types.StringType,
	}
)

type gapDSModel struct {
	ID                 types.String  `tfsdk:"id"`
	ProjectID          types.String  `tfsdk:"project_id"`
	ExternalKey        types.String  `tfsdk:"external_key"`
	Name               types.String  `tfsdk:"name"`
	Description        types.String  `tfsdk:"description"`
	Category           types.String  `tfsdk:"category"`
	Type               types.String  `tfsdk:"type"`
	PriorityScore      types.Float64 `tfsdk:"priority_score"`
	PriorityLevel      types.String  `tfsdk:"priority_level"`
	IsQuickWin         types.Bool    `tfsdk:"is_quick_win"`
	Status             types.String  `tfsdk:"status"`
	GainPoints         types.Float64 `tfsdk:"gain_points"`
	PriorityIndicators types.Object  `tfsdk:"priority_indicators"`
	Mitigation         types.Object  `tfsdk:"mitigation"`
	LastDetectedAt     types.String  `tfsdk:"last_detected_at"`
	ResolvedAt         types.String  `tfsdk:"resolved_at"`
	CreatedAt          types.String  `tfsdk:"created_at"`
	UpdatedAt          types.String  `tfsdk:"updated_at"`
}

func (d *gapDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gap"
}

func priorityMetricAttr() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Computed: true,
		Attributes: map[string]schema.Attribute{
			"count":      schema.Int64Attribute{Computed: true},
			"weight":     schema.Float64Attribute{Computed: true},
			"max_weight": schema.Float64Attribute{Computed: true},
		},
	}
}

func (d *gapDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a single security gap by ID.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Required: true},
			"project_id":     schema.StringAttribute{Optional: true, Description: "Blast project ID. Overrides the provider default."},
			"external_key":   schema.StringAttribute{Computed: true},
			"name":           schema.StringAttribute{Computed: true},
			"description":    schema.StringAttribute{Computed: true},
			"category":       schema.StringAttribute{Computed: true},
			"type":           schema.StringAttribute{Computed: true},
			"priority_score": schema.Float64Attribute{Computed: true},
			"priority_level": schema.StringAttribute{Computed: true},
			"is_quick_win":   schema.BoolAttribute{Computed: true},
			"status":         schema.StringAttribute{Computed: true},
			"gain_points":    schema.Float64Attribute{Computed: true},
			"priority_indicators": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"effective_exposure":    priorityMetricAttr(),
					"suspicious_activities": priorityMetricAttr(),
					"violations_found":      priorityMetricAttr(),
					"threat_intel": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"count":      schema.Int64Attribute{Computed: true},
							"weight":     schema.Float64Attribute{Computed: true},
							"max_weight": schema.Float64Attribute{Computed: true},
							"threat_id":  schema.StringAttribute{Computed: true},
							"link":       schema.StringAttribute{Computed: true},
						},
					},
				},
			},
			"mitigation": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"description":    schema.StringAttribute{Computed: true},
					"link":           schema.StringAttribute{Computed: true},
					"guardrail_name": schema.StringAttribute{Computed: true},
					"guardrail_id":   schema.StringAttribute{Computed: true},
					"impact_label":   schema.StringAttribute{Computed: true},
				},
			},
			"last_detected_at": schema.StringAttribute{Computed: true},
			"resolved_at":      schema.StringAttribute{Computed: true},
			"created_at":       schema.StringAttribute{Computed: true},
			"updated_at":       schema.StringAttribute{Computed: true},
		},
	}
}

func (d *gapDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	c, diags := providerClient(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	d.client = c
}

func metricObjValue(m client.GapPriorityMetric) types.Object {
	return types.ObjectValueMust(gapPriorityMetricAttrTypes, map[string]attr.Value{
		"count":      types.Int64Value(int64(m.Count)),
		"weight":     types.Float64Value(m.Weight),
		"max_weight": types.Float64Value(m.MaxWeight),
	})
}

func (d *gapDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state gapDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := d.client.GetGap(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Gap not found", "No gap with id "+url.PathEscape(state.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Error reading blast_gap", err.Error())
		return
	}

	state.ExternalKey = types.StringValue(g.ExternalKey)
	state.Name = types.StringValue(g.Name)
	state.Description = types.StringValue(g.Description)
	state.Category = types.StringValue(g.Category)
	state.Type = types.StringValue(g.Type)
	state.PriorityScore = types.Float64Value(g.PriorityScore)
	state.PriorityLevel = types.StringValue(g.PriorityLevel)
	state.IsQuickWin = types.BoolValue(g.IsQuickWin)
	state.Status = types.StringValue(g.Status)
	state.GainPoints = types.Float64Value(g.GainPoints)
	state.LastDetectedAt = types.StringValue(g.LastDetectedAt)
	state.ResolvedAt = types.StringValue(g.ResolvedAt)
	state.CreatedAt = types.StringValue(g.CreatedAt)
	state.UpdatedAt = types.StringValue(g.UpdatedAt)

	pi := g.PriorityIndicators
	state.PriorityIndicators = types.ObjectValueMust(gapPriorityIndicatorsAttrTypes, map[string]attr.Value{
		"effective_exposure":    metricObjValue(pi.EffectiveExposure),
		"suspicious_activities": metricObjValue(pi.SuspiciousActivities),
		"violations_found":      metricObjValue(pi.ViolationsFound),
		"threat_intel": types.ObjectValueMust(gapThreatIntelAttrTypes, map[string]attr.Value{
			"count":      types.Int64Value(int64(pi.ThreatIntel.Count)),
			"weight":     types.Float64Value(pi.ThreatIntel.Weight),
			"max_weight": types.Float64Value(pi.ThreatIntel.MaxWeight),
			"threat_id":  types.StringValue(pi.ThreatIntel.ThreatID),
			"link":       types.StringValue(pi.ThreatIntel.Link),
		}),
	})

	mit := g.Mitigation
	state.Mitigation = types.ObjectValueMust(gapMitigationAttrTypes, map[string]attr.Value{
		"description":    types.StringValue(mit.Description),
		"link":           types.StringValue(mit.Link),
		"guardrail_name": types.StringValue(mit.GuardrailName),
		"guardrail_id":   types.StringValue(mit.GuardrailID),
		"impact_label":   types.StringValue(mit.ImpactLabel),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

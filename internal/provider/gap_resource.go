package provider

import (
	"context"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &gapResource{}
	_ resource.ResourceWithImportState = &gapResource{}
)

func NewGapResource() resource.Resource { return &gapResource{} }

type gapResource struct{ client *client.Client }

type gapMetricModel struct {
	Count     types.Int64   `tfsdk:"count"`
	Weight    types.Float64 `tfsdk:"weight"`
	MaxWeight types.Float64 `tfsdk:"max_weight"`
}

type gapThreatIntelResModel struct {
	Count     types.Int64   `tfsdk:"count"`
	Weight    types.Float64 `tfsdk:"weight"`
	MaxWeight types.Float64 `tfsdk:"max_weight"`
	ThreatID  types.String  `tfsdk:"threat_id"`
	Link      types.String  `tfsdk:"link"`
}

type gapIndicatorsModel struct {
	EffectiveExposure    gapMetricModel         `tfsdk:"effective_exposure"`
	SuspiciousActivities gapMetricModel         `tfsdk:"suspicious_activities"`
	ViolationsFound      gapMetricModel         `tfsdk:"violations_found"`
	ThreatIntel          gapThreatIntelResModel `tfsdk:"threat_intel"`
}

type gapMitigationResModel struct {
	Description   types.String `tfsdk:"description"`
	Link          types.String `tfsdk:"link"`
	GuardrailName types.String `tfsdk:"guardrail_name"`
	GuardrailID   types.String `tfsdk:"guardrail_id"`
	ImpactLabel   types.String `tfsdk:"impact_label"`
}

type gapModel struct {
	ID                 types.String           `tfsdk:"id"`
	ProjectID          types.String           `tfsdk:"project_id"`
	ExternalKey        types.String           `tfsdk:"external_key"`
	Name               types.String           `tfsdk:"name"`
	Description        types.String           `tfsdk:"description"`
	Category           types.String           `tfsdk:"category"`
	Type               types.String           `tfsdk:"type"`
	PriorityScore      types.Int64            `tfsdk:"priority_score"`
	PriorityLevel      types.String           `tfsdk:"priority_level"`
	IsQuickWin         types.Bool             `tfsdk:"is_quick_win"`
	Status             types.String           `tfsdk:"status"`
	GainPoints         types.Int64            `tfsdk:"gain_points"`
	PriorityIndicators *gapIndicatorsModel    `tfsdk:"priority_indicators"`
	Mitigation         *gapMitigationResModel `tfsdk:"mitigation"`
	LastDetectedAt     types.String           `tfsdk:"last_detected_at"`
	ResolvedAt         types.String           `tfsdk:"resolved_at"`
	CreatedAt          types.String           `tfsdk:"created_at"`
	UpdatedAt          types.String           `tfsdk:"updated_at"`
}

func gapMetricSchema(desc string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: desc,
		Attributes: map[string]schema.Attribute{
			"count":      schema.Int64Attribute{Required: true, Description: "Number of occurrences driving this indicator."},
			"weight":     schema.Float64Attribute{Required: true, Description: "Weight of this indicator in the priority score."},
			"max_weight": schema.Float64Attribute{Required: true, Description: "Maximum possible weight for this indicator."},
		},
	}
}

func (r *gapResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gap"
}

func (r *gapResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A defense gap: a detected security-posture issue tracked by Blast. Gaps are project-scoped — a specific project_id is required (the default \"all\" scope is rejected by the API).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Required:      true,
				Description:   "Blast project ID. Gaps are project-scoped; this field is required (the default all-units scope is rejected by the API). Changing this forces a new gap.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"external_key": schema.StringAttribute{
				Optional:    true,
				Description: "Stable key for external upsert syncs. Unique per project when set.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name of the gap.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Detailed description of the gap.",
			},
			"category": schema.StringAttribute{
				Required:    true,
				Description: "Gap category. The API validates this; known values: missing_defense, defense_misalignment, exclusion_hygiene, defense_orchestration.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Gap type. The API validates this; known values: EXPIRED_EXCLUSION, UNUSED_EXCLUSION, UNMANAGED_EXCLUSION, CONFIGURATION_RESTRICTION, ACCESS_RESTRICTION, DEFENSE_PROTECTION, etc.",
			},
			"priority_score": schema.Int64Attribute{
				Required:    true,
				Description: "Priority score from 0 to 100.",
			},
			"priority_level": schema.StringAttribute{
				Required:    true,
				Description: "Priority level: critical, high, medium, or low.",
			},
			"is_quick_win": schema.BoolAttribute{
				Required:    true,
				Description: "Whether resolving this gap is considered a quick win.",
			},
			"status": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Gap status: open (default), in_progress, resolved, closed, or ignored.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"gain_points": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Points awarded when this gap is resolved. Defaults to 0.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"priority_indicators": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Breakdown of signals driving the priority score.",
				Attributes: map[string]schema.Attribute{
					"effective_exposure":    gapMetricSchema("Effective exposure signal."),
					"suspicious_activities": gapMetricSchema("Suspicious activities signal."),
					"violations_found":      gapMetricSchema("Violations found signal."),
					"threat_intel": schema.SingleNestedAttribute{
						Required:    true,
						Description: "Threat intelligence signal.",
						Attributes: map[string]schema.Attribute{
							"count":      schema.Int64Attribute{Required: true},
							"weight":     schema.Float64Attribute{Required: true},
							"max_weight": schema.Float64Attribute{Required: true},
							"threat_id": schema.StringAttribute{
								Optional:      true,
								Computed:      true,
								Description:   "ID of the linked threat record.",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
							"link": schema.StringAttribute{
								Optional:      true,
								Computed:      true,
								Description:   "Link to the threat record.",
								PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							},
						},
					},
				},
			},
			"mitigation": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Remediation guidance for this gap.",
				Attributes: map[string]schema.Attribute{
					"description": schema.StringAttribute{
						Required:    true,
						Description: "Human-readable remediation description.",
					},
					"link": schema.StringAttribute{
						Optional:      true,
						Computed:      true,
						Description:   "Deep link to the mitigating guardrail or exclusion.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"guardrail_name": schema.StringAttribute{
						Optional:      true,
						Computed:      true,
						Description:   "Name of the mitigating guardrail.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"guardrail_id": schema.StringAttribute{
						Optional:      true,
						Computed:      true,
						Description:   "ID of the mitigating guardrail.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"impact_label": schema.StringAttribute{
						Optional:      true,
						Computed:      true,
						Description:   "Human-readable label describing the impact (e.g. \"542 potential exposures\").",
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
				},
			},
			"last_detected_at": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "ISO 8601 timestamp of the most recent detection.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"resolved_at": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "ISO 8601 timestamp when the gap was resolved. When status is resolved and this is omitted, the API defaults it to the current time.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *gapResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, d := providerClient(req.ProviderData)
	resp.Diagnostics.Append(d...)
	r.client = c
}

func gapModelToCreateGap(plan gapModel) client.CreateGap {
	in := client.CreateGap{
		Name:          plan.Name.ValueString(),
		Category:      plan.Category.ValueString(),
		Type:          plan.Type.ValueString(),
		PriorityScore: int(plan.PriorityScore.ValueInt64()),
		PriorityLevel: plan.PriorityLevel.ValueString(),
		IsQuickWin:    plan.IsQuickWin.ValueBool(),
	}
	if !plan.ExternalKey.IsNull() && !plan.ExternalKey.IsUnknown() {
		in.ExternalKey = plan.ExternalKey.ValueString()
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		in.Description = plan.Description.ValueString()
	}
	if !plan.Status.IsNull() && !plan.Status.IsUnknown() {
		in.Status = plan.Status.ValueString()
	}
	if !plan.GainPoints.IsNull() && !plan.GainPoints.IsUnknown() {
		gp := int(plan.GainPoints.ValueInt64())
		in.GainPoints = &gp
	}
	if !plan.LastDetectedAt.IsNull() && !plan.LastDetectedAt.IsUnknown() {
		in.LastDetectedAt = plan.LastDetectedAt.ValueString()
	}
	if !plan.ResolvedAt.IsNull() && !plan.ResolvedAt.IsUnknown() {
		in.ResolvedAt = plan.ResolvedAt.ValueString()
	}

	if plan.PriorityIndicators != nil {
		pi := plan.PriorityIndicators
		in.PriorityIndicators = client.GapPriorityIndicators{
			EffectiveExposure: client.GapPriorityMetric{
				Count: int(pi.EffectiveExposure.Count.ValueInt64()), Weight: pi.EffectiveExposure.Weight.ValueFloat64(), MaxWeight: pi.EffectiveExposure.MaxWeight.ValueFloat64(),
			},
			SuspiciousActivities: client.GapPriorityMetric{
				Count: int(pi.SuspiciousActivities.Count.ValueInt64()), Weight: pi.SuspiciousActivities.Weight.ValueFloat64(), MaxWeight: pi.SuspiciousActivities.MaxWeight.ValueFloat64(),
			},
			ViolationsFound: client.GapPriorityMetric{
				Count: int(pi.ViolationsFound.Count.ValueInt64()), Weight: pi.ViolationsFound.Weight.ValueFloat64(), MaxWeight: pi.ViolationsFound.MaxWeight.ValueFloat64(),
			},
			ThreatIntel: client.GapThreatIntel{
				Count: int(pi.ThreatIntel.Count.ValueInt64()), Weight: pi.ThreatIntel.Weight.ValueFloat64(), MaxWeight: pi.ThreatIntel.MaxWeight.ValueFloat64(),
				ThreatID: pi.ThreatIntel.ThreatID.ValueString(),
				Link:     pi.ThreatIntel.Link.ValueString(),
			},
		}
	}

	if plan.Mitigation != nil {
		mit := plan.Mitigation
		in.Mitigation = client.GapMitigation{
			Description:   mit.Description.ValueString(),
			Link:          mit.Link.ValueString(),
			GuardrailName: mit.GuardrailName.ValueString(),
			GuardrailID:   mit.GuardrailID.ValueString(),
			ImpactLabel:   mit.ImpactLabel.ValueString(),
		}
	}
	return in
}

func gapDetailToModel(g *client.GapDetail, plan gapModel) (gapModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	m := plan
	m.ID = types.StringValue(g.ID)
	m.Name = types.StringValue(g.Name)
	m.Category = types.StringValue(g.Category)
	m.Type = types.StringValue(g.Type)
	m.PriorityScore = types.Int64Value(int64(g.PriorityScore))
	m.PriorityLevel = types.StringValue(g.PriorityLevel)
	m.IsQuickWin = types.BoolValue(g.IsQuickWin)
	m.Status = types.StringValue(g.Status)
	m.GainPoints = types.Int64Value(int64(g.GainPoints))
	m.ExternalKey = optString(g.ExternalKey)
	m.Description = optString(g.Description)
	m.LastDetectedAt = optString(g.LastDetectedAt)
	m.ResolvedAt = optString(g.ResolvedAt)
	m.CreatedAt = types.StringValue(g.CreatedAt)
	m.UpdatedAt = types.StringValue(g.UpdatedAt)

	pi := g.PriorityIndicators
	indicators := gapIndicatorsModel{
		EffectiveExposure: gapMetricModel{
			Count:     types.Int64Value(int64(pi.EffectiveExposure.Count)),
			Weight:    types.Float64Value(pi.EffectiveExposure.Weight),
			MaxWeight: types.Float64Value(pi.EffectiveExposure.MaxWeight),
		},
		SuspiciousActivities: gapMetricModel{
			Count:     types.Int64Value(int64(pi.SuspiciousActivities.Count)),
			Weight:    types.Float64Value(pi.SuspiciousActivities.Weight),
			MaxWeight: types.Float64Value(pi.SuspiciousActivities.MaxWeight),
		},
		ViolationsFound: gapMetricModel{
			Count:     types.Int64Value(int64(pi.ViolationsFound.Count)),
			Weight:    types.Float64Value(pi.ViolationsFound.Weight),
			MaxWeight: types.Float64Value(pi.ViolationsFound.MaxWeight),
		},
		ThreatIntel: gapThreatIntelResModel{
			Count:     types.Int64Value(int64(pi.ThreatIntel.Count)),
			Weight:    types.Float64Value(pi.ThreatIntel.Weight),
			MaxWeight: types.Float64Value(pi.ThreatIntel.MaxWeight),
			ThreatID:  optString(pi.ThreatIntel.ThreatID),
			Link:      optString(pi.ThreatIntel.Link),
		},
	}
	m.PriorityIndicators = &indicators

	mit := g.Mitigation
	mitigation := gapMitigationResModel{
		Description:   types.StringValue(mit.Description),
		Link:          optString(mit.Link),
		GuardrailName: optString(mit.GuardrailName),
		GuardrailID:   optString(mit.GuardrailID),
		ImpactLabel:   optString(mit.ImpactLabel),
	}
	m.Mitigation = &mitigation

	return m, diags
}

func (r *gapResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gapModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := r.client.CreateGap(ctx, plan.ProjectID.ValueString(), gapModelToCreateGap(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating blast_gap", err.Error())
		return
	}

	// Re-read to get the full stored state (create response may omit some computed fields).
	if full, err := r.client.GetGap(ctx, plan.ProjectID.ValueString(), g.ID); err == nil {
		g = full
	}

	state, diags := gapDetailToModel(g, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *gapResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gapModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := r.client.GetGap(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading blast_gap", err.Error())
		return
	}

	newState, diags := gapDetailToModel(g, state)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *gapResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state gapModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	g, err := r.client.UpdateGap(ctx, plan.ProjectID.ValueString(), plan.ID.ValueString(), gapModelToCreateGap(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating blast_gap", err.Error())
		return
	}

	newState, diags := gapDetailToModel(g, plan)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *gapResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gapModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteGap(ctx, state.ProjectID.ValueString(), state.ID.ValueString()); err != nil {
		if !client.IsNotFound(err) {
			resp.Diagnostics.AddError("Error deleting blast_gap", err.Error())
		}
	}
}

// ImportState accepts "<project_id>/<id>".
// project_id is required for gaps (the all-units scope is rejected), so it must always be provided.
func (r *gapResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, id := splitImportID(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if project != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), project)...)
	}
}


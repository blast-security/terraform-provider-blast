package provider

import (
	"context"
	"encoding/json"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &controlChangeResource{}
	_ resource.ResourceWithImportState    = &controlChangeResource{}
	_ resource.ResourceWithValidateConfig = &controlChangeResource{}
)

func NewControlChangeResource() resource.Resource { return &controlChangeResource{} }

type controlChangeResource struct{ client *client.Client }

type controlChangeModel struct {
	taskCommon
	SecurityControlID types.String `tfsdk:"security_control_id"`
	Name              types.String `tfsdk:"name"`
	Policy            types.String `tfsdk:"policy"`
	AttachUnitIDs     types.Set    `tfsdk:"attach_unit_ids"`
	DetachUnitIDs     types.Set    `tfsdk:"detach_unit_ids"`
}

func (r *controlChangeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_control_change"
}

func (r *controlChangeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := taskCommonAttrs()
	attrs["security_control_id"] = schema.StringAttribute{
		Required: true, Description: "Blast ID of the existing SCP or RCP to change.",
		PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
	}
	attrs["name"] = schema.StringAttribute{
		Optional: true, Computed: true, Description: "Task name. Blast assigns one when omitted. Can be changed in place.",
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
	attrs["policy"] = schema.StringAttribute{
		Required: true,
		Description: "Full desired policy document (JSON). Required even for attachment-only changes: pass the current one, " +
			"e.g. data.blast_security_control.x.policy. Changing it creates a new control-change task.",
		PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = !jsonEqual(req.StateValue.ValueString(), req.PlanValue.ValueString())
		}, "Changing the policy creates a new control-change task.", "")},
	}
	attrs["attach_unit_ids"] = schema.SetAttribute{
		Optional: true, ElementType: types.StringType, Description: "Org unit / account IDs to newly attach the control to.",
		PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
	}
	attrs["detach_unit_ids"] = schema.SetAttribute{
		Optional: true, ElementType: types.StringType, Description: "Org unit / account IDs to detach the control from.",
		PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
	}
	resp.Schema = schema.Schema{
		Description: "Changes an existing Service Control Policy or Resource Control Policy (content and/or attachments) by " +
			"creating a Blast control-change task, optionally opening a Terraform PR. Only one open control-change task per control " +
			"is allowed. The cloud changes only when the task is deployed.",
		Attributes: attrs,
	}
}

func jsonEqual(a, b string) bool {
	if a == b {
		return true
	}
	var va, vb any
	if json.Unmarshal([]byte(a), &va) != nil || json.Unmarshal([]byte(b), &vb) != nil {
		return false
	}
	ja, _ := json.Marshal(va)
	jb, _ := json.Marshal(vb)
	return string(ja) == string(jb)
}

func (r *controlChangeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	c, d := providerClient(req.ProviderData)
	resp.Diagnostics.Append(d...)
	r.client = c
}

func (r *controlChangeResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m controlChangeModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !m.Policy.IsNull() && !m.Policy.IsUnknown() && !json.Valid([]byte(m.Policy.ValueString())) {
		resp.Diagnostics.AddAttributeError(path.Root("policy"), "Invalid policy", "policy must be a valid JSON document (tip: use jsonencode()).")
	}
	resp.Diagnostics.Append(validatePRConfig(ctx, m.DeliveryMode, m.PullRequest)...)
}

func (r *controlChangeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan controlChangeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	attach, d := strSet(ctx, plan.AttachUnitIDs)
	resp.Diagnostics.Append(d...)
	detach, d := strSet(ctx, plan.DetachUnitIDs)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.CreateControlChange{
		SecurityControlID: plan.SecurityControlID.ValueString(),
		Policy:            plan.Policy.ValueString(),
		AttachUnitIDs:     attach,
		DetachUnitIDs:     detach,
	}
	if !plan.Name.IsUnknown() {
		in.Name = plan.Name.ValueString()
	}
	task, err := r.client.CreateControlChange(ctx, plan.ProjectID.ValueString(), in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating control-change task", conflictHint(err))
		return
	}
	plan.applyTask(task)
	plan.Name = types.StringValue(task.Name)
	resp.Diagnostics.Append(plan.openPR(ctx, r.client, task.Name)...)
	if full, err := r.client.GetTask(ctx, plan.ProjectID.ValueString(), task.ID); err == nil {
		prURL := plan.TerraformPrURL
		plan.applyTask(full)
		if full.Name != "" {
			plan.Name = types.StringValue(full.Name)
		}
		if plan.TerraformPrURL.IsNull() {
			plan.TerraformPrURL = prURL
		}
	}
	if plan.TerraformPrURL.IsUnknown() {
		plan.TerraformPrURL = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *controlChangeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state controlChangeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	task, err := r.client.GetTask(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading task", err.Error())
		return
	}
	state.applyTask(task)
	state.Name = types.StringValue(task.Name)
	if state.SecurityControlID.IsNull() { // import
		state.SecurityControlID = types.StringValue(task.SecurityControlID)
		state.Policy = optString(client.PolicyString(task.Policy))
		if len(task.AttachUnitIDs) > 0 {
			state.AttachUnitIDs = toSet(task.AttachUnitIDs)
		}
		if len(task.DetachUnitIDs) > 0 {
			state.DetachUnitIDs = toSet(task.DetachUnitIDs)
		}
		state.DeliveryMode = types.StringValue(deliveryTask)
		if task.TerraformPrURL != "" {
			state.DeliveryMode = types.StringValue(deliveryOpenPR)
		}
		state.OnDestroy = types.StringValue(onDestroyDelete)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *controlChangeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state controlChangeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	if !plan.Name.IsUnknown() && !plan.Name.Equal(state.Name) {
		if _, err := r.client.UpdateTask(ctx, plan.ProjectID.ValueString(), state.ID.ValueString(), client.UpdateTask{Name: plan.Name.ValueString()}); err != nil {
			resp.Diagnostics.AddError("Error renaming task", err.Error())
			return
		}
	}
	plan.TerraformPrURL = state.TerraformPrURL
	if state.DeliveryMode.ValueString() != deliveryOpenPR && plan.DeliveryMode.ValueString() == deliveryOpenPR {
		resp.Diagnostics.Append(plan.openPR(ctx, r.client, plan.Name.ValueString())...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	task, err := r.client.GetTask(ctx, plan.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading task", err.Error())
		return
	}
	prURL := plan.TerraformPrURL
	plan.applyTask(task)
	plan.Name = types.StringValue(task.Name)
	if plan.TerraformPrURL.IsNull() || plan.TerraformPrURL.IsUnknown() {
		plan.TerraformPrURL = prURL
	}
	if plan.TerraformPrURL.IsUnknown() {
		plan.TerraformPrURL = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *controlChangeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state controlChangeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(state.destroy(ctx, r.client)...)
}

func (r *controlChangeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, id := splitImportID(req.ID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if project != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), project)...)
	}
}

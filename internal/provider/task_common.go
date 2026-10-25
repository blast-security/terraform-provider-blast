package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Shared pieces for resources that are backed by a Blast task
// (blast_guardrail_deployment, blast_security_control_change).
//
// Lifecycle:
//   create  -> POST the task; if delivery_mode = "open_pr", POST /tasks/{id}/terraform-pr
//   read    -> GET /tasks/{id}; refreshes status + PR URL only (task inputs are immutable)
//   update  -> only `name` (PATCH) and task -> open_pr (opens the PR) are in-place
//   delete  -> on_destroy = "delete" (DELETE /tasks/{id}) or "abandon" (forget it)
//
// Deleting the task never rolls back a change that was already deployed to the cloud.

const (
	onDestroyDelete  = "delete"
	onDestroyAbandon = "abandon"
)

type pullRequestModel struct {
	Title              types.String `tfsdk:"title"`
	Description        types.String `tfsdk:"description"`
	SourceBranchPrefix types.String `tfsdk:"source_branch_prefix"`
	Targets            types.Set    `tfsdk:"targets"`
}

var pullRequestAttrTypes = map[string]attr.Type{
	"title":                types.StringType,
	"description":          types.StringType,
	"source_branch_prefix": types.StringType,
	"targets":              types.SetType{ElemType: types.StringType},
}

func taskCommonAttrs() map[string]schema.Attribute {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed: true, Description: "Blast task ID.", PlanModifiers: keep,
		},
		"project_id": schema.StringAttribute{
			Optional: true, Description: "Blast project ID. Overrides the provider default.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"delivery_mode": schema.StringAttribute{
			Optional: true, Computed: true, Default: stringdefault.StaticString(deliveryTask),
			Description: "How the change is delivered: \"task\" (create a Blast task only, deploy from Blast) or \"open_pr\" " +
				"(also open a Terraform PR/MR through the tenant's git integration). Switching task -> open_pr opens the PR in place.",
			Validators: []validator.String{stringvalidator.OneOf(deliveryModes...)},
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
					resp.RequiresReplace = req.StateValue.ValueString() == deliveryOpenPR && req.PlanValue.ValueString() != deliveryOpenPR
				}, "A PR that was already opened cannot be withdrawn; switching back to \"task\" replaces the task.", ""),
			},
		},
		"pull_request": schema.SingleNestedAttribute{
			Optional:    true,
			Description: "PR/MR settings used when delivery_mode = \"open_pr\".",
			PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
				// Once a PR exists, changing its settings means a new task + PR.
				resp.RequiresReplace = !req.StateValue.IsNull()
			}, "Changing PR settings after the PR was opened replaces the task.", "")},
			Attributes: map[string]schema.Attribute{
				"title":                schema.StringAttribute{Optional: true, Description: "PR/MR title. Defaults to \"Blast: <task name>\"."},
				"description":          schema.StringAttribute{Optional: true},
				"source_branch_prefix": schema.StringAttribute{Optional: true},
				"targets": schema.SetAttribute{
					Optional: true, ElementType: types.StringType,
					Description: "Org unit / account IDs the generated Terraform applies to. Defaults per the API.",
				},
			},
		},
		"on_destroy": schema.StringAttribute{
			Optional: true, Computed: true, Default: stringdefault.StaticString(onDestroyDelete),
			Description: "What `terraform destroy` does: \"delete\" deletes the Blast task, \"abandon\" only removes it from state. " +
				"Neither rolls back a change already deployed to the cloud.",
			Validators: []validator.String{stringvalidator.OneOf(onDestroyDelete, onDestroyAbandon)},
		},
		"status":           schema.StringAttribute{Computed: true, Description: "Current task status (Open, In Progress, Closed, ...)."},
		"task_type":        schema.StringAttribute{Computed: true, Description: "Guardrail, Control or Exclusion.", PlanModifiers: keep},
		"terraform_pr_url": schema.StringAttribute{Computed: true, Description: "URL of the opened PR/MR, if any."},
		"created_at":       schema.StringAttribute{Computed: true, PlanModifiers: keep},
	}
}

// taskCommon is embedded by both task-backed resource models.
type taskCommon struct {
	ID             types.String `tfsdk:"id"`
	ProjectID      types.String `tfsdk:"project_id"`
	DeliveryMode   types.String `tfsdk:"delivery_mode"`
	PullRequest    types.Object `tfsdk:"pull_request"`
	OnDestroy      types.String `tfsdk:"on_destroy"`
	Status         types.String `tfsdk:"status"`
	TaskType       types.String `tfsdk:"task_type"`
	TerraformPrURL types.String `tfsdk:"terraform_pr_url"`
	CreatedAt      types.String `tfsdk:"created_at"`
}

func (t *taskCommon) applyTask(task *client.Task) {
	t.ID = types.StringValue(task.ID)
	t.Status = types.StringValue(task.Status)
	t.TaskType = types.StringValue(task.Type)
	t.CreatedAt = types.StringValue(task.CreatedAt)
	if task.TerraformPrURL != "" {
		t.TerraformPrURL = types.StringValue(task.TerraformPrURL)
	} else if t.TerraformPrURL.IsUnknown() {
		t.TerraformPrURL = types.StringNull()
	}
}

// openPR opens a Terraform PR for the task when delivery_mode = open_pr.
func (t *taskCommon) openPR(ctx context.Context, c *client.Client, taskName string) diag.Diagnostics {
	var diags diag.Diagnostics
	if t.DeliveryMode.ValueString() != deliveryOpenPR {
		return diags
	}
	in := client.CreateTerraformPr{Title: "Blast: " + taskName}
	if !t.PullRequest.IsNull() && !t.PullRequest.IsUnknown() {
		var pr pullRequestModel
		diags.Append(t.PullRequest.As(ctx, &pr, objectAsOpts)...)
		if v := pr.Title.ValueString(); v != "" {
			in.Title = v
		}
		in.Description = pr.Description.ValueString()
		in.SourceBranchPrefix = pr.SourceBranchPrefix.ValueString()
		targets, d := strSet(ctx, pr.Targets)
		diags.Append(d...)
		in.Targets = targets
	}
	if diags.HasError() {
		return diags
	}
	out, err := c.CreateTerraformPr(ctx, t.ProjectID.ValueString(), t.ID.ValueString(), in)
	if err != nil {
		diags.AddError("Task created, but opening the Terraform PR failed",
			fmt.Sprintf("Task %s was created in Blast. The resource is marked tainted; the next apply deletes and recreates it. "+
				"Check that the tenant has a git integration connected.\n\n%s", t.ID.ValueString(), err))
		return diags
	}
	t.TerraformPrURL = types.StringValue(out.URL)
	return diags
}

func (t *taskCommon) destroy(ctx context.Context, c *client.Client) diag.Diagnostics {
	var diags diag.Diagnostics
	if t.OnDestroy.ValueString() == onDestroyAbandon {
		return diags
	}
	if err := c.DeleteTask(ctx, t.ProjectID.ValueString(), t.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		diags.AddError("Error deleting Blast task", err.Error())
	}
	return diags
}

func conflictHint(err error) string {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict {
		return err.Error() + "\n\nThis control already has an open control-change task. Close it, set it to On Hold, or delete it in Blast, then apply again."
	}
	return err.Error()
}

func validatePRConfig(_ context.Context, _ types.String, _ types.Object) diag.Diagnostics {
	// pull_request may be set while delivery_mode = "task" so switching to "open_pr" later is a one-line change.
	return nil
}

package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/blast-security/terraform-provider-blast/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ptr[T any](v T) *T { return &v }

func nullSlack() *automationSlackModel {
	return &automationSlackModel{
		Channel: types.StringNull(), UseNotificationRoutingRules: types.BoolNull(), CustomText: types.StringNull(),
		AllowExclusionRequest: types.BoolNull(), AllowRequestFor: types.ListNull(types.StringType),
		MaxExclusionDurationDays: types.Int64Null(), NotifyOnReview: types.BoolNull(), NotifyRequesterOnExpiry: types.BoolNull(),
	}
}

// The API rejects fields a trigger does not offer, so only configured fields may be sent.
func TestAutomationInputSendsOnlyConfiguredFields(t *testing.T) {
	slack := nullSlack()
	slack.Channel = types.StringValue("#sec-alerts")
	m := &automationRuleModel{
		Name: types.StringValue("r"), Trigger: types.StringValue("COVERAGE_DECREASED"), Active: types.BoolValue(true), Slack: slack,
	}
	in, d := automationInput(context.Background(), m)
	if d.HasError() {
		t.Fatal(d)
	}
	b, _ := json.Marshal(in)
	want := `{"name":"r","trigger":"COVERAGE_DECREASED","active":true,"slack":{"channel":"#sec-alerts"}}`
	if string(b) != want {
		t.Fatalf("body\n got %s\nwant %s", b, want)
	}
}

// What the API fills in on its own must not show up as a change against the configuration.
func TestApplyKeepsConfiguredValuesTheAPIOnlyNormalises(t *testing.T) {
	slack := nullSlack()
	slack.Channel = types.StringValue("#sec-alerts")
	m := &automationRuleModel{
		Name: types.StringValue("r"), Trigger: types.StringValue("BLOCKED_ACTIVITY"), Active: types.BoolValue(true),
		Conditions: &automationConditionsModel{SecurityControlIDs: &automationConditionModel{Operator: types.StringNull(), Values: toList([]string{"sc-1"})}},
		Slack:      slack,
	}
	api := &client.AutomationRule{
		ID: "a1", Name: "r", Trigger: "BLOCKED_ACTIVITY", Active: true,
		Conditions: &client.AutomationConditions{SecurityControlIDs: &client.AutomationCondition{Operator: "EQUAL", Values: []string{"sc-1"}}},
		Slack:      &client.AutomationSlack{Channel: "sec-alerts"},
	}
	if d := (&automationRuleResource{}).apply(m, api); d.HasError() {
		t.Fatal(d)
	}
	if got := m.Slack.Channel.ValueString(); got != "#sec-alerts" {
		t.Errorf("channel = %q, want the configured #sec-alerts", got)
	}
	if !m.Conditions.SecurityControlIDs.Operator.IsNull() {
		t.Errorf("operator = %v, want null (EQUAL is the API default)", m.Conditions.SecurityControlIDs.Operator)
	}
	if !m.Slack.NotifyOnReview.IsNull() || !m.Slack.AllowExclusionRequest.IsNull() {
		t.Error("unset flags must stay null")
	}
}

func TestApplyRoutingRuleDefaultsAndRealDrift(t *testing.T) {
	slack := nullSlack()
	slack.UseNotificationRoutingRules = types.BoolValue(true)
	slack.AllowExclusionRequest = types.BoolValue(true)
	m := &automationRuleModel{Name: types.StringValue("r"), Trigger: types.StringValue("BLOCKED_ACTIVITY"), Slack: slack}
	api := &client.AutomationRule{
		ID: "a1", Name: "r renamed", Trigger: "BLOCKED_ACTIVITY", Active: false,
		Slack: &client.AutomationSlack{UseNotificationRoutingRules: ptr(true), AllowExclusionRequest: ptr(true), MaxExclusionDurationDays: ptr(int64(defaultExclusionDurationDays))},
	}
	(&automationRuleResource{}).apply(m, api)
	if !m.Slack.MaxExclusionDurationDays.IsNull() {
		t.Errorf("days = %v, want null (180 is the API default)", m.Slack.MaxExclusionDurationDays)
	}
	if m.Name.ValueString() != "r renamed" || m.Active.ValueBool() {
		t.Error("a change made outside Terraform must show as drift")
	}
	api.Slack.MaxExclusionDurationDays = ptr(int64(30))
	(&automationRuleResource{}).apply(m, api)
	if m.Slack.MaxExclusionDurationDays.ValueInt64() != 30 {
		t.Errorf("days = %v, want 30", m.Slack.MaxExclusionDurationDays)
	}
}

func TestApplyWarnsAboutWhatAnUpdateWouldRemove(t *testing.T) {
	m := &automationRuleModel{}
	api := &client.AutomationRule{
		ID: "a1", Name: "r", Trigger: "BLOCKED_ACTIVITY",
		Slack:            &client.AutomationSlack{Channel: "a", OtherChannels: []string{"b"}},
		UnmanagedActions: []string{"WEBHOOK"},
	}
	d := (&automationRuleResource{}).apply(m, api)
	if d.WarningsCount() != 2 {
		t.Fatalf("want 2 warnings (other channels, unmanaged actions), got %v", d)
	}
	if m.Slack.Channel.ValueString() != "a" {
		t.Errorf("import must take the API value, got %v", m.Slack.Channel)
	}
}

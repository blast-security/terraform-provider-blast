package client

import (
	"context"
	"net/http"
	"net/url"
)

// ---------- automation rules (/automations) ----------
//
// The API validates the body strictly per trigger: unknown fields, and fields the trigger does
// not offer, are rejected in one combined 400. So every optional field is a pointer or omitempty
// and is sent only when the configuration sets it.

type AutomationCondition struct {
	Operator string   `json:"operator,omitempty"`
	Values   []string `json:"values"`
}

type AutomationConditions struct {
	SecurityControlIDs *AutomationCondition `json:"securityControlIds,omitempty"`
	SourceEntityIDs    *AutomationCondition `json:"sourceEntityIds,omitempty"`
}

type AutomationSlack struct {
	Channel                     string   `json:"channel,omitempty"`
	UseNotificationRoutingRules *bool    `json:"useNotificationRoutingRules,omitempty"`
	CustomText                  string   `json:"customText,omitempty"`
	AllowExclusionRequest       *bool    `json:"allowExclusionRequest,omitempty"`
	AllowRequestFor             []string `json:"allowRequestFor,omitempty"`
	MaxExclusionDurationDays    *int64   `json:"maxExclusionDurationDays,omitempty"`
	NotifyOnReview              *bool    `json:"notifyOnReview,omitempty"`
	NotifyRequesterOnExpiry     *bool    `json:"notifyRequesterOnExpiry,omitempty"`
	// Response only: further channels of a rule created in the console. Never sent.
	OtherChannels []string `json:"otherChannels,omitempty"`
}

type AutomationJira struct {
	Project        string `json:"project"`
	IssueType      string `json:"issueType"`
	Summary        string `json:"summary"`
	UpdateOnReview *bool  `json:"updateOnReview,omitempty"`
}

// AutomationRuleInput is the body of POST /automations and PUT /automations/{id}.
// PUT replaces the whole rule, including actions this API cannot express.
type AutomationRuleInput struct {
	Name       string                `json:"name"`
	Trigger    string                `json:"trigger"`
	Active     *bool                 `json:"active,omitempty"`
	Conditions *AutomationConditions `json:"conditions,omitempty"`
	Slack      *AutomationSlack      `json:"slack,omitempty"`
	Jira       *AutomationJira       `json:"jira,omitempty"`
}

type AutomationRule struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Trigger          string                `json:"trigger"`
	Active           bool                  `json:"active"`
	Conditions       *AutomationConditions `json:"conditions"`
	Slack            *AutomationSlack      `json:"slack"`
	Jira             *AutomationJira       `json:"jira"`
	UnmanagedActions []string              `json:"unmanagedActions"`
	CreatedBy        string                `json:"createdBy"`
	ModifiedBy       string                `json:"modifiedBy"`
	CreatedAt        string                `json:"createdAt"`
	ModifiedAt       string                `json:"modifiedAt"`
}

func (c *Client) CreateAutomationRule(ctx context.Context, project string, in AutomationRuleInput) (*AutomationRule, error) {
	var out AutomationRule
	err := c.Do(ctx, http.MethodPost, "/automations", RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) GetAutomationRule(ctx context.Context, project, id string) (*AutomationRule, error) {
	var out AutomationRule
	err := c.Do(ctx, http.MethodGet, "/automations/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, &out)
	return &out, err
}

func (c *Client) ReplaceAutomationRule(ctx context.Context, project, id string, in AutomationRuleInput) (*AutomationRule, error) {
	var out AutomationRule
	err := c.Do(ctx, http.MethodPut, "/automations/"+url.PathEscape(id), RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) DeleteAutomationRule(ctx context.Context, project, id string) error {
	return c.Do(ctx, http.MethodDelete, "/automations/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, nil)
}

type AutomationRuleSummary struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Trigger      string   `json:"trigger"`
	Active       bool     `json:"active"`
	Destinations []string `json:"destinations"`
	ModifiedBy   string   `json:"modifiedBy"`
	ModifiedAt   string   `json:"modifiedAt"`
}

// automationsPageSize is the API's maximum page size for GET /automations (it rejects larger).
const automationsPageSize = 200

func (c *Client) ListAutomationRules(ctx context.Context, project, trigger string, active *bool) ([]AutomationRuleSummary, error) {
	q := url.Values{}
	if trigger != "" {
		q.Set("trigger", trigger)
	}
	if active != nil {
		if *active {
			q.Set("active", "true")
		} else {
			q.Set("active", "false")
		}
	}
	return listPagedSize[AutomationRuleSummary](ctx, c, "/automations", project, q, automationsPageSize)
}

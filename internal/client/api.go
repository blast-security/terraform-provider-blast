package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// ---------- shared ----------

type Page struct {
	Size          int `json:"size"`
	Number        int `json:"number"`
	TotalElements int `json:"totalElements"`
	TotalPages    int `json:"totalPages"`
}

const pageSize = 500
const maxPages = 1000

// ---------- exclusions ----------

type ExclusionRule struct {
	Identifier string   `json:"identifier"`
	Operator   string   `json:"operator"`
	Values     []string `json:"values"`
}

type CreateExclusion struct {
	GuardrailCatalogIDs []string        `json:"guardrailCatalogIds,omitempty"`
	Name                string          `json:"name"`
	Note                string          `json:"note,omitempty"`
	PlatformType        string          `json:"platformType,omitempty"`
	Sources             []ExclusionRule `json:"sources,omitempty"`
	Targets             []ExclusionRule `json:"targets,omitempty"`
	Units               []string        `json:"units,omitempty"`
	ExpirationDate      string          `json:"expirationDate,omitempty"`
}

// UpdateExclusion is sent as a full replacement of the mutable fields.
type UpdateExclusion struct {
	Name         string          `json:"name"`
	Note         string          `json:"note"`
	PlatformType string          `json:"platformType,omitempty"`
	Sources      []ExclusionRule `json:"sources,omitempty"`
	Targets      []ExclusionRule `json:"targets,omitempty"`
	Units        []string        `json:"units,omitempty"`
}

type Exclusion struct {
	ID                  string          `json:"id"`
	GuardrailID         string          `json:"guardrailId"`
	GuardrailCatalogIDs []string        `json:"guardrailCatalogIds"`
	Name                string          `json:"name"`
	Note                string          `json:"note"`
	PlatformType        string          `json:"platformType"`
	Sources             []ExclusionRule `json:"sources"`
	Targets             []ExclusionRule `json:"targets"`
	Units               []string        `json:"units"`
	ExpirationDate      string          `json:"expirationDate"`
	IsExpired           bool            `json:"isExpired"`
	CreatedAt           string          `json:"createdAt"`
	CreatedBy           string          `json:"createdBy"`
	UpdatedAt           string          `json:"updatedAt"`
}

func (c *Client) CreateExclusion(ctx context.Context, project string, in CreateExclusion) (*Exclusion, error) {
	var out Exclusion
	err := c.Do(ctx, http.MethodPost, "/exclusions", RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) GetExclusion(ctx context.Context, project, id string) (*Exclusion, error) {
	var out Exclusion
	err := c.Do(ctx, http.MethodGet, "/exclusions/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, &out)
	return &out, err
}

func (c *Client) UpdateExclusion(ctx context.Context, project, id string, in UpdateExclusion) (*Exclusion, error) {
	var out Exclusion
	err := c.Do(ctx, http.MethodPatch, "/exclusions/"+url.PathEscape(id), RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) DeleteExclusion(ctx context.Context, project, id string) error {
	return c.Do(ctx, http.MethodDelete, "/exclusions/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, nil)
}

func (c *Client) ListExclusions(ctx context.Context, project, guardrailID string) ([]Exclusion, error) {
	var all []Exclusion
	err := Paginate(ctx, maxPages, func(page int) (int, int, error) {
		q := url.Values{"page": {strconv.Itoa(page)}, "size": {strconv.Itoa(pageSize)}}
		if guardrailID != "" {
			q.Set("guardrailId", guardrailID)
		}
		var out struct {
			Data []Exclusion `json:"data"`
			Page *Page       `json:"page"`
		}
		if err := c.Do(ctx, http.MethodGet, "/exclusions", RequestOptions{ProjectID: project, Query: q}, nil, &out); err != nil {
			return 0, 0, err
		}
		all = append(all, out.Data...)
		total := 1
		if out.Page != nil {
			total = out.Page.TotalPages
		}
		return len(out.Data), total, nil
	})
	return all, err
}

// ---------- guardrails ----------

type MitreTechnique struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Tactics []string `json:"tactics"`
}

type SecurityControlRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type Guardrail struct {
	ID                   string               `json:"id"`
	CatalogID            string               `json:"catalogId"`
	Name                 string               `json:"name"`
	Description          string               `json:"description"`
	Severity             string               `json:"severity"`
	Status               string               `json:"status"`
	Coverage             *float64             `json:"coverage"`
	DefenseScore         *float64             `json:"defenseScore"`
	HasViolations        bool                 `json:"hasViolations"`
	HasEffectiveExposure bool                 `json:"hasEffectiveExposure"`
	SimulationResults    string               `json:"simulationResults"`
	Risks                []string             `json:"risks"`
	PlatformType         string               `json:"platformType"`
	PlatformService      string               `json:"platformService"`
	GuardrailType        string               `json:"guardrailType"`
	SecurityFrameworks   []string             `json:"securityFrameworks"`
	PolicyID             string               `json:"policyId"`
	SecurityControls     []SecurityControlRef `json:"securityControls"`
	MitreTechniques      json.RawMessage      `json:"mitreTechniques"`
}

func (c *Client) GetGuardrail(ctx context.Context, project, catalogID string) (*Guardrail, error) {
	var out Guardrail
	err := c.Do(ctx, http.MethodGet, "/guardrails/"+url.PathEscape(catalogID), RequestOptions{ProjectID: project}, nil, &out)
	return &out, err
}

// ListGuardrails passes filters straight through as query params (comma-separated values allowed).
func (c *Client) ListGuardrails(ctx context.Context, project string, filters url.Values) ([]Guardrail, error) {
	return listPaged[Guardrail](ctx, c, "/guardrails", project, filters)
}

// ---------- security controls ----------

type PlatformResource struct {
	ResourceID            string `json:"resourceId"`
	ResourceName          string `json:"resourceName"`
	ResourceType          string `json:"resourceType"`
	PlatformType          string `json:"platformType"`
	AttachedEffectiveness bool   `json:"attachedEffectiveness"`
	Monitored             bool   `json:"monitored"`
}

type SecurityControl struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	Type               string             `json:"type"`
	CloudProvider      string             `json:"cloudProvider"`
	Status             string             `json:"status"`
	AttachedGuardrails []string           `json:"attachedGuardrails"`
	ControlID          string             `json:"controlId"`
	ResourceType       string             `json:"resourceType"`
	OpenActionCount    *float64           `json:"openActionCount"`
	ActionsDisabled    bool               `json:"actionsDisabled"`
	PlatformResources  []PlatformResource `json:"platformResources"`
}

func (c *Client) GetSecurityControl(ctx context.Context, project, id string) (*SecurityControl, error) {
	var out SecurityControl
	err := c.Do(ctx, http.MethodGet, "/security-controls/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, &out)
	return &out, err
}

func (c *Client) GetSecurityControlPolicy(ctx context.Context, project, id string, optimized bool) (string, error) {
	var out struct {
		Content json.RawMessage `json:"content"`
	}
	q := url.Values{}
	if optimized {
		q.Set("optimized", "true")
	}
	if err := c.Do(ctx, http.MethodGet, "/security-controls/"+url.PathEscape(id)+"/policy", RequestOptions{ProjectID: project, Query: q}, nil, &out); err != nil {
		return "", err
	}
	return rawToString(out.Content), nil
}

func (c *Client) ListSecurityControls(ctx context.Context, project string, filters url.Values) ([]SecurityControl, error) {
	return listPaged[SecurityControl](ctx, c, "/security-controls", project, filters)
}

// ---------- tasks ----------

type CreateTask struct {
	Name                string   `json:"name"`
	GuardrailCatalogIDs []string `json:"guardrailCatalogIds"`
	SecurityControlType string   `json:"securityControlType"`
	PlatformUnitIDs     []string `json:"platformUnitIds"`
	PlatformType        string   `json:"platformType,omitempty"`
	NewControl          *bool    `json:"newControl,omitempty"`
	SecurityControlID   string   `json:"securityControlId,omitempty"`
	SecurityControlName string   `json:"securityControlName"`
}

type CreateControlChange struct {
	SecurityControlID string   `json:"securityControlId"`
	Name              string   `json:"name,omitempty"`
	Policy            string   `json:"policy"`
	AttachUnitIDs     []string `json:"attachUnitIds,omitempty"`
	DetachUnitIDs     []string `json:"detachUnitIds,omitempty"`
}

type UpdateTask struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

type Task struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Status              string          `json:"status"`
	Type                string          `json:"type"`
	SecurityControlType string          `json:"securityControlType"`
	SecurityControlID   string          `json:"securityControlId"`
	PlatformType        string          `json:"platformType"`
	GuardrailCatalogIDs []string        `json:"guardrailCatalogIds"`
	PlatformUnitIDs     []string        `json:"platformUnitIds"`
	AttachUnitIDs       []string        `json:"attachUnitIds"`
	DetachUnitIDs       []string        `json:"detachUnitIds"`
	Policy              json.RawMessage `json:"policy"`
	Terraform           string          `json:"terraform"`
	TerraformPrURL      string          `json:"terraformPrUrl"`
	CreatedAt           string          `json:"createdAt"`
}

type CreateTerraformPr struct {
	Title              string   `json:"title"`
	Description        string   `json:"description,omitempty"`
	SourceBranchPrefix string   `json:"sourceBranchPrefix,omitempty"`
	Targets            []string `json:"targets,omitempty"`
}

type TerraformPr struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

func (c *Client) CreateTask(ctx context.Context, project string, in CreateTask) (*Task, error) {
	var out Task
	err := c.Do(ctx, http.MethodPost, "/tasks", RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) CreateControlChange(ctx context.Context, project string, in CreateControlChange) (*Task, error) {
	var out Task
	err := c.Do(ctx, http.MethodPost, "/tasks/control-changes", RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) GetTask(ctx context.Context, project, id string) (*Task, error) {
	var out Task
	err := c.Do(ctx, http.MethodGet, "/tasks/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, &out)
	return &out, err
}

func (c *Client) UpdateTask(ctx context.Context, project, id string, in UpdateTask) (*Task, error) {
	var out Task
	err := c.Do(ctx, http.MethodPatch, "/tasks/"+url.PathEscape(id), RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) DeleteTask(ctx context.Context, project, id string) error {
	return c.Do(ctx, http.MethodDelete, "/tasks/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, nil)
}

func (c *Client) CreateTerraformPr(ctx context.Context, project, taskID string, in CreateTerraformPr) (*TerraformPr, error) {
	var out TerraformPr
	err := c.Do(ctx, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/terraform-pr", RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

// ---------- helpers ----------

func listPaged[T any](ctx context.Context, c *Client, path, project string, filters url.Values) ([]T, error) {
	var all []T
	err := Paginate(ctx, maxPages, func(page int) (int, int, error) {
		q := url.Values{}
		for k, v := range filters {
			q[k] = v
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("size", strconv.Itoa(pageSize))
		var out struct {
			Data []T   `json:"data"`
			Page *Page `json:"page"`
		}
		if err := c.Do(ctx, http.MethodGet, path, RequestOptions{ProjectID: project, Query: q}, nil, &out); err != nil {
			return 0, 0, err
		}
		all = append(all, out.Data...)
		total := 1
		if out.Page != nil {
			total = out.Page.TotalPages
		}
		return len(out.Data), total, nil
	})
	return all, err
}

// rawToString turns a JSON value that may be a string or an object into a string.
func rawToString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

// PolicyString exposes rawToString for task policy fields.
func PolicyString(raw json.RawMessage) string { return rawToString(raw) }

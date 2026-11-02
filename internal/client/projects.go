package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Projects are tenant-scoped: the x-project-id header is never sent.
var tenantScoped = RequestOptions{NoProject: true}

type ProjectUnit struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	PlatformType  string   `json:"platformType"`
	UnitTypes     []string `json:"unitTypes"`
	Regions       []string `json:"regions"`
	PlatformOrgID string   `json:"platformOrgId"`
	GroupingUnit  bool     `json:"groupingUnit"`
}

type Project struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	PlatformTypes []string      `json:"platformTypes"`
	NumberOfUnits *float64      `json:"numberOfUnits"`
	Status        string        `json:"status"`
	Default       bool          `json:"default"`
	Description   string        `json:"description"`
	Types         []string      `json:"types"`
	Units         []ProjectUnit `json:"units"`
}

// CreateProject: types is a comma-separated string in the API.
type CreateProject struct {
	Name        string   `json:"name"`
	UnitIDs     []string `json:"unitIds,omitempty"`
	Description string   `json:"description,omitempty"`
	Types       string   `json:"types,omitempty"`
}

// UpdateProject is sent as a full desired state. UnitIDs replaces membership exactly
// (an empty, non-nil slice clears it; nil leaves membership untouched).
type UpdateProject struct {
	Name        string    `json:"name,omitempty"`
	Description *string   `json:"description,omitempty"`
	Status      string    `json:"status,omitempty"`
	UnitIDs     *[]string `json:"unitIds,omitempty"`
	Types       *string   `json:"types,omitempty"`
}

type OrgUnit struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	PlatformType     string `json:"platformType"`
	PlatformOrgID    string `json:"platformOrgId"`
	MonitoringStatus string `json:"monitoringStatus"`
	ParentID         string `json:"parentId"`
}

func (c *Client) CreateProject(ctx context.Context, in CreateProject) (*Project, error) {
	var out Project
	err := c.Do(ctx, http.MethodPost, "/projects", tenantScoped, in, &out)
	return &out, err
}

func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var out Project
	err := c.Do(ctx, http.MethodGet, "/projects/"+url.PathEscape(id), tenantScoped, nil, &out)
	return &out, err
}

func (c *Client) UpdateProject(ctx context.Context, id string, in UpdateProject) (*Project, error) {
	var out Project
	err := c.Do(ctx, http.MethodPut, "/projects/"+url.PathEscape(id), tenantScoped, in, &out)
	return &out, err
}

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	return c.Do(ctx, http.MethodDelete, "/projects/"+url.PathEscape(id), tenantScoped, nil, nil)
}

func (c *Client) ListProjects(ctx context.Context, filters url.Values) ([]Project, error) {
	var all []Project
	err := Paginate(ctx, maxPages, func(page int) (int, int, error) {
		q := url.Values{}
		for k, v := range filters {
			q[k] = v
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("size", strconv.Itoa(pageSize))
		var out struct {
			Data []Project `json:"data"`
			Page *Page     `json:"page"`
		}
		opts := tenantScoped
		opts.Query = q
		if err := c.Do(ctx, http.MethodGet, "/projects", opts, nil, &out); err != nil {
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

func (c *Client) ProjectNameAvailable(ctx context.Context, name string) (bool, error) {
	var out struct {
		Available bool `json:"available"`
	}
	opts := tenantScoped
	opts.Query = url.Values{"name": {name}}
	err := c.Do(ctx, http.MethodGet, "/projects/name-available", opts, nil, &out)
	return out.Available, err
}

// ListOrgUnits returns the flattened cloud org tree; projectID restricts it to one project.
func (c *Client) ListOrgUnits(ctx context.Context, projectID string) ([]OrgUnit, error) {
	var out struct {
		Data []OrgUnit `json:"data"`
	}
	opts := tenantScoped
	if projectID != "" {
		opts.Query = url.Values{"projectId": {projectID}}
	}
	err := c.Do(ctx, http.MethodGet, "/projects/organizations", opts, nil, &out)
	return out.Data, err
}

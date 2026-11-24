package client

import (
	"context"
	"net/http"
	"net/url"
)

// ---------- gaps (/gaps) ----------

type GapPriorityMetric struct {
	Count     int     `json:"count"`
	Weight    float64 `json:"weight"`
	MaxWeight float64 `json:"maxWeight"`
}

type GapThreatIntel struct {
	Count     int     `json:"count"`
	Weight    float64 `json:"weight"`
	MaxWeight float64 `json:"maxWeight"`
	ThreatID  string  `json:"threatId"`
	Link      string  `json:"link"`
}

type GapPriorityIndicators struct {
	EffectiveExposure    GapPriorityMetric `json:"effectiveExposure"`
	SuspiciousActivities GapPriorityMetric `json:"suspiciousActivities"`
	ViolationsFound      GapPriorityMetric `json:"violationsFound"`
	ThreatIntel          GapThreatIntel    `json:"threatIntel"`
}

type GapMitigation struct {
	Description   string `json:"description"`
	Link          string `json:"link"`
	GuardrailName string `json:"guardrailName"`
	GuardrailID   string `json:"guardrailId"`
	ImpactLabel   string `json:"impactLabel"`
}

// GapListItem is returned by GET /gaps (paginated list).
type GapListItem struct {
	ID                 string                `json:"id"`
	ProjectID          string                `json:"projectId"`
	ExternalKey        string                `json:"externalKey"`
	Name               string                `json:"name"`
	Category           string                `json:"category"`
	Type               string                `json:"type"`
	PriorityScore      float64               `json:"priorityScore"`
	PriorityLevel      string                `json:"priorityLevel"`
	IsQuickWin         bool                  `json:"isQuickWin"`
	Status             string                `json:"status"`
	GainPoints         float64               `json:"gainPoints"`
	PriorityIndicators GapPriorityIndicators `json:"priorityIndicators"`
	CreatedAt          string                `json:"createdAt"`
	UpdatedAt          string                `json:"updatedAt"`
}

// GapDetail is returned by GET/POST/PUT /gaps/{id}.
type GapDetail struct {
	ID                 string                `json:"id"`
	ProjectID          string                `json:"projectId"`
	ExternalKey        string                `json:"externalKey"`
	Name               string                `json:"name"`
	Description        string                `json:"description"`
	Category           string                `json:"category"`
	Type               string                `json:"type"`
	PriorityScore      float64               `json:"priorityScore"`
	PriorityLevel      string                `json:"priorityLevel"`
	IsQuickWin         bool                  `json:"isQuickWin"`
	Status             string                `json:"status"`
	GainPoints         float64               `json:"gainPoints"`
	PriorityIndicators GapPriorityIndicators `json:"priorityIndicators"`
	Mitigation         GapMitigation         `json:"mitigation"`
	LastDetectedAt     string                `json:"lastDetectedAt"`
	ResolvedAt         string                `json:"resolvedAt"`
	CreatedAt          string                `json:"createdAt"`
	UpdatedAt          string                `json:"updatedAt"`
}

// CreateGap is both the POST body (create) and PUT body (full replacement).
type CreateGap struct {
	ExternalKey        string                `json:"externalKey,omitempty"`
	Name               string                `json:"name"`
	Description        string                `json:"description,omitempty"`
	Category           string                `json:"category"`
	Type               string                `json:"type"`
	PriorityScore      int                   `json:"priorityScore"`
	PriorityLevel      string                `json:"priorityLevel"`
	IsQuickWin         bool                  `json:"isQuickWin"`
	Status             string                `json:"status,omitempty"`
	GainPoints         *int                  `json:"gainPoints,omitempty"`
	PriorityIndicators GapPriorityIndicators `json:"priorityIndicators"`
	Mitigation         GapMitigation         `json:"mitigation"`
	LastDetectedAt     string                `json:"lastDetectedAt,omitempty"`
	ResolvedAt         string                `json:"resolvedAt,omitempty"`
}

// GapListFilters holds optional query parameters for GET /gaps.
type GapListFilters struct {
	Tab           string
	Category      []string
	Type          []string
	PriorityLevel []string
	Status        []string
	IsQuickWin    *bool
	ExternalKey   string
	Name          string
}

func (c *Client) CreateGap(ctx context.Context, project string, in CreateGap) (*GapDetail, error) {
	var out GapDetail
	err := c.Do(ctx, http.MethodPost, "/gaps", RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) GetGap(ctx context.Context, project, id string) (*GapDetail, error) {
	var out GapDetail
	err := c.Do(ctx, http.MethodGet, "/gaps/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, &out)
	return &out, err
}

func (c *Client) UpdateGap(ctx context.Context, project, id string, in CreateGap) (*GapDetail, error) {
	var out GapDetail
	err := c.Do(ctx, http.MethodPut, "/gaps/"+url.PathEscape(id), RequestOptions{ProjectID: project}, in, &out)
	return &out, err
}

func (c *Client) DeleteGap(ctx context.Context, project, id string) error {
	return c.Do(ctx, http.MethodDelete, "/gaps/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, nil)
}

func (c *Client) ListGaps(ctx context.Context, project string, filters GapListFilters) ([]GapListItem, error) {
	q := url.Values{}
	if filters.Tab != "" {
		q.Set("tab", filters.Tab)
	}
	for _, v := range filters.Category {
		q.Add("category", v)
	}
	for _, v := range filters.Type {
		q.Add("type", v)
	}
	for _, v := range filters.PriorityLevel {
		q.Add("priorityLevel", v)
	}
	for _, v := range filters.Status {
		q.Add("status", v)
	}
	if filters.IsQuickWin != nil {
		if *filters.IsQuickWin {
			q.Set("isQuickWin", "true")
		} else {
			q.Set("isQuickWin", "false")
		}
	}
	if filters.ExternalKey != "" {
		q.Set("externalKey", filters.ExternalKey)
	}
	if filters.Name != "" {
		q.Set("name", filters.Name)
	}
	return listPaged[GapListItem](ctx, c, "/gaps", project, q)
}

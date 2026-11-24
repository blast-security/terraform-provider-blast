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

func (c *Client) GetGap(ctx context.Context, project, id string) (*GapDetail, error) {
	var out GapDetail
	err := c.Do(ctx, http.MethodGet, "/gaps/"+url.PathEscape(id), RequestOptions{ProjectID: project}, nil, &out)
	return &out, err
}

package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func registerGaps(api *http.ServeMux, s *store, ext string) {
	api.HandleFunc("GET "+ext+"/gaps", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		q := r.URL.Query()
		tab := q.Get("tab")
		catFilter := q["category"]
		typeFilter := q["type"]
		levelFilter := q["priorityLevel"]
		statusFilter := q["status"]
		quickWin := q.Get("isQuickWin")
		extKey := q.Get("externalKey")
		nameFilter := strings.ToLower(q.Get("name"))

		var out []obj
		for _, g := range s.gaps {
			st := strVal(g["status"])

			// Tab filter: "open" = open+in_progress, "history" = resolved+closed+ignored, "all" = no filter.
			if tab == "open" && st != "open" && st != "in_progress" {
				continue
			}
			if tab == "history" && st != "resolved" && st != "closed" && st != "ignored" {
				continue
			}

			if len(catFilter) > 0 && !containsStr(catFilter, strVal(g["category"])) {
				continue
			}
			if len(typeFilter) > 0 && !containsStr(typeFilter, strVal(g["type"])) {
				continue
			}
			if len(levelFilter) > 0 && !containsStr(levelFilter, strVal(g["priorityLevel"])) {
				continue
			}
			if len(statusFilter) > 0 && !containsStr(statusFilter, st) {
				continue
			}
			if quickWin == "true" && g["isQuickWin"] != true {
				continue
			}
			if quickWin == "false" && g["isQuickWin"] != false {
				continue
			}
			if extKey != "" && strVal(g["externalKey"]) != extKey {
				continue
			}
			if nameFilter != "" && !strings.Contains(strings.ToLower(strVal(g["name"])), nameFilter) {
				continue
			}
			out = append(out, gapSummary(g))
		}
		writeJSON(w, 200, page(out))
	})

	api.HandleFunc("POST "+ext+"/gaps", func(w http.ResponseWriter, r *http.Request) {
		var in obj
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, 400, obj{"message": "invalid JSON"})
			return
		}
		for _, k := range []string{"name", "category", "type", "priorityScore", "priorityLevel", "isQuickWin"} {
			if in[k] == nil {
				writeJSON(w, 400, obj{"message": k + " is required"})
				return
			}
		}
		if in["mitigation"] == nil {
			writeJSON(w, 400, obj{"message": "mitigation is required"})
			return
		}
		if in["priorityIndicators"] == nil {
			writeJSON(w, 400, obj{"message": "priorityIndicators is required"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		// Reject duplicate externalKey within the same store (simplified: no per-project scoping in mock).
		if ek := strVal(in["externalKey"]); ek != "" {
			for _, g := range s.gaps {
				if strVal(g["externalKey"]) == ek {
					writeJSON(w, 409, obj{"message": "externalKey already exists in this project"})
					return
				}
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		in["id"] = s.id("gap")
		in["projectId"] = r.Header.Get("x-project-id")
		in["createdAt"] = now
		in["updatedAt"] = now
		if in["status"] == nil {
			in["status"] = "open"
		}
		if in["gainPoints"] == nil {
			in["gainPoints"] = 0.0
		}
		s.gaps[in["id"].(string)] = in
		writeJSON(w, 201, gapDetail(in))
	})

	api.HandleFunc("GET "+ext+"/gaps/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		g, ok := s.gaps[r.PathValue("id")]
		if !ok {
			writeJSON(w, 404, obj{"message": "Gap not found"})
			return
		}
		writeJSON(w, 200, gapDetail(g))
	})

	api.HandleFunc("PUT "+ext+"/gaps/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		id := r.PathValue("id")
		old, ok := s.gaps[id]
		if !ok {
			writeJSON(w, 404, obj{"message": "Gap not found"})
			return
		}
		var in obj
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, 400, obj{"message": "invalid JSON"})
			return
		}
		for _, k := range []string{"name", "category", "type", "priorityScore", "priorityLevel", "isQuickWin"} {
			if in[k] == nil {
				writeJSON(w, 400, obj{"message": k + " is required"})
				return
			}
		}
		if in["mitigation"] == nil {
			writeJSON(w, 400, obj{"message": "mitigation is required"})
			return
		}
		if in["priorityIndicators"] == nil {
			writeJSON(w, 400, obj{"message": "priorityIndicators is required"})
			return
		}
		// Check externalKey conflict with other gaps.
		if ek := strVal(in["externalKey"]); ek != "" {
			for gid, g := range s.gaps {
				if gid != id && strVal(g["externalKey"]) == ek {
					writeJSON(w, 409, obj{"message": "externalKey already exists in this project"})
					return
				}
			}
		}
		in["id"] = id
		in["projectId"] = r.Header.Get("x-project-id")
		in["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		if in["createdAt"] == nil {
			in["createdAt"] = old["createdAt"]
		}
		if in["status"] == nil {
			in["status"] = "open"
		}
		if in["gainPoints"] == nil {
			in["gainPoints"] = 0.0
		}
		s.gaps[id] = in
		writeJSON(w, 200, gapDetail(in))
	})

	api.HandleFunc("DELETE "+ext+"/gaps/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		id := r.PathValue("id")
		if _, ok := s.gaps[id]; !ok {
			writeJSON(w, 404, obj{"message": "Gap not found"})
			return
		}
		delete(s.gaps, id)
		writeJSON(w, 204, nil)
	})
}

func gapDetail(g obj) obj {
	out := gapSummary(g)
	out["description"] = strVal(g["description"])
	out["lastDetectedAt"] = strVal(g["lastDetectedAt"])
	out["resolvedAt"] = strVal(g["resolvedAt"])

	if m, ok := g["mitigation"].(map[string]any); ok {
		out["mitigation"] = obj{
			"description":   strVal(m["description"]),
			"link":          strVal(m["link"]),
			"guardrailName": strVal(m["guardrailName"]),
			"guardrailId":   strVal(m["guardrailId"]),
			"impactLabel":   strVal(m["impactLabel"]),
		}
	} else {
		out["mitigation"] = obj{
			"description":   "Remediate " + strVal(g["name"]),
			"link":          "",
			"guardrailName": "",
			"guardrailId":   "",
			"impactLabel":   "",
		}
	}
	return out
}

func gapSummary(g obj) obj {
	pi := gapIndicators(g["priorityIndicators"])
	return obj{
		"id":                 g["id"],
		"projectId":          strVal(g["projectId"]),
		"externalKey":        strVal(g["externalKey"]),
		"name":               g["name"],
		"category":           g["category"],
		"type":               g["type"],
		"priorityScore":      floatVal(g["priorityScore"]),
		"priorityLevel":      g["priorityLevel"],
		"isQuickWin":         g["isQuickWin"],
		"status":             strVal(g["status"]),
		"gainPoints":         floatVal(g["gainPoints"]),
		"priorityIndicators": pi,
		"createdAt":          strVal(g["createdAt"]),
		"updatedAt":          strVal(g["updatedAt"]),
	}
}

func gapIndicators(v any) obj {
	if m, ok := v.(map[string]any); ok {
		return obj{
			"effectiveExposure":    metricFrom(m["effectiveExposure"]),
			"suspiciousActivities": metricFrom(m["suspiciousActivities"]),
			"violationsFound":      metricFrom(m["violationsFound"]),
			"threatIntel":          threatIntelFrom(m["threatIntel"]),
		}
	}
	return obj{
		"effectiveExposure":    metricObj(0, 0, 0),
		"suspiciousActivities": metricObj(0, 0, 0),
		"violationsFound":      metricObj(0, 0, 0),
		"threatIntel":          obj{"count": 0, "weight": 0.0, "maxWeight": 0.0, "threatId": "", "link": ""},
	}
}

func metricFrom(v any) obj {
	if m, ok := v.(map[string]any); ok {
		return obj{
			"count":     floatVal(m["count"]),
			"weight":    floatVal(m["weight"]),
			"maxWeight": floatVal(m["maxWeight"]),
		}
	}
	return metricObj(0, 0, 0)
}

func threatIntelFrom(v any) obj {
	if m, ok := v.(map[string]any); ok {
		return obj{
			"count":     floatVal(m["count"]),
			"weight":    floatVal(m["weight"]),
			"maxWeight": floatVal(m["maxWeight"]),
			"threatId":  strVal(m["threatId"]),
			"link":      strVal(m["link"]),
		}
	}
	return obj{"count": 0, "weight": 0.0, "maxWeight": 0.0, "threatId": "", "link": ""}
}

func metricObj(count int, weight, maxWeight float64) obj {
	return obj{"count": count, "weight": weight, "maxWeight": maxWeight}
}

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func floatVal(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func registerGaps(api *http.ServeMux, s *store, ext string) {
	api.HandleFunc("GET "+ext+"/gaps", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		var out []obj
		for _, g := range s.gaps {
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
		s.mu.Lock()
		defer s.mu.Unlock()
		now := time.Now().UTC().Format(time.RFC3339)
		in["id"] = s.id("gap")
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
		_ = json.NewDecoder(r.Body).Decode(&in)
		in["id"] = id
		in["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		if in["createdAt"] == nil {
			in["createdAt"] = old["createdAt"]
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
		writeJSON(w, 200, obj{"id": id, "deleted": true})
	})
}

func gapDetail(g obj) obj {
	out := gapSummary(g)
	out["description"] = strVal(g["description"])
	out["lastDetectedAt"] = strVal(g["lastDetectedAt"])
	out["resolvedAt"] = strVal(g["resolvedAt"])
	out["mitigation"] = obj{
		"description":   fmt.Sprintf("Remediate %s", strVal(g["name"])),
		"link":          "",
		"guardrailName": "",
		"guardrailId":   "",
		"impactLabel":   "",
	}
	out["priorityIndicators"] = obj{
		"effectiveExposure":    metricObj(0, 0.0, 0.0),
		"suspiciousActivities": metricObj(0, 0.0, 0.0),
		"violationsFound":      metricObj(0, 0.0, 0.0),
		"threatIntel":          obj{"count": 0, "weight": 0.0, "maxWeight": 0.0, "threatId": "", "link": ""},
	}
	return out
}

func gapSummary(g obj) obj {
	return obj{
		"id":            g["id"],
		"projectId":     strVal(g["projectId"]),
		"externalKey":   strVal(g["externalKey"]),
		"name":          g["name"],
		"category":      g["category"],
		"type":          g["type"],
		"priorityScore": floatVal(g["priorityScore"]),
		"priorityLevel": g["priorityLevel"],
		"isQuickWin":    g["isQuickWin"],
		"status":        strVal(g["status"]),
		"gainPoints":    floatVal(g["gainPoints"]),
		"createdAt":     strVal(g["createdAt"]),
		"updatedAt":     strVal(g["updatedAt"]),
	}
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

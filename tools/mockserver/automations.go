package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Automation rules, shaped like the External API /automations:
//   - the body is checked strictly per trigger, and every problem comes back in one 400;
//   - the response fills in defaults (operator EQUAL, allowExclusionRequest false, 180 days)
//     and returns the Slack channel name without "#";
//   - PUT replaces the whole rule, including actions the API cannot express.

var automationSlackFields = map[string][]string{
	"BLOCKED_ACTIVITY":          {"channel", "useNotificationRoutingRules", "customText", "allowExclusionRequest", "allowRequestFor", "maxExclusionDurationDays"},
	"EXCLUSION_ABOUT_TO_EXPIRE": {"channel", "customText", "notifyRequesterOnExpiry"},
	"COVERAGE_DECREASED":        {"channel", "customText"},
	"NEW_GUARDRAILS_ADDED":      {"channel", "customText"},
	"EXCLUSION_REQUEST_CREATED": {"channel", "customText", "notifyOnReview"},
	"CONTROL_DRIFT_DETECTED":    {"channel", "customText"},
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func validateAutomation(in obj) []string {
	var errs []string
	for k := range in {
		if !contains([]string{"name", "trigger", "active", "conditions", "slack", "jira"}, k) {
			errs = append(errs, k+": unknown field")
		}
	}
	if s, _ := in["name"].(string); s == "" {
		errs = append(errs, "name: required")
	}
	trigger, _ := in["trigger"].(string)
	allowed, ok := automationSlackFields[trigger]
	if !ok {
		return append(errs, "trigger: must be one of the documented triggers")
	}
	if in["conditions"] != nil && trigger != "BLOCKED_ACTIVITY" {
		errs = append(errs, "conditions: not offered by "+trigger)
	}
	if in["jira"] != nil && trigger != "EXCLUSION_REQUEST_CREATED" {
		errs = append(errs, "jira: not offered by "+trigger)
	}
	slack, _ := in["slack"].(map[string]any)
	if slack == nil && (trigger != "EXCLUSION_REQUEST_CREATED" || in["jira"] == nil) {
		errs = append(errs, "slack: required")
	}
	for k := range slack {
		if !contains(allowed, k) {
			errs = append(errs, "slack."+k+": not offered by "+trigger)
		}
	}
	if slack != nil {
		_, hasChannel := slack["channel"]
		routing, _ := slack["useNotificationRoutingRules"].(bool)
		if hasChannel == routing {
			errs = append(errs, "slack: give channel or useNotificationRoutingRules, not both")
		}
		if allow, _ := slack["allowExclusionRequest"].(bool); allow && !routing {
			errs = append(errs, "slack.allowExclusionRequest: needs useNotificationRoutingRules")
		}
	}
	sort.Strings(errs)
	return errs
}

// automationResponse turns a stored request body into the public response, as the API does.
func automationResponse(id string, in obj, meta obj) obj {
	out := obj{"id": id, "name": in["name"], "trigger": in["trigger"], "active": true}
	if a, ok := in["active"].(bool); ok {
		out["active"] = a
	}
	if c, ok := in["conditions"].(map[string]any); ok {
		conds := obj{}
		for _, k := range []string{"securityControlIds", "sourceEntityIds"} {
			if v, ok := c[k].(map[string]any); ok {
				op, _ := v["operator"].(string)
				if op == "" {
					op = "EQUAL"
				}
				conds[k] = obj{"operator": op, "values": v["values"]}
			}
		}
		if len(conds) > 0 {
			out["conditions"] = conds
		}
	}
	if s, ok := in["slack"].(map[string]any); ok {
		slack := obj{}
		if s["customText"] != nil && s["customText"] != "" {
			slack["customText"] = s["customText"]
		}
		if routing, _ := s["useNotificationRoutingRules"].(bool); routing {
			enabled, _ := s["allowExclusionRequest"].(bool)
			slack["useNotificationRoutingRules"] = true
			slack["allowExclusionRequest"] = enabled
			if enabled {
				if l, ok := s["allowRequestFor"].([]any); ok && len(l) > 0 {
					slack["allowRequestFor"] = l
				}
				slack["maxExclusionDurationDays"] = 180
				if d, ok := s["maxExclusionDurationDays"]; ok {
					slack["maxExclusionDurationDays"] = d
				}
			}
		} else {
			slack["channel"] = strings.TrimPrefix(fmt.Sprint(s["channel"]), "#")
			for _, k := range []string{"notifyOnReview", "notifyRequesterOnExpiry"} {
				if b, _ := s[k].(bool); b {
					slack[k] = true
				}
			}
		}
		out["slack"] = slack
	}
	if j, ok := in["jira"].(map[string]any); ok {
		jira := obj{"project": j["project"], "issueType": j["issueType"], "summary": j["summary"]}
		if b, _ := j["updateOnReview"].(bool); b {
			jira["updateOnReview"] = true
		}
		out["jira"] = jira
	}
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func registerAutomations(api *http.ServeMux, s *store, ext string) {
	decode := func(w http.ResponseWriter, r *http.Request) (obj, bool) {
		var in obj
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeJSON(w, 400, obj{"message": "invalid JSON"})
			return nil, false
		}
		if errs := validateAutomation(in); len(errs) > 0 {
			writeJSON(w, 400, obj{"message": fmt.Sprintf("The automation rule has %d problem(s); see errors.", len(errs)), "errors": errs})
			return nil, false
		}
		return in, true
	}
	api.HandleFunc("POST "+ext+"/automations", func(w http.ResponseWriter, r *http.Request) {
		in, ok := decode(w, r)
		if !ok {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		now := time.Now().UTC().Format(time.RFC3339)
		id := s.id("auto")
		meta := obj{"createdAt": now, "modifiedAt": now, "createdBy": "terraform@blast.security", "modifiedBy": "terraform@blast.security"}
		s.automations[id] = obj{"body": in, "meta": meta}
		writeJSON(w, 201, automationResponse(id, in, meta))
	})
	api.HandleFunc("GET "+ext+"/automations/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		rec, ok := s.automations[r.PathValue("id")]
		if !ok {
			writeJSON(w, 404, obj{"message": "Automation rule not found"})
			return
		}
		writeJSON(w, 200, automationResponse(r.PathValue("id"), rec["body"].(obj), rec["meta"].(obj)))
	})
	api.HandleFunc("PUT "+ext+"/automations/{id}", func(w http.ResponseWriter, r *http.Request) {
		in, ok := decode(w, r)
		if !ok {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		rec, found := s.automations[r.PathValue("id")]
		if !found {
			writeJSON(w, 404, obj{"message": "Automation rule not found"})
			return
		}
		meta := rec["meta"].(obj)
		delete(meta, "unmanagedActions") // PUT replaces every action
		meta["modifiedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
		s.automations[r.PathValue("id")] = obj{"body": in, "meta": meta}
		writeJSON(w, 200, automationResponse(r.PathValue("id"), in, meta))
	})
	api.HandleFunc("DELETE "+ext+"/automations/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.automations[r.PathValue("id")]; !ok {
			writeJSON(w, 404, obj{"message": "Automation rule not found"})
			return
		}
		delete(s.automations, r.PathValue("id"))
		writeJSON(w, 200, obj{"success": true})
	})
}

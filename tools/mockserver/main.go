// Command mockserver is an in-memory fake of the Blast External API (the subset the
// provider uses) plus the OAuth token endpoint. Use it to try the provider locally:
//
//	go run ./tools/mockserver -addr :8089
//	export BLAST_API_URL=http://localhost:8089 BLAST_TOKEN_URL=http://localhost:8089/auth/v1/token
//	export BLAST_CLIENT_ID=test BLAST_CLIENT_SECRET=test
//
// It deliberately returns some enum values as raw keys (e.g. ACCOUNT_ID) to exercise
// the provider's label/raw-key normalisation.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type obj = map[string]any

type store struct {
	mu         sync.Mutex
	seq        int
	exclusions map[string]obj
	tasks      map[string]obj
	guardrails []obj
	controls   []obj
	policies   map[string]string
}

func newStore() *store {
	return &store{
		exclusions: map[string]obj{},
		tasks:      map[string]obj{},
		guardrails: []obj{
			{"id": "gr-1", "catalogId": "g-aws-s3-73735b0c", "name": "Block public S3 buckets", "severity": "High", "status": "Active",
				"coverage": 80.0, "platformType": "Amazon Web Services", "platformService": "S3", "risks": []string{"Data Leakage"},
				"guardrailType": "Configuration Restriction", "securityFrameworks": []string{"CIS AWS Foundations 4.0"},
				"description": "Prevents S3 buckets from being made public.", "policyId": "pol-1",
				"securityControls": []obj{{"id": "sc-1", "name": "blast-scp-baseline", "type": "Service Control Policy", "status": "Active"}}},
			{"id": "gr-2", "catalogId": "g-aws-vpc-f7264580", "name": "Deny default VPC usage", "severity": "Medium", "status": "Active",
				"coverage": 40.0, "platformType": "Amazon Web Services", "platformService": "VPC", "risks": []string{"Network Exposure"}},
			{"id": "gr-3", "catalogId": "g-az-st-11111111", "name": "Azure storage TLS", "severity": "High", "status": "Inactive",
				"platformType": "Azure Cloud", "platformService": "Storage"},
		},
		controls: []obj{
			{"id": "sc-1", "name": "blast-scp-baseline", "type": "Service Control Policy", "cloudProvider": "Amazon Web Services",
				"status": "Active", "attachedGuardrails": []string{"g-aws-s3-73735b0c"}, "controlId": "arn:aws:organizations::111111111111:policy/o-abc/service_control_policy/p-abc",
				"platformResources": []obj{{"resourceId": "ou-abcd-11111111", "resourceName": "Prod", "resourceType": "OU", "platformType": "Amazon Web Services", "attachedEffectiveness": true, "monitored": true}}},
			{"id": "sc-2", "name": "blast-rcp-data", "type": "Resource Control Policy", "cloudProvider": "Amazon Web Services", "status": "Active"},
		},
		policies: map[string]string{
			"sc-1": `{"Version":"2012-10-17","Statement":[{"Sid":"DenyPublicS3","Effect":"Deny","Action":"s3:PutBucketPolicy","Resource":"*"}]}`,
			"sc-2": `{"Version":"2012-10-17","Statement":[]}`,
		},
	}
}

func (s *store) id(prefix string) string { s.seq++; return fmt.Sprintf("%s-%04d", prefix, s.seq) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func page(data []obj) obj {
	if data == nil {
		data = []obj{}
	}
	return obj{"data": data, "page": obj{"size": len(data), "number": 0, "totalElements": len(data), "totalPages": 1}}
}

// rawKey turns "Account ID" into "ACCOUNT_ID" the way the backend's raw enum keys look.
func rawKey(s string) string { return strings.ToUpper(strings.ReplaceAll(s, " ", "_")) }

func main() {
	addr := flag.String("addr", ":8089", "listen address")
	flag.Parse()
	s := newStore()
	const token = "mock-token"

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/v1/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") == "" || r.Form.Get("client_secret") == "" {
			writeJSON(w, 401, obj{"error": "invalid_client"})
			return
		}
		writeJSON(w, 200, obj{"access_token": token, "token_type": "Bearer", "expires_in": 3600})
	})

	api := http.NewServeMux()
	ext := "/api/v1/external"

	// ---- exclusions ----
	api.HandleFunc("GET "+ext+"/exclusions", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		gid := r.URL.Query().Get("guardrailId")
		var out []obj
		for _, e := range s.exclusions {
			if gid != "" && !strings.Contains(fmt.Sprint(e["guardrailCatalogIds"]), gid) {
				continue
			}
			out = append(out, e)
		}
		writeJSON(w, 200, page(out))
	})
	api.HandleFunc("POST "+ext+"/exclusions", func(w http.ResponseWriter, r *http.Request) {
		var in obj
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in["name"] == nil {
			writeJSON(w, 400, obj{"message": "name is required"})
			return
		}
		if (in["sources"] != nil) == (in["targets"] != nil) {
			writeJSON(w, 400, obj{"message": "provide either sources or targets"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		now := time.Now().UTC().Format(time.RFC3339)
		e := obj{"id": s.id("exc"), "name": in["name"], "note": in["note"], "guardrailCatalogIds": in["guardrailCatalogIds"],
			"platformType": "AWS", "sources": rawRules(in["sources"]), "targets": rawRules(in["targets"]),
			"createdAt": now, "updatedAt": now, "createdBy": "terraform@blast.security", "isExpired": false}
		if in["expirationDate"] != nil {
			e["expirationDate"] = fmt.Sprint(in["expirationDate"]) + "T00:00:00.000Z"
		}
		s.exclusions[e["id"].(string)] = e
		writeJSON(w, 201, e)
	})
	api.HandleFunc("GET "+ext+"/exclusions/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if e, ok := s.exclusions[r.PathValue("id")]; ok {
			writeJSON(w, 200, e)
			return
		}
		writeJSON(w, 404, obj{"message": "Exclusion not found"})
	})
	api.HandleFunc("PATCH "+ext+"/exclusions/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in obj
		_ = json.NewDecoder(r.Body).Decode(&in)
		s.mu.Lock()
		defer s.mu.Unlock()
		e, ok := s.exclusions[r.PathValue("id")]
		if !ok {
			writeJSON(w, 404, obj{"message": "Exclusion not found"})
			return
		}
		for _, k := range []string{"name", "note"} {
			if v, ok := in[k]; ok {
				e[k] = v
			}
		}
		for _, k := range []string{"sources", "targets"} {
			if v, ok := in[k]; ok && v != nil {
				e[k] = rawRules(v)
			}
		}
		e["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		writeJSON(w, 200, e)
	})
	api.HandleFunc("DELETE "+ext+"/exclusions/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		id := r.PathValue("id")
		if _, ok := s.exclusions[id]; !ok {
			writeJSON(w, 404, obj{"message": "Exclusion not found"})
			return
		}
		delete(s.exclusions, id)
		writeJSON(w, 200, obj{"id": id, "deleted": true})
	})

	// ---- guardrails ----
	api.HandleFunc("GET "+ext+"/guardrails", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var out []obj
		for _, g := range s.guardrails {
			if v := q.Get("platformType"); v != "" && !strings.Contains(v, fmt.Sprint(g["platformType"])) {
				continue
			}
			if v := q.Get("severity"); v != "" && !strings.Contains(v, fmt.Sprint(g["severity"])) {
				continue
			}
			if v := q.Get("status"); v != "" && v != g["status"] {
				continue
			}
			if v := q.Get("name"); v != "" && !strings.Contains(strings.ToLower(fmt.Sprint(g["name"])), strings.ToLower(v)) {
				continue
			}
			out = append(out, g)
		}
		writeJSON(w, 200, page(out))
	})
	api.HandleFunc("GET "+ext+"/guardrails/{catalogId}", func(w http.ResponseWriter, r *http.Request) {
		for _, g := range s.guardrails {
			if g["catalogId"] == r.PathValue("catalogId") {
				writeJSON(w, 200, g)
				return
			}
		}
		writeJSON(w, 404, obj{"message": "Guardrail not found"})
	})

	// ---- security controls ----
	api.HandleFunc("GET "+ext+"/security-controls", func(w http.ResponseWriter, r *http.Request) {
		t := r.URL.Query().Get("securityControlType")
		var out []obj
		for _, c := range s.controls {
			if t != "" && t != c["type"] {
				continue
			}
			out = append(out, c)
		}
		writeJSON(w, 200, page(out))
	})
	api.HandleFunc("GET "+ext+"/security-controls/{id}", func(w http.ResponseWriter, r *http.Request) {
		for _, c := range s.controls {
			if c["id"] == r.PathValue("id") {
				writeJSON(w, 200, c)
				return
			}
		}
		writeJSON(w, 404, obj{"message": "Security control not found"})
	})
	api.HandleFunc("GET "+ext+"/security-controls/{id}/policy", func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.policies[r.PathValue("id")]
		if !ok {
			writeJSON(w, 404, obj{"message": "Policy not found"})
			return
		}
		writeJSON(w, 200, obj{"content": p})
	})

	// ---- tasks ----
	newTask := func(in obj, typ string) obj {
		t := obj{"id": s.id("task"), "status": "Open", "type": typ, "createdAt": time.Now().UTC().Format(time.RFC3339)}
		for k, v := range in {
			t[k] = v
		}
		if t["name"] == nil || t["name"] == "" {
			t["name"] = "Control change " + t["id"].(string)
		}
		s.tasks[t["id"].(string)] = t
		return t
	}
	api.HandleFunc("POST "+ext+"/tasks", func(w http.ResponseWriter, r *http.Request) {
		var in obj
		_ = json.NewDecoder(r.Body).Decode(&in)
		for _, k := range []string{"name", "guardrailCatalogIds", "securityControlType", "platformUnitIds", "securityControlName"} {
			if in[k] == nil {
				writeJSON(w, 400, obj{"message": k + " is required"})
				return
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(in, "newControl")
		if in["platformType"] == nil {
			in["platformType"] = "Amazon Web Services"
		}
		writeJSON(w, 201, newTask(in, "Guardrail"))
	})
	api.HandleFunc("POST "+ext+"/tasks/control-changes", func(w http.ResponseWriter, r *http.Request) {
		var in obj
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in["securityControlId"] == nil || in["policy"] == nil {
			writeJSON(w, 400, obj{"message": "securityControlId and policy are required"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, t := range s.tasks {
			if t["type"] == "Control" && t["securityControlId"] == in["securityControlId"] && t["status"] == "Open" {
				writeJSON(w, 409, obj{"message": "An open control-change task already exists"})
				return
			}
		}
		writeJSON(w, 201, newTask(in, "Control"))
	})
	api.HandleFunc("GET "+ext+"/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if t, ok := s.tasks[r.PathValue("id")]; ok {
			writeJSON(w, 200, t)
			return
		}
		writeJSON(w, 404, obj{"message": "Task not found"})
	})
	api.HandleFunc("PATCH "+ext+"/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in obj
		_ = json.NewDecoder(r.Body).Decode(&in)
		s.mu.Lock()
		defer s.mu.Unlock()
		t, ok := s.tasks[r.PathValue("id")]
		if !ok {
			writeJSON(w, 404, obj{"message": "Task not found"})
			return
		}
		for _, k := range []string{"name", "status"} {
			if v, ok := in[k]; ok && v != "" {
				t[k] = v
			}
		}
		writeJSON(w, 200, t)
	})
	api.HandleFunc("DELETE "+ext+"/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		id := r.PathValue("id")
		if _, ok := s.tasks[id]; !ok {
			writeJSON(w, 404, obj{"message": "Task not found"})
			return
		}
		delete(s.tasks, id)
		writeJSON(w, 200, obj{"id": id, "deleted": true})
	})
	api.HandleFunc("POST "+ext+"/tasks/{id}/terraform-pr", func(w http.ResponseWriter, r *http.Request) {
		var in obj
		_ = json.NewDecoder(r.Body).Decode(&in)
		s.mu.Lock()
		defer s.mu.Unlock()
		t, ok := s.tasks[r.PathValue("id")]
		if !ok {
			writeJSON(w, 404, obj{"message": "Task not found"})
			return
		}
		u := "https://github.com/acme/infra/pull/" + strings.TrimPrefix(t["id"].(string), "task-")
		t["terraformPrUrl"] = u
		writeJSON(w, 201, obj{"id": t["id"], "url": u, "title": in["title"]})
	})

	mux.Handle(ext+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			writeJSON(w, 401, obj{"message": "Invalid or missing token"})
			return
		}
		api.ServeHTTP(w, r)
	}))

	logged := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s project=%q", r.Method, r.URL.RequestURI(), r.Header.Get("x-project-id"))
		mux.ServeHTTP(w, r)
	})
	log.Printf("Blast mock API listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, logged))
}

// rawRules echoes rules back with raw enum keys, like the real backend may.
func rawRules(v any) any {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]obj, 0, len(list))
	for _, item := range list {
		m, _ := item.(map[string]any)
		out = append(out, obj{"identifier": rawKey(fmt.Sprint(m["identifier"])), "operator": rawKey(fmt.Sprint(m["operator"])), "values": m["values"]})
	}
	return out
}

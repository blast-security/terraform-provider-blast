package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func newTestServer(t *testing.T, api http.HandlerFunc) (*httptest.Server, *int32) {
	t.Helper()
	var tokenCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenCalls, 1)
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "id" || r.Form.Get("client_secret") != "secret" {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
	})
	mux.HandleFunc("/api/v1/external/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		api(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &tokenCalls
}

func newTestClient(t *testing.T, srv *httptest.Server, project string) *Client {
	t.Helper()
	c, err := New(Config{APIURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret", ProjectID: project})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTokenIsCachedAndProjectHeaderSent(t *testing.T) {
	var gotProjects []string
	srv, tokenCalls := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotProjects = append(gotProjects, r.Header.Get("x-project-id"))
		_, _ = w.Write([]byte(`{"id":"exc-1","name":"n"}`))
	})
	c := newTestClient(t, srv, "default-prj")
	ctx := context.Background()
	if _, err := c.GetExclusion(ctx, "", "exc-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetExclusion(ctx, "override", "exc-1"); err != nil {
		t.Fatal(err)
	}
	if *tokenCalls != 1 {
		t.Fatalf("token fetched %d times, want 1", *tokenCalls)
	}
	if gotProjects[0] != "default-prj" || gotProjects[1] != "override" {
		t.Fatalf("project headers = %v", gotProjects)
	}
}

func TestBadCredentials(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	c, _ := New(Config{APIURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "wrong"})
	_, err := c.GetExclusion(context.Background(), "", "x")
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "wrong") {
		t.Fatalf("unexpected error (must be 401 and must not leak the secret): %v", err)
	}
}

func TestNotFound(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"message":"Exclusion not found"}`))
	})
	_, err := newTestClient(t, srv, "").GetExclusion(context.Background(), "", "missing")
	if !IsNotFound(err) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestPagination(t *testing.T) {
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		p, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if r.URL.Query().Get("severity") != "High,Critical" {
			t.Errorf("filters not forwarded: %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": "g" + strconv.Itoa(p), "catalogId": "c" + strconv.Itoa(p), "name": "n", "severity": "High", "status": "Active"}},
			"page": map[string]any{"size": 1, "number": p, "totalElements": 3, "totalPages": 3},
		})
	})
	q := map[string][]string{"severity": {"High,Critical"}}
	list, err := newTestClient(t, srv, "").ListGuardrails(context.Background(), "", q)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[2].CatalogID != "c2" {
		t.Fatalf("got %+v", list)
	}
}

func TestPolicyContentStringOrObject(t *testing.T) {
	for _, body := range []string{`{"content":"{\"Version\":\"2012-10-17\"}"}`, `{"content":{"Version":"2012-10-17"}}`} {
		srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) })
		got, err := newTestClient(t, srv, "").GetSecurityControlPolicy(context.Background(), "", "sc-1", false)
		if err != nil {
			t.Fatal(err)
		}
		if got != `{"Version":"2012-10-17"}` {
			t.Fatalf("body %s -> %q", body, got)
		}
	}
}

func TestJWTExpiryFallback(t *testing.T) {
	// header.{"exp":2000000000}.sig
	tok := "eyJhbGciOiJub25lIn0.eyJleHAiOjIwMDAwMDAwMDB9.sig"
	if got := jwtExpiry(tok).Unix(); got != 2000000000 {
		t.Fatalf("exp = %d", got)
	}
}

func TestProjectsNeverSendProjectHeader(t *testing.T) {
	var got []string
	srv, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("x-project-id"))
		_, _ = w.Write([]byte(`{"id":"p1","name":"n","data":[],"page":{"totalPages":1}}`))
	})
	c := newTestClient(t, srv, "default-prj")
	ctx := context.Background()
	if _, err := c.GetProject(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListProjects(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetExclusion(ctx, "", "e1"); err != nil {
		t.Fatal(err)
	}
	if got[0] != "" || got[1] != "" || got[2] != "default-prj" {
		t.Fatalf("x-project-id headers = %q (projects must omit it, exclusions must keep it)", got)
	}
}

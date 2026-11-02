// Package client is a thin HTTP client for the Blast Security External API.
package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAPIURL   = "https://api.blast.security"
	DefaultTokenURL = "https://auth.blast.security/auth/v1/token"
	basePath        = "/api/v1/external"
	projectHeader   = "x-project-id"
)

// Config holds everything needed to talk to the API.
type Config struct {
	APIURL       string
	TokenURL     string
	ClientID     string
	ClientSecret string
	ProjectID    string // default x-project-id; empty = "All Cloud Units"
	UserAgent    string
	HTTPClient   *http.Client
}

// Client is safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client

	mu     sync.Mutex
	token  string
	expiry time.Time
}

// APIError is returned for any non-2xx response.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("blast api: %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, strings.TrimSpace(e.Body))
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func New(cfg Config) (*Client, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("client_id and client_secret are required")
	}
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = DefaultTokenURL
	}
	cfg.APIURL = strings.TrimRight(cfg.APIURL, "/")
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{cfg: cfg, http: hc}, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

// accessToken returns a cached token, refreshing it 60s before expiry.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expiry.Add(-60*time.Second)) {
		return c.token, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("blast auth: requesting token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Never echo the request (it contains the secret); the response body is safe to show.
		return "", fmt.Errorf("blast auth: token endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil || tr.AccessToken == "" {
		return "", fmt.Errorf("blast auth: unexpected token response (no access_token)")
	}

	c.token = tr.AccessToken
	switch {
	case tr.ExpiresIn > 0:
		c.expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	case jwtExpiry(tr.AccessToken) != (time.Time{}):
		c.expiry = jwtExpiry(tr.AccessToken)
	default:
		c.expiry = time.Now().Add(5 * time.Minute)
	}
	return c.token, nil
}

// jwtExpiry reads the exp claim without verifying the signature (the API verifies it).
func jwtExpiry(tok string) time.Time {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(raw, &claims) != nil || claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}

// RequestOptions are per-call overrides.
type RequestOptions struct {
	ProjectID string // overrides the provider default when non-empty
	NoProject bool   // tenant-scoped endpoints: never send x-project-id
	Query     url.Values
}

// Do performs an authenticated JSON request against /api/v1/external{path}.
func (c *Client) Do(ctx context.Context, method, path string, opts RequestOptions, in, out any) error {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return err
	}

	u := c.cfg.APIURL + basePath + path
	if len(opts.Query) > 0 {
		u += "?" + opts.Query.Encode()
	}

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	project := opts.ProjectID
	if project == "" {
		project = c.cfg.ProjectID
	}
	if project != "" && !opts.NoProject {
		req.Header.Set(projectHeader, project)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("blast api: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Method: method, Path: path, Body: string(respBody)}
	}
	if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("blast api: decoding %s %s response: %w", method, path, err)
		}
	}
	return nil
}

// Paginate fetches every page of a list endpoint. fetch returns the page items count and totalPages.
func Paginate(ctx context.Context, max int, fetch func(page int) (n int, totalPages int, err error)) error {
	for page := 0; ; page++ {
		n, total, err := fetch(page)
		if err != nil {
			return err
		}
		if n == 0 || page+1 >= total || (max > 0 && page+1 >= max) {
			return nil
		}
	}
}

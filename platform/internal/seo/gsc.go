package seo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GSCRow is one row from the Search Console Analytics API.
type GSCRow struct {
	Page        string
	Query       string
	Date        string
	Impressions int
	Clicks      int
	CTR         float64
	AvgPosition float64
}

// GSCClient is a minimal Search Console client. It reads credentials from a
// service-account JSON file provided by Cowork. When no credentials are present,
// Configured() returns false and FetchSiteMetrics returns ErrNotConfigured.
type GSCClient struct {
	credentialsPath string
	client          *http.Client
}

// NewGSCClient reads GSC_CREDENTIALS_JSON env var.
func NewGSCClient(credsPath string) *GSCClient {
	return &GSCClient{credentialsPath: credsPath, client: &http.Client{Timeout: 30 * time.Second}}
}

// ErrNotConfigured is returned when the GSC client has no credentials.
var ErrNotConfigured = errors.New("gsc: not configured")

// Configured reports whether the client can make API calls.
func (g *GSCClient) Configured() bool {
	if g == nil || g.credentialsPath == "" {
		return false
	}
	_, err := os.Stat(g.credentialsPath)
	return err == nil
}

// FetchSiteMetrics returns rows for the last N days for a given domain.
// The current implementation uses a service-account JWT to obtain a token,
// then calls the Search Analytics query endpoint.
func (g *GSCClient) FetchSiteMetrics(ctx context.Context, domain string, days int) ([]GSCRow, error) {
	if !g.Configured() {
		return nil, ErrNotConfigured
	}
	// Load service account.
	saJSON, err := os.ReadFile(g.credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal(saJSON, &sa); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	token, err := g.fetchAccessToken(ctx, sa.ClientEmail, sa.PrivateKey, sa.TokenURI)
	if err != nil {
		return nil, fmt.Errorf("access token: %w", err)
	}

	endTime := time.Now().UTC()
	startTime := endTime.AddDate(0, 0, -days)
	body := map[string]any{
		"startDate":  startTime.Format("2006-01-02"),
		"endDate":    endTime.Format("2006-01-02"),
		"dimensions": []string{"page", "query", "date"},
		"rowLimit":   1000,
	}
	b, _ := json.Marshal(body)

	siteURL := url.PathEscape("sc-domain:" + domain)
	apiURL := "https://searchconsole.googleapis.com/webmasters/v3/sites/" + siteURL + "/searchAnalytics/query"
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("content-type", "application/json")
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gsc api %d", resp.StatusCode)
	}
	var out struct {
		Rows []struct {
			Keys        []string `json:"keys"`
			Clicks      float64  `json:"clicks"`
			Impressions float64  `json:"impressions"`
			CTR         float64  `json:"ctr"`
			Position    float64  `json:"position"`
		} `json:"rows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	rows := make([]GSCRow, 0, len(out.Rows))
	for _, r := range out.Rows {
		if len(r.Keys) < 3 {
			continue
		}
		rows = append(rows, GSCRow{
			Page:        r.Keys[0],
			Query:       r.Keys[1],
			Date:        r.Keys[2],
			Impressions: int(r.Impressions),
			Clicks:      int(r.Clicks),
			CTR:         r.CTR,
			AvgPosition: r.Position,
		})
	}
	return rows, nil
}

// fetchAccessToken does a JWT-bearer flow against Google's token endpoint.
// Kept minimal — zero external deps.
func (g *GSCClient) fetchAccessToken(ctx context.Context, clientEmail, privateKey, tokenURI string) (string, error) {
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}
	jwt, err := buildServiceAccountJWT(clientEmail, privateKey, "https://www.googleapis.com/auth/webmasters.readonly")
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", jwt)
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("token endpoint %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.AccessToken, nil
}

// extractSlug pulls the article slug from a full page URL.
func extractSlug(pageURL, domain string) string {
	u, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return ""
	}
	// Only the first segment is the article slug for our layout.
	if idx := strings.Index(path, "/"); idx >= 0 {
		path = path[:idx]
	}
	return path
}

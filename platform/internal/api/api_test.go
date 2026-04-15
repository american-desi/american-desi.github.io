package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/american-desi/platform/internal/config"
	"github.com/american-desi/platform/internal/db"
	"github.com/american-desi/platform/internal/logging"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "api_test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	cfg := &config.Config{HTTPAddr: ":0", DataDir: dir, DBPath: filepath.Join(dir, "api_test.db"), SitesDir: filepath.Join(dir, "sites"), DashboardDir: ""}
	s := &Server{
		Config: cfg,
		Log:    logging.New(),
		DB:     d,
	}
	return s, dir
}

func TestHealthz(t *testing.T) {
	s, _ := newTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("healthz status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out["status"] != "ok" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestListSitesEmpty(t *testing.T) {
	s, _ := newTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sites")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "null" && !strings.HasPrefix(strings.TrimSpace(string(body)), "[") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestReadyz(t *testing.T) {
	s, _ := newTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestRequestIDEcho(t *testing.T) {
	s, _ := newTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", ts.URL+"/healthz", nil)
	req.Header.Set("X-Request-ID", "rid-test-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Request-ID") != "rid-test-123" {
		t.Fatalf("request id not echoed: %q", resp.Header.Get("X-Request-ID"))
	}
}

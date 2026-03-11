package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type testHarness struct {
	app    *App
	server *httptest.Server
	client *http.Client
	cancel context.CancelFunc
}

func newTestHarness(t *testing.T, fakeDocker bool) *testHarness {
	t.Helper()
	return newTestHarnessWithConfig(t, fakeDocker, nil)
}

func newTestHarnessWithConfig(t *testing.T, fakeDocker bool, mutate func(*Config)) *testHarness {
	t.Helper()

	tmpDir := t.TempDir()
	containerRoot := filepath.Join(tmpDir, "containers")
	if err := os.MkdirAll(containerRoot, 0o755); err != nil {
		t.Fatalf("mkdir container root: %v", err)
	}

	if fakeDocker {
		setupFakeDocker(t, tmpDir)
	}

	cfg := Config{
		Port:                 "0",
		DBPath:               filepath.Join(tmpDir, "data", "app.db"),
		DiunWebhookSecret:    "",
		DefaultCooldown:      600,
		ContainerRoot:        containerRoot,
		AdminUsername:        "admin",
		AdminPassword:        "test-password",
		SessionTTLSeconds:    3600,
		RememberMeTTLSeconds: 30 * 24 * 3600,
		APITimeoutSeconds:    10,
		SSEMaxConnections:    20,
		PullRetryMaxAttempts: 3,
		PullRetryDelaySec:    1,
		AutoWindowStart:      -1,
		AutoWindowEnd:        -1,
		LoginRateLimit: LoginRateLimiterConfig{
			WindowSeconds: 600,
			MaxAttempts:   5,
			LockSeconds:   900,
		},
	}
	if mutate != nil {
		mutate(&cfg)
	}

	db, err := openDB(cfg.DBPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	app := &App{
		cfg:              cfg,
		db:               db,
		logBroker:        newLogBroker(),
		dashboardBroker:  newDashboardEventBroker(),
		queue:            make(chan int64, 128),
		sessions:         newSessionStore(time.Duration(cfg.SessionTTLSeconds) * time.Second),
		sseLimiter:       newSSELimiter(cfg.SSEMaxConnections),
		dashboardLimiter: newSSELimiter(cfg.DashboardSSEMax),
		loginLimiter:     newLoginRateLimiter(cfg.LoginRateLimit),
		pushDedupe:       map[string]int64{},
		assetVersion:     "test-asset-version",
	}
	if err := app.initAuthAndWebhookSecret(context.Background()); err != nil {
		t.Fatalf("init auth/webhook secret: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go app.worker(ctx)

	mux := http.NewServeMux()
	app.routes(mux)
	server := httptest.NewServer(mux)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	h := &testHarness{
		app:    app,
		server: server,
		client: client,
		cancel: cancel,
	}
	t.Cleanup(func() {
		h.cancel()
		h.server.Close()
		_ = h.app.db.Close()
	})
	return h
}

func setupFakeDocker(t *testing.T, root string) {
	t.Helper()

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	script := `#!/bin/sh
if [ "${DOCKER_SLEEP_SEC}" != "" ]; then
  sleep "${DOCKER_SLEEP_SEC}"
fi
echo "fake docker $*"
if [ "${DOCKER_FAIL_ONCE}" = "1" ]; then
  state_file="${DOCKER_STATE_FILE:-/tmp/docker-fail-once.state}"
  if [ ! -f "$state_file" ]; then
    touch "$state_file"
    msg="${DOCKER_FAIL_ONCE_MESSAGE:-forced failure once}"
    echo "$msg" >&2
    exit 1
  fi
fi
if [ "${DOCKER_FORCE_FAIL}" = "1" ]; then
  echo "forced failure" >&2
  exit 1
fi
exit 0
`
	dockerPath := filepath.Join(binDir, "docker")
	if err := os.WriteFile(dockerPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("DOCKER_SLEEP_SEC", "0.15")
	t.Setenv("DOCKER_FAIL_ONCE", "0")
	t.Setenv("DOCKER_FAIL_ONCE_MESSAGE", "")
	t.Setenv("DOCKER_STATE_FILE", filepath.Join(root, "docker_fail_once.state"))
}

func doJSONRequest(t *testing.T, client *http.Client, method, url string, body any, headers map[string]string) (*http.Response, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp, raw
}

func doRawRequest(t *testing.T, client *http.Client, method, url, rawBody string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(rawBody))
	if err != nil {
		t.Fatalf("new raw request: %v", err)
	}
	if strings.TrimSpace(rawBody) != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do raw request: %v", err)
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read raw response body: %v", err)
	}
	return resp, raw
}

func loginAsAdmin(t *testing.T, h *testHarness) {
	t.Helper()
	resp, _ := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/login", map[string]any{
		"username": "admin",
		"password": "test-password",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func readSSEEvent(r *bufio.Reader) (string, string, error) {
	eventName := ""
	data := ""
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if eventName == "" && data == "" {
				continue
			}
			return eventName, data, nil
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			chunk := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				data = chunk
			} else {
				data += "\n" + chunk
			}
		}
	}
}

func TestIntegrationUnauthorizedMutationBlocked(t *testing.T) {
	h := newTestHarness(t, false)
	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/targets", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/targets status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	var apiErr apiErrorResponse
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if apiErr.Code != "login_required" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":        "app1",
		"compose_dir": "/not-used",
		"image_repo":  "ghcr.io/acme/app1",
	}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if apiErr.Code != "login_required" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/containers/discover", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /api/containers/discover status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if apiErr.Code != "login_required" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}
}

func TestIntegrationDiscoverContainers(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	app1Dir := filepath.Join(h.app.cfg.ContainerRoot, "app1")
	if err := os.MkdirAll(app1Dir, 0o755); err != nil {
		t.Fatalf("mkdir app1 dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(app1Dir, "docker-compose.yml"), []byte(`
services:
  web:
    image: ghcr.io/acme/web:latest
  worker:
    image: "ghcr.io/acme/worker:1.2.3"
`), 0o644); err != nil {
		t.Fatalf("write app1 compose: %v", err)
	}

	app2Dir := filepath.Join(h.app.cfg.ContainerRoot, "app2")
	if err := os.MkdirAll(app2Dir, 0o755); err != nil {
		t.Fatalf("mkdir app2 dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(app2Dir, "compose.yaml"), []byte(`
services:
  api:
    image: ghcr.io/acme/api:stable
`), 0o644); err != nil {
		t.Fatalf("write app2 compose: %v", err)
	}

	ignoredDir := filepath.Join(h.app.cfg.ContainerRoot, "no-compose")
	if err := os.MkdirAll(ignoredDir, 0o755); err != nil {
		t.Fatalf("mkdir ignored dir: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/containers/discover", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("discover status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var out struct {
		Items []DiscoveredContainer `json:"items"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal discover response: %v", err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("discovered len = %d, want 2", len(out.Items))
	}
	if out.Items[0].Name != "app1" || out.Items[1].Name != "app2" {
		t.Fatalf("unexpected discover ordering/items: %+v", out.Items)
	}
	if out.Items[0].ComposeFile != "docker-compose.yml" {
		t.Fatalf("app1 compose file = %q", out.Items[0].ComposeFile)
	}
	if len(out.Items[0].ImageRepoCandidates) != 2 {
		t.Fatalf("app1 image repos = %+v", out.Items[0].ImageRepoCandidates)
	}
	if out.Items[0].ImageRepoCandidates[0] != "ghcr.io/acme/web" || out.Items[0].ImageRepoCandidates[1] != "ghcr.io/acme/worker" {
		t.Fatalf("unexpected app1 repos: %+v", out.Items[0].ImageRepoCandidates)
	}
	if len(out.Items[1].ImageRepoCandidates) != 1 || out.Items[1].ImageRepoCandidates[0] != "ghcr.io/acme/api" {
		t.Fatalf("unexpected app2 repos: %+v", out.Items[1].ImageRepoCandidates)
	}
}

func TestIntegrationDiscoverContainersWithComposeVariables(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	appDir := filepath.Join(h.app.cfg.ContainerRoot, "app-var")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("mkdir app-var dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, ".env"), []byte(`
API_REPO=ghcr.io/acme/api
API_TAG=1.0.0
`), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "docker-compose.yml"), []byte(`
services:
  api:
    image: ${API_REPO}:${API_TAG}
  worker:
    image: ${WORKER_REPO:-ghcr.io/acme/worker}:${WORKER_TAG-default}
  skipped:
    image: ${MISSING_REPO}
`), 0o644); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	t.Setenv("API_TAG", "9.9.9")

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/containers/discover", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("discover status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var out struct {
		Items []DiscoveredContainer `json:"items"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal discover response: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("discovered len = %d, want 1", len(out.Items))
	}
	if out.Items[0].Name != "app-var" {
		t.Fatalf("unexpected discovered item: %+v", out.Items[0])
	}
	wantRepos := []string{"ghcr.io/acme/api", "ghcr.io/acme/worker"}
	if len(out.Items[0].ImageRepoCandidates) != len(wantRepos) {
		t.Fatalf("unexpected repos: %+v", out.Items[0].ImageRepoCandidates)
	}
	for i := range wantRepos {
		if out.Items[0].ImageRepoCandidates[i] != wantRepos[i] {
			t.Fatalf("repo[%d] = %q, want %q", i, out.Items[0].ImageRepoCandidates[i], wantRepos[i])
		}
	}
}

func TestIntegrationRootPageBySession(t *testing.T) {
	h := newTestHarness(t, false)

	noFollowClient := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := noFollowClient.Get(h.server.URL + "/")
	if err != nil {
		t.Fatalf("get root unauthenticated: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("unauth root status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Fatalf("unauth root redirect location = %q, want %q", loc, "/login")
	}
	resp.Body.Close()

	resp, err = h.client.Get(h.server.URL + "/login")
	if err != nil {
		t.Fatalf("get login page: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login page status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := string(raw)
	if !strings.Contains(body, `id="login-btn"`) {
		t.Fatalf("unauth /login should serve login page")
	}

	loginAsAdmin(t, h)
	resp, err = h.client.Get(h.server.URL + "/")
	if err != nil {
		t.Fatalf("get root authenticated: %v", err)
	}
	raw, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth root status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body = string(raw)
	if !strings.Contains(body, `id="run-update"`) {
		t.Fatalf("auth root should serve dashboard page")
	}
}

func TestIntegrationAuthLoginFlow(t *testing.T) {
	h := newTestHarness(t, false)

	resp, _ := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/login", map[string]any{
		"username": "admin",
		"password": "wrong-password",
	}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong login status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	loginAsAdmin(t, h)

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/auth/me", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var me authMeResponse
	if err := json.Unmarshal(raw, &me); err != nil {
		t.Fatalf("unmarshal me: %v", err)
	}
	if !me.Authenticated || me.Username != "admin" {
		t.Fatalf("unexpected me response: %+v", me)
	}

	resp, _ = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/logout", map[string]any{}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/auth/me", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me-after-logout status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if err := json.Unmarshal(raw, &me); err != nil {
		t.Fatalf("unmarshal me after logout: %v", err)
	}
	if me.Authenticated {
		t.Fatalf("expected unauthenticated after logout: %+v", me)
	}
}

func TestIntegrationAuthLoginRateLimit(t *testing.T) {
	h := newTestHarnessWithConfig(t, false, func(cfg *Config) {
		cfg.LoginRateLimit = LoginRateLimiterConfig{
			WindowSeconds: 600,
			MaxAttempts:   5,
			LockSeconds:   1,
		}
	})

	for i := 0; i < 5; i++ {
		resp, _ := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/login", map[string]any{
			"username": "admin",
			"password": "wrong-password",
		}, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("wrong login #%d status = %d, want %d", i+1, resp.StatusCode, http.StatusUnauthorized)
		}
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/login", map[string]any{
		"username": "admin",
		"password": "wrong-password",
	}, nil)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("rate-limited login status = %d, want %d body=%s", resp.StatusCode, http.StatusTooManyRequests, string(raw))
	}
	var out authRateLimitedResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal rate-limited response: %v", err)
	}
	if out.Code != "auth_rate_limited" {
		t.Fatalf("unexpected rate limit response code: %+v", out)
	}
	if out.RetryAfterSeconds <= 0 {
		t.Fatalf("retry_after_seconds should be > 0, got %d", out.RetryAfterSeconds)
	}
	if got := strings.TrimSpace(resp.Header.Get("Retry-After")); got == "" {
		t.Fatal("expected Retry-After header on rate-limited login")
	}

	time.Sleep(1200 * time.Millisecond)

	resp, _ = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/login", map[string]any{
		"username": "admin",
		"password": "test-password",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login after lock expiration status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, _ = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/login", map[string]any{
		"username": "admin",
		"password": "wrong-password",
	}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("post-success wrong login should reset limiter; status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestIntegrationAuthRememberMeTTL(t *testing.T) {
	h := newTestHarnessWithConfig(t, false, func(cfg *Config) {
		cfg.SessionTTLSeconds = 120
		cfg.RememberMeTTLSeconds = 3600
	})

	resp, _ := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/login", map[string]any{
		"username": "admin",
		"password": "test-password",
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("normal login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var normalCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName {
			normalCookie = c
			break
		}
	}
	if normalCookie == nil {
		t.Fatal("normal login did not set session cookie")
	}

	resp, _ = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/auth/login", map[string]any{
		"username":    "admin",
		"password":    "test-password",
		"remember_me": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("remember login status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var rememberCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName {
			rememberCookie = c
			break
		}
	}
	if rememberCookie == nil {
		t.Fatal("remember login did not set session cookie")
	}
	if rememberCookie.MaxAge <= normalCookie.MaxAge {
		t.Fatalf("remember max-age should be longer: normal=%d remember=%d", normalCookie.MaxAge, rememberCookie.MaxAge)
	}
	if rememberCookie.MaxAge < 3000 {
		t.Fatalf("remember max-age too short: %d", rememberCookie.MaxAge)
	}
}

func TestIntegrationMetricsIncludesLoginAndWebhookBreakdown(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	now := time.Now().Unix()
	if err := h.app.recordAuthLoginEvent(context.Background(), now-10, false, false, "admin", "127.0.0.1"); err != nil {
		t.Fatalf("record auth login failure: %v", err)
	}
	if err := h.app.recordAuthLoginEvent(context.Background(), now-9, false, true, "admin", "127.0.0.1"); err != nil {
		t.Fatalf("record auth login limited: %v", err)
	}
	if err := h.app.recordWebhookReceipt(context.Background(), now-8, false, http.StatusUnauthorized, webhookReasonSecretMismatch, "invalid secret", nil); err != nil {
		t.Fatalf("record webhook failed receipt: %v", err)
	}
	if err := h.app.recordWebhookReceipt(context.Background(), now-7, true, http.StatusOK, webhookReasonNoMatch, "", nil); err != nil {
		t.Fatalf("record webhook success receipt: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/metrics", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var m Metrics
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal metrics: %v", err)
	}
	if m.LoginFailures24h != 1 {
		t.Fatalf("login_failures_24h = %d, want 1", m.LoginFailures24h)
	}
	if m.LoginRateLimited24h != 1 {
		t.Fatalf("login_rate_limited_24h = %d, want 1", m.LoginRateLimited24h)
	}
	if m.WebhookTotalLast24h != 2 {
		t.Fatalf("webhook_total_24h = %d, want 2", m.WebhookTotalLast24h)
	}
	if m.WebhookFailures24h != 1 {
		t.Fatalf("webhook_failures_24h = %d, want 1", m.WebhookFailures24h)
	}
}

func TestIntegrationMetricsIncludesDashboardAndPushBreakdown(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	now := time.Now().Unix()
	if err := h.app.recordPushDelivery(context.Background(), true, "", now-5); err != nil {
		t.Fatalf("record push success delivery: %v", err)
	}
	if err := h.app.recordPushDelivery(context.Background(), false, "send failed", now-4); err != nil {
		t.Fatalf("record push failed delivery: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/metrics", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var m Metrics
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal metrics: %v", err)
	}
	if m.PushSent24h != 1 {
		t.Fatalf("push_sent_24h = %d, want 1", m.PushSent24h)
	}
	if m.PushFailed24h != 1 {
		t.Fatalf("push_failed_24h = %d, want 1", m.PushFailed24h)
	}
	if m.DashboardStreamActive < 0 {
		t.Fatalf("dashboard_stream_active should be >= 0, got %d", m.DashboardStreamActive)
	}
	if m.DashboardStreamRejectedTotal < 0 {
		t.Fatalf("dashboard_stream_rejected_total should be >= 0, got %d", m.DashboardStreamRejectedTotal)
	}
}

func TestIntegrationMetricsToleratesMissingOptionalTelemetryTables(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	for _, stmt := range []string{
		`DROP TABLE webhook_receipts`,
		`DROP TABLE auth_login_events`,
		`DROP TABLE push_delivery_logs`,
	} {
		if _, err := h.app.db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/metrics", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var m Metrics
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal metrics: %v", err)
	}
	if m.WebhookTotalLast24h != 0 || m.WebhookFailures24h != 0 {
		t.Fatalf("webhook metrics = (%d,%d), want zero fallback", m.WebhookTotalLast24h, m.WebhookFailures24h)
	}
	if m.LoginFailures24h != 0 || m.LoginRateLimited24h != 0 {
		t.Fatalf("login metrics = (%d,%d), want zero fallback", m.LoginFailures24h, m.LoginRateLimited24h)
	}
	if m.PushSent24h != 0 || m.PushFailed24h != 0 {
		t.Fatalf("push metrics = (%d,%d), want zero fallback", m.PushSent24h, m.PushFailed24h)
	}
}

func TestIntegrationDashboardStreamAuthAndPatch(t *testing.T) {
	h := newTestHarness(t, false)

	resp, _ := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/stream/dashboard", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth dashboard stream status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	loginAsAdmin(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.server.URL+"/api/stream/dashboard", nil)
	if err != nil {
		t.Fatalf("new stream request: %v", err)
	}
	streamResp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("stream request failed: %v", err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(streamResp.Body)
		t.Fatalf("dashboard stream status = %d, want %d body=%s", streamResp.StatusCode, http.StatusOK, string(raw))
	}

	reader := bufio.NewReader(streamResp.Body)
	eventName, data, err := readSSEEvent(reader)
	if err != nil {
		t.Fatalf("read initial sse event: %v", err)
	}
	if eventName != "patch" {
		t.Fatalf("initial event name = %q, want patch", eventName)
	}
	var initial DashboardPatchEvent
	if err := json.Unmarshal([]byte(data), &initial); err != nil {
		t.Fatalf("unmarshal initial patch: %v", err)
	}
	if initial.EventType != "snapshot" {
		t.Fatalf("initial event type = %q, want snapshot", initial.EventType)
	}

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "stream-app")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(composeDir, "docker-compose.yml"), []byte("services:\n  app:\n    image: ghcr.io/acme/stream:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "stream-app",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/stream",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}

	seenTargetPatch := false
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		evName, evData, readErr := readSSEEvent(reader)
		if readErr != nil {
			t.Fatalf("read stream patch event: %v", readErr)
		}
		if evName != "patch" {
			continue
		}
		var patch DashboardPatchEvent
		if err := json.Unmarshal([]byte(evData), &patch); err != nil {
			t.Fatalf("unmarshal patch event: %v", err)
		}
		if patch.EventType != eventTypeTargetCreated {
			continue
		}
		for _, section := range patch.Sections {
			if section == dashboardSectionTargets {
				seenTargetPatch = true
				break
			}
		}
		if seenTargetPatch {
			break
		}
	}
	if !seenTargetPatch {
		t.Fatal("expected target_created dashboard patch with targets section")
	}
}

func TestIntegrationLoginPageServesVersionedAssets(t *testing.T) {
	h := newTestHarness(t, false)

	resp, err := h.client.Get(h.server.URL + "/login")
	if err != nil {
		t.Fatalf("get login page: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login page status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("login page cache-control = %q, want no-store", got)
	}
	body := string(raw)
	if !strings.Contains(body, "/styles.css?v=test-asset-version") {
		t.Fatalf("login page missing versioned stylesheet: %s", body)
	}
	if !strings.Contains(body, "/login.js?v=test-asset-version") {
		t.Fatalf("login page missing versioned login.js: %s", body)
	}
}

func TestIntegrationDashboardPageServesVersionedAssets(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	resp, err := h.client.Get(h.server.URL + "/")
	if err != nil {
		t.Fatalf("get dashboard page: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dashboard page status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("dashboard page cache-control = %q, want no-store", got)
	}
	body := string(raw)
	if !strings.Contains(body, `data-asset-version="test-asset-version"`) {
		t.Fatalf("dashboard page missing asset version bootstrap: %s", body)
	}
	if !strings.Contains(body, "/app.js?v=test-asset-version") {
		t.Fatalf("dashboard page missing versioned app.js: %s", body)
	}
}

func TestIntegrationPushSubscriptionEndpoints(t *testing.T) {
	h := newTestHarnessWithConfig(t, false, func(cfg *Config) {
		cfg.WebPushEnabled = true
		cfg.WebPushVAPIDPublic = "BCV5S0mXPU8J10j1h7zJ5o3nWACW0hseFKo-yotXKz2I6tA50f6ogFT8f58jdMqVY74w26X43ziOmA3fJkg5fJw"
		cfg.WebPushVAPIDPrivate = "yl2O5dW3TQX0_lqNw-FA_9dmjWJyrm9FzaYXf8f5cpo"
		cfg.WebPushSubject = "mailto:test@example.com"
	})

	resp, _ := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/push/config", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth push config status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	loginAsAdmin(t, h)
	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/push/config", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push config status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var cfg PushConfigResponse
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal push config: %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("expected push enabled")
	}
	if cfg.Subscribed {
		t.Fatal("expected push unsubscribed initially")
	}
	if cfg.HasAnySubscriptions {
		t.Fatal("expected has_any_subscriptions=false initially")
	}

	endpoint := "https://push.example.test/subscription-1"
	payload := map[string]any{
		"endpoint": endpoint,
		"keys": map[string]any{
			"p256dh": "test-p256dh",
			"auth":   "test-auth",
		},
	}
	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/push/subscriptions", payload, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push subscribe status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/push/config", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push config after subscribe status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal push config after subscribe: %v", err)
	}
	if cfg.Subscribed {
		t.Fatal("expected push subscribed=false without endpoint query")
	}
	if !cfg.HasAnySubscriptions {
		t.Fatal("expected has_any_subscriptions=true after upsert")
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/push/config?endpoint="+url.QueryEscape(endpoint), nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push config by endpoint status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal push config by endpoint: %v", err)
	}
	if !cfg.Subscribed {
		t.Fatal("expected endpoint subscribed=true")
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/push/config?endpoint="+url.QueryEscape("https://push.example.test/other-device"), nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push config by other endpoint status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal push config by other endpoint: %v", err)
	}
	if cfg.Subscribed {
		t.Fatal("expected other endpoint subscribed=false")
	}
	if !cfg.HasAnySubscriptions {
		t.Fatal("expected has_any_subscriptions=true for other endpoint query")
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodDelete, h.server.URL+"/api/push/subscriptions?endpoint="+url.QueryEscape(endpoint), nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push unsubscribe status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/push/config", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push config after unsubscribe status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal push config after unsubscribe: %v", err)
	}
	if cfg.Subscribed {
		t.Fatal("expected push subscribed=false after disable")
	}
}

func TestIntegrationPushDisabledRejectsSubscriptionMutation(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/push/subscriptions", map[string]any{
		"endpoint": "https://push.example.test/subscription-disabled",
		"keys": map[string]any{
			"p256dh": "x",
			"auth":   "y",
		},
	}, nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("push subscribe disabled status = %d, want %d body=%s", resp.StatusCode, http.StatusServiceUnavailable, string(raw))
	}
	var apiErr apiErrorResponse
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("unmarshal api error: %v", err)
	}
	if apiErr.Code != "push_disabled" {
		t.Fatalf("push disabled code = %q, want push_disabled", apiErr.Code)
	}
}

func TestIntegrationPushDisableAllWithoutEndpoint(t *testing.T) {
	h := newTestHarnessWithConfig(t, false, func(cfg *Config) {
		cfg.WebPushEnabled = true
		cfg.WebPushVAPIDPublic = "BCV5S0mXPU8J10j1h7zJ5o3nWACW0hseFKo-yotXKz2I6tA50f6ogFT8f58jdMqVY74w26X43ziOmA3fJkg5fJw"
		cfg.WebPushVAPIDPrivate = "yl2O5dW3TQX0_lqNw-FA_9dmjWJyrm9FzaYXf8f5cpo"
		cfg.WebPushSubject = "mailto:test@example.com"
	})
	loginAsAdmin(t, h)

	// Register two subscriptions.
	for i := 1; i <= 2; i++ {
		resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/push/subscriptions", map[string]any{
			"endpoint": "https://push.example.test/subscription-disable-all-" + strconv.Itoa(i),
			"keys": map[string]any{
				"p256dh": "p256dh-" + strconv.Itoa(i),
				"auth":   "auth-" + strconv.Itoa(i),
			},
		}, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("create push subscription #%d status = %d, want %d body=%s", i, resp.StatusCode, http.StatusOK, string(raw))
		}
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodDelete, h.server.URL+"/api/push/subscriptions", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete all push subscriptions status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal delete all response: %v", err)
	}
	if out["subscribed"] != false {
		t.Fatalf("subscribed = %v, want false", out["subscribed"])
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/push/config", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("push config after delete all status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var cfg PushConfigResponse
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal push config: %v", err)
	}
	if cfg.Subscribed {
		t.Fatal("expected subscribed=false after delete-all")
	}
}

func TestIntegrationUpdateJobLifecycle(t *testing.T) {
	h := newTestHarness(t, true)
	loginAsAdmin(t, h)

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "app1")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: ghcr.io/acme/app1:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "app1",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/app1",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var target Target
	if err := json.Unmarshal(raw, &target); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/jobs/update", map[string]any{
		"target_ids": []int64{target.ID},
	}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create update job status = %d, want %d body=%s", resp.StatusCode, http.StatusAccepted, string(raw))
	}

	var queuedResp map[string]any
	if err := json.Unmarshal(raw, &queuedResp); err != nil {
		t.Fatalf("unmarshal queue response: %v", err)
	}
	if queuedResp["status"] != statusQueued {
		t.Fatalf("expected queued response status, got %v", queuedResp["status"])
	}
	jobID := int64(queuedResp["job_id"].(float64))

	deadline := time.Now().Add(10 * time.Second)
	seenRunning := false
	seenSuccess := false
	for time.Now().Before(deadline) {
		resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/jobs?limit=20", nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("list jobs status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var jobsResp struct {
			Jobs []Job `json:"jobs"`
		}
		if err := json.Unmarshal(raw, &jobsResp); err != nil {
			t.Fatalf("unmarshal jobs response: %v", err)
		}
		for _, job := range jobsResp.Jobs {
			if job.ID != jobID {
				continue
			}
			if job.Status == statusRunning {
				seenRunning = true
			}
			if job.Status == statusSuccess {
				seenSuccess = true
				if job.StartedAt == nil || job.EndedAt == nil {
					t.Fatalf("expected started_at and ended_at for completed job: %+v", job)
				}
			}
		}
		if seenSuccess {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !seenSuccess {
		t.Fatalf("job %d did not reach success state", jobID)
	}
	if !seenRunning {
		t.Fatalf("job %d never observed in running state", jobID)
	}
}

func TestIntegrationUpdateJobPullRetry(t *testing.T) {
	h := newTestHarness(t, true)
	loginAsAdmin(t, h)
	t.Setenv("DOCKER_FAIL_ONCE", "1")

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "app-retry")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: ghcr.io/acme/retry:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "app-retry",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/retry",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var target Target
	if err := json.Unmarshal(raw, &target); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/jobs/update", map[string]any{
		"target_ids": []int64{target.ID},
	}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create update job status = %d, want %d body=%s", resp.StatusCode, http.StatusAccepted, string(raw))
	}
	var queuedResp map[string]any
	if err := json.Unmarshal(raw, &queuedResp); err != nil {
		t.Fatalf("unmarshal queue response: %v", err)
	}
	jobID := int64(queuedResp["job_id"].(float64))

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/jobs?limit=20", nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("list jobs status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var jobsResp struct {
			Jobs []Job `json:"jobs"`
		}
		if err := json.Unmarshal(raw, &jobsResp); err != nil {
			t.Fatalf("unmarshal jobs response: %v", err)
		}
		for _, job := range jobsResp.Jobs {
			if job.ID != jobID {
				continue
			}
			if job.Status == statusSuccess {
				return
			}
			if job.Status == statusFailed || job.Status == statusBlocked {
				t.Fatalf("expected retry to recover pull failure, got status=%s", job.Status)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("job %d did not reach success state with retry", jobID)
}

func TestIntegrationUpdateJobPullRetryTooManyRequests(t *testing.T) {
	h := newTestHarnessWithConfig(t, true, func(cfg *Config) {
		cfg.PullRetryMaxAttempts = 3
		cfg.PullRetryDelaySec = 0
	})
	loginAsAdmin(t, h)
	t.Setenv("DOCKER_FAIL_ONCE", "1")
	t.Setenv("DOCKER_FAIL_ONCE_MESSAGE", "Error response from daemon: toomanyrequests: retry-after: 183.761µs, allowed: 44000/minute")

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "app-retry-rate")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: ghcr.io/acme/retry-rate:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "app-retry-rate",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/retry-rate",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var target Target
	if err := json.Unmarshal(raw, &target); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/jobs/update", map[string]any{
		"target_ids": []int64{target.ID},
	}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create update job status = %d, want %d body=%s", resp.StatusCode, http.StatusAccepted, string(raw))
	}
	var queuedResp map[string]any
	if err := json.Unmarshal(raw, &queuedResp); err != nil {
		t.Fatalf("unmarshal queue response: %v", err)
	}
	jobID := int64(queuedResp["job_id"].(float64))

	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/jobs?limit=20", nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("list jobs status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var jobsResp struct {
			Jobs []Job `json:"jobs"`
		}
		if err := json.Unmarshal(raw, &jobsResp); err != nil {
			t.Fatalf("unmarshal jobs response: %v", err)
		}
		for _, job := range jobsResp.Jobs {
			if job.ID != jobID {
				continue
			}
			if job.Status == statusSuccess {
				logs, err := h.app.getJobLogs(context.Background(), jobID)
				if err != nil {
					t.Fatalf("get job logs: %v", err)
				}
				foundRetryDelayLog := false
				for _, line := range logs {
					if strings.Contains(line, "retrying pull attempt 2/3 after 2s") {
						foundRetryDelayLog = true
						break
					}
				}
				if !foundRetryDelayLog {
					t.Fatalf("expected retry delay log after toomanyrequests; logs=%v", logs)
				}
				return
			}
			if job.Status == statusFailed || job.Status == statusBlocked {
				t.Fatalf("expected retry to recover toomanyrequests pull failure, got status=%s", job.Status)
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("job %d did not reach success state with retry", jobID)
}

func TestIntegrationWebhookSecretMismatchHandling(t *testing.T) {
	h := newTestHarness(t, false)
	if h.app.webhookSecret == "" {
		t.Fatal("expected auto-generated webhook secret")
	}

	resp, _ := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/diun/webhook/config", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("webhook config without login status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	loginAsAdmin(t, h)
	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/diun/webhook/config", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook config status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var cfg webhookConfigResponse
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal webhook config: %v", err)
	}
	if cfg.Secret != "" {
		t.Fatalf("webhook config should not expose full secret by default: %+v", cfg)
	}
	if cfg.SecretMasked == "" || cfg.SecretMasked == h.app.webhookSecret {
		t.Fatalf("unexpected webhook masked secret from config: %+v", cfg)
	}

	payload := map[string]any{
		"entry": map[string]any{
			"image": map[string]any{
				"name": "ghcr.io/acme/app1:latest",
			},
		},
	}

	resp, _ = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("webhook without secret status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	resp, _ = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": "wrong-secret",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("webhook wrong secret status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook valid secret status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
}

func TestIntegrationDiunReceiptsEndpoint(t *testing.T) {
	h := newTestHarness(t, true)

	resp, _ := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/diun/receipts", nil, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth receipts status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	loginAsAdmin(t, h)

	payload := map[string]any{
		"entry": map[string]any{
			"image": map[string]any{
				"name": "ghcr.io/acme/receipt-app:latest",
			},
		},
	}
	resp, _ = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": "wrong-secret",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("webhook wrong secret status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	resp, _ = doRawRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", "{invalid-json", map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("webhook invalid payload status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "receipt-app")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(composeDir, "docker-compose.yml"), []byte("services:\n  app:\n    image: ghcr.io/acme/receipt-app:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "receipt-app",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/receipt-app",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var target Target
	if err := json.Unmarshal(raw, &target); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPatch, h.server.URL+"/api/targets/"+strconv.FormatInt(target.ID, 10), map[string]any{
		"auto_update_enabled": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable auto update status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	resp, _ = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook queued status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/diun/receipts?limit=10", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("receipts status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var out struct {
		Receipts []WebhookReceiptSummary `json:"receipts"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal receipts response: %v", err)
	}
	if len(out.Receipts) < 3 {
		t.Fatalf("expected at least 3 receipts, got %d", len(out.Receipts))
	}

	foundReason := map[string]bool{
		webhookReasonSecretMismatch: false,
		webhookReasonPayloadInvalid: false,
		webhookReasonQueued:         false,
	}
	foundQueuedJobID := false
	for _, receipt := range out.Receipts {
		if _, ok := foundReason[receipt.ReasonCode]; ok {
			foundReason[receipt.ReasonCode] = true
		}
		if receipt.ReasonCode == webhookReasonQueued && receipt.QueuedJobID != nil && *receipt.QueuedJobID > 0 {
			foundQueuedJobID = true
		}
	}
	for code, ok := range foundReason {
		if !ok {
			t.Fatalf("missing webhook receipt reason code: %s", code)
		}
	}
	if !foundQueuedJobID {
		t.Fatal("expected queued receipt to include queued_job_id")
	}
}

func TestIntegrationWebhookMatchesAdditionalImageRepo(t *testing.T) {
	h := newTestHarness(t, true)
	loginAsAdmin(t, h)

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "app-multi")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(`
services:
  app:
    image: ghcr.io/acme/main:latest
  sidecar:
    image: ghcr.io/acme/sidecar:latest
`), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "app-multi",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/main",
		"image_repos":  []string{"ghcr.io/acme/main", "ghcr.io/acme/sidecar"},
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var created Target
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}
	if len(created.ImageRepos) != 2 {
		t.Fatalf("created target image_repos = %+v, want 2 repos", created.ImageRepos)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPatch, h.server.URL+"/api/targets/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"auto_update_enabled": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable auto update status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	payload := map[string]any{
		"entry": map[string]any{
			"image": map[string]any{
				"name": "ghcr.io/acme/sidecar:latest",
			},
		},
	}
	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var webhookResp diunWebhookResponse
	if err := json.Unmarshal(raw, &webhookResp); err != nil {
		t.Fatalf("unmarshal webhook response: %v", err)
	}
	if webhookResp.MatchedCount != 1 {
		t.Fatalf("matched_count = %d, want 1", webhookResp.MatchedCount)
	}
	if webhookResp.QueuedCount != 1 {
		t.Fatalf("queued_count = %d, want 1", webhookResp.QueuedCount)
	}
	if len(webhookResp.SkippedIDs) != 0 {
		t.Fatalf("skipped_ids = %+v, want empty", webhookResp.SkippedIDs)
	}
}

func TestIntegrationWebhookMatchesEntryImageStringPayload(t *testing.T) {
	h := newTestHarness(t, true)
	loginAsAdmin(t, h)

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "entry-image-string")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(`
services:
  app:
    image: ghcr.io/acme/string-entry:latest
`), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "entry-image-string",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/string-entry",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var created Target
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPatch, h.server.URL+"/api/targets/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"auto_update_enabled": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable auto update status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	payload := map[string]any{
		"entry": map[string]any{
			"image":  "ghcr.io/acme/string-entry:5.4.3",
			"tag":    "5.4.3",
			"digest": "sha256:11223344",
		},
	}
	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var webhookResp diunWebhookResponse
	if err := json.Unmarshal(raw, &webhookResp); err != nil {
		t.Fatalf("unmarshal webhook response: %v", err)
	}
	if webhookResp.ImageRepo != "ghcr.io/acme/string-entry" {
		t.Fatalf("image_repo = %q, want ghcr.io/acme/string-entry", webhookResp.ImageRepo)
	}
	if webhookResp.MatchedCount != 1 {
		t.Fatalf("matched_count = %d, want 1", webhookResp.MatchedCount)
	}
	if webhookResp.QueuedCount != 1 {
		t.Fatalf("queued_count = %d, want 1", webhookResp.QueuedCount)
	}
}

func TestIntegrationWebhookMatchesDockerHubCanonicalRepo(t *testing.T) {
	h := newTestHarness(t, true)
	loginAsAdmin(t, h)

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "dockerhub-app")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte(`
services:
  app:
    image: nginx:latest
`), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "dockerhub-app",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "nginx",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var created Target
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}
	if created.ImageRepo != "docker.io/library/nginx" {
		t.Fatalf("created image_repo = %q, want docker.io/library/nginx", created.ImageRepo)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPatch, h.server.URL+"/api/targets/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"auto_update_enabled": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable auto update status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	payload := map[string]any{
		"entry": map[string]any{
			"image": map[string]any{
				"repository": "docker.io/library/nginx",
			},
			"tag": "1.27.1",
		},
	}
	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var webhookResp diunWebhookResponse
	if err := json.Unmarshal(raw, &webhookResp); err != nil {
		t.Fatalf("unmarshal webhook response: %v", err)
	}
	if webhookResp.ImageRepo != "docker.io/library/nginx" {
		t.Fatalf("image_repo = %q, want docker.io/library/nginx", webhookResp.ImageRepo)
	}
	if webhookResp.MatchedCount != 1 {
		t.Fatalf("matched_count = %d, want 1", webhookResp.MatchedCount)
	}
	if webhookResp.QueuedCount != 1 {
		t.Fatalf("queued_count = %d, want 1", webhookResp.QueuedCount)
	}
}

func TestIntegrationWebhookMatchesLegacyNormalizedTargetImageRepo(t *testing.T) {
	h := newTestHarness(t, true)
	loginAsAdmin(t, h)

	now := time.Now().Unix()
	res, err := h.app.db.Exec(`
		INSERT INTO targets (name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, 1, 600, ?, ?)`,
		"legacy-nginx",
		filepath.Join(h.app.cfg.ContainerRoot, "sample-app"),
		"docker-compose.yml",
		"nginx",
		now,
		now,
	)
	if err != nil {
		t.Fatalf("insert legacy target: %v", err)
	}
	targetID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	if _, err := h.app.db.Exec(`
		INSERT INTO target_image_repos (target_id, image_repo, created_at)
		VALUES (?, ?, ?)`,
		targetID,
		"nginx",
		now,
	); err != nil {
		t.Fatalf("insert legacy target_image_repo: %v", err)
	}

	if err := migrate(h.app.db); err != nil {
		t.Fatalf("migrate legacy data: %v", err)
	}

	payload := map[string]any{
		"entry": map[string]any{
			"image": map[string]any{
				"name": "nginx:latest",
			},
			"tag": "latest",
		},
	}
	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var webhookResp diunWebhookResponse
	if err := json.Unmarshal(raw, &webhookResp); err != nil {
		t.Fatalf("unmarshal webhook response: %v", err)
	}
	if webhookResp.ImageRepo != "docker.io/library/nginx" {
		t.Fatalf("image_repo = %q, want docker.io/library/nginx", webhookResp.ImageRepo)
	}
	if webhookResp.MatchedCount != 1 {
		t.Fatalf("matched_count = %d, want 1", webhookResp.MatchedCount)
	}
	if webhookResp.QueuedCount != 1 {
		t.Fatalf("queued_count = %d, want 1", webhookResp.QueuedCount)
	}

	var primaryRepo string
	if err := h.app.db.QueryRow(`SELECT image_repo FROM targets WHERE id = ?`, targetID).Scan(&primaryRepo); err != nil {
		t.Fatalf("query normalized target image_repo: %v", err)
	}
	if primaryRepo != "docker.io/library/nginx" {
		t.Fatalf("normalized target image_repo = %q, want docker.io/library/nginx", primaryRepo)
	}
}

func TestIntegrationWebhookQueueFullStoresReceiptReason(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "queue-full-app")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(composeDir, "docker-compose.yml"), []byte("services:\n  app:\n    image: ghcr.io/acme/queue-full:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "queue-full-app",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/queue-full",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var created Target
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPatch, h.server.URL+"/api/targets/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"auto_update_enabled": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable auto update status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	h.app.queue = nil

	payload := map[string]any{
		"image": "ghcr.io/acme/queue-full:1.0.0",
	}
	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("webhook status = %d, want %d body=%s", resp.StatusCode, http.StatusServiceUnavailable, string(raw))
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/diun/receipts?limit=1", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("receipts status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var out struct {
		Receipts []WebhookReceiptSummary `json:"receipts"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal receipts response: %v", err)
	}
	if len(out.Receipts) != 1 {
		t.Fatalf("receipts len = %d, want 1", len(out.Receipts))
	}
	if out.Receipts[0].ReasonCode != webhookReasonQueueFull {
		t.Fatalf("receipt reason_code = %q, want %q", out.Receipts[0].ReasonCode, webhookReasonQueueFull)
	}
}

func TestIntegrationWebhookInternalErrorStoresReceiptReason(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "internal-error-app")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(composeDir, "docker-compose.yml"), []byte("services:\n  app:\n    image: ghcr.io/acme/internal-error:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "internal-error-app",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/internal-error",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var created Target
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPatch, h.server.URL+"/api/targets/"+strconv.FormatInt(created.ID, 10), map[string]any{
		"auto_update_enabled": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable auto update status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	if _, err := h.app.db.Exec(`DROP TABLE job_targets`); err != nil {
		t.Fatalf("drop job_targets: %v", err)
	}

	payload := map[string]any{
		"image": "ghcr.io/acme/internal-error:2.0.0",
	}
	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("webhook status = %d, want %d body=%s", resp.StatusCode, http.StatusInternalServerError, string(raw))
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/diun/receipts?limit=1", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("receipts status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var out struct {
		Receipts []WebhookReceiptSummary `json:"receipts"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal receipts response: %v", err)
	}
	if len(out.Receipts) != 1 {
		t.Fatalf("receipts len = %d, want 1", len(out.Receipts))
	}
	if out.Receipts[0].ReasonCode != webhookReasonInternalError {
		t.Fatalf("receipt reason_code = %q, want %q", out.Receipts[0].ReasonCode, webhookReasonInternalError)
	}
}

func TestIntegrationWebhookTestPayloadRecorded(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	payload := map[string]any{
		"status":  "test",
		"message": "notification test",
	}
	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook test status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/diun/events", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var out struct {
		Events []DiunEvent `json:"events"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal events: %v", err)
	}
	if len(out.Events) == 0 {
		t.Fatal("expected at least one diun event")
	}
	if out.Events[0].ImageRepo != "__diun_test__" {
		t.Fatalf("unexpected test event image_repo: %q", out.Events[0].ImageRepo)
	}
}

func TestIntegrationDeleteTargetSafety(t *testing.T) {
	h := newTestHarness(t, true)
	loginAsAdmin(t, h)
	t.Setenv("DOCKER_SLEEP_SEC", "1")

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "app-delete")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: ghcr.io/acme/delete:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "app-delete",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/delete",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var target Target
	if err := json.Unmarshal(raw, &target); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/jobs/update", map[string]any{
		"target_ids": []int64{target.ID},
	}, nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create update job status = %d, want %d body=%s", resp.StatusCode, http.StatusAccepted, string(raw))
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodDelete, h.server.URL+"/api/targets/"+strconv.FormatInt(target.ID, 10), nil, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("delete during active job status = %d, want %d body=%s", resp.StatusCode, http.StatusConflict, string(raw))
	}
	var apiErr apiErrorResponse
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("unmarshal delete conflict: %v", err)
	}
	if apiErr.Code != "target_has_active_jobs" {
		t.Fatalf("unexpected delete conflict error: %+v", apiErr)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/jobs?limit=10", nil, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("list jobs status = %d body=%s", resp.StatusCode, string(raw))
		}
		var jobsResp struct {
			Jobs []Job `json:"jobs"`
		}
		if err := json.Unmarshal(raw, &jobsResp); err != nil {
			t.Fatalf("unmarshal jobs response: %v", err)
		}
		if len(jobsResp.Jobs) > 0 && jobsResp.Jobs[0].Status == statusSuccess {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodDelete, h.server.URL+"/api/targets/"+strconv.FormatInt(target.ID, 10), nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete target status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
}

func TestIntegrationJobsCursorPagination(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	now := time.Now().Unix()
	for i := 0; i < 5; i++ {
		_, err := h.app.db.ExecContext(context.Background(),
			`INSERT INTO jobs (type, trigger, status, created_at) VALUES (?, ?, ?, ?)`,
			jobTypePrune, triggerManual, statusSuccess, now+int64(i),
		)
		if err != nil {
			t.Fatalf("insert job %d: %v", i, err)
		}
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/jobs?limit=2", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("page1 status = %d body=%s", resp.StatusCode, string(raw))
	}
	var page1 struct {
		Jobs       []Job  `json:"jobs"`
		NextCursor *int64 `json:"next_cursor,omitempty"`
	}
	if err := json.Unmarshal(raw, &page1); err != nil {
		t.Fatalf("unmarshal page1: %v", err)
	}
	if len(page1.Jobs) != 2 {
		t.Fatalf("page1 jobs len = %d, want 2", len(page1.Jobs))
	}
	if page1.NextCursor == nil {
		t.Fatalf("expected next_cursor on page1")
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/jobs?limit=2&cursor="+strconv.FormatInt(*page1.NextCursor, 10), nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("page2 status = %d body=%s", resp.StatusCode, string(raw))
	}
	var page2 struct {
		Jobs       []Job  `json:"jobs"`
		NextCursor *int64 `json:"next_cursor,omitempty"`
	}
	if err := json.Unmarshal(raw, &page2); err != nil {
		t.Fatalf("unmarshal page2: %v", err)
	}
	if len(page2.Jobs) != 2 {
		t.Fatalf("page2 jobs len = %d, want 2", len(page2.Jobs))
	}
	if page2.Jobs[0].ID >= page1.Jobs[len(page1.Jobs)-1].ID {
		t.Fatalf("expected page2 to contain older jobs; page1=%d page2=%d", page1.Jobs[len(page1.Jobs)-1].ID, page2.Jobs[0].ID)
	}
}

func TestIntegrationDeleteJobHistory(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	now := time.Now().Unix()
	successRes, err := h.app.db.ExecContext(context.Background(),
		`INSERT INTO jobs (type, trigger, status, created_at, started_at, ended_at) VALUES (?, ?, ?, ?, ?, ?)`,
		jobTypeUpdate, triggerManual, statusSuccess, now, now, now+10,
	)
	if err != nil {
		t.Fatalf("insert success job: %v", err)
	}
	successID, err := successRes.LastInsertId()
	if err != nil {
		t.Fatalf("success job last insert id: %v", err)
	}

	queuedRes, err := h.app.db.ExecContext(context.Background(),
		`INSERT INTO jobs (type, trigger, status, created_at) VALUES (?, ?, ?, ?)`,
		jobTypeUpdate, triggerManual, statusQueued, now+1,
	)
	if err != nil {
		t.Fatalf("insert queued job: %v", err)
	}
	queuedID, err := queuedRes.LastInsertId()
	if err != nil {
		t.Fatalf("queued job last insert id: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodDelete, h.server.URL+"/api/jobs/"+strconv.FormatInt(successID, 10), nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete completed job status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var count int
	if err := h.app.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM jobs WHERE id = ?`, successID).Scan(&count); err != nil {
		t.Fatalf("count deleted job: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected deleted job count=0, got %d", count)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodDelete, h.server.URL+"/api/jobs/"+strconv.FormatInt(queuedID, 10), nil, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("delete queued job status = %d, want %d body=%s", resp.StatusCode, http.StatusConflict, string(raw))
	}
	var apiErr apiErrorResponse
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("unmarshal conflict error: %v", err)
	}
	if apiErr.Code != "job_active" {
		t.Fatalf("unexpected error code: %+v", apiErr)
	}
}

func TestIntegrationDeleteAllJobHistory(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	now := time.Now().Unix()
	if _, err := h.app.db.ExecContext(context.Background(),
		`INSERT INTO jobs (type, trigger, status, created_at, started_at, ended_at) VALUES (?, ?, ?, ?, ?, ?)`,
		jobTypeUpdate, triggerManual, statusSuccess, now, now, now+10,
	); err != nil {
		t.Fatalf("insert success job: %v", err)
	}
	if _, err := h.app.db.ExecContext(context.Background(),
		`INSERT INTO jobs (type, trigger, status, created_at) VALUES (?, ?, ?, ?)`,
		jobTypePrune, triggerManual, statusRunning, now+1,
	); err != nil {
		t.Fatalf("insert running job: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/jobs/delete-all", map[string]any{
		"confirm": true,
	}, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("delete-all with active job status = %d, want %d body=%s", resp.StatusCode, http.StatusConflict, string(raw))
	}
	var apiErr apiErrorResponse
	if err := json.Unmarshal(raw, &apiErr); err != nil {
		t.Fatalf("unmarshal conflict error: %v", err)
	}
	if apiErr.Code != "job_active" {
		t.Fatalf("unexpected conflict code: %+v", apiErr)
	}

	if _, err := h.app.db.ExecContext(context.Background(),
		`UPDATE jobs SET status = ?, started_at = ?, ended_at = ? WHERE status = ?`,
		statusSuccess, now+2, now+3, statusRunning,
	); err != nil {
		t.Fatalf("update running job to success: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/jobs/delete-all", map[string]any{
		"confirm": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete-all status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var out struct {
		Deleted      bool  `json:"deleted"`
		DeletedCount int64 `json:"deleted_count"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal delete-all response: %v", err)
	}
	if !out.Deleted {
		t.Fatalf("expected deleted=true response: %+v", out)
	}
	if out.DeletedCount != 2 {
		t.Fatalf("deleted_count = %d, want 2", out.DeletedCount)
	}

	var total int64
	if err := h.app.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM jobs`).Scan(&total); err != nil {
		t.Fatalf("count jobs after delete-all: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total jobs 0 after delete-all, got %d", total)
	}
}

func TestIntegrationWebhookMaintenanceWindow(t *testing.T) {
	currentHour := time.Now().Hour()
	blockStart := (currentHour + 1) % 24
	blockEnd := (currentHour + 2) % 24

	h := newTestHarnessWithConfig(t, false, func(cfg *Config) {
		cfg.AutoWindowStart = blockStart
		cfg.AutoWindowEnd = blockEnd
	})
	loginAsAdmin(t, h)

	composeDir := filepath.Join(h.app.cfg.ContainerRoot, "app-window")
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		t.Fatalf("mkdir compose dir: %v", err)
	}
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  app:\n    image: ghcr.io/acme/window:latest\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/targets", map[string]any{
		"name":         "app-window",
		"compose_dir":  composeDir,
		"compose_file": "docker-compose.yml",
		"image_repo":   "ghcr.io/acme/window",
	}, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create target status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(raw))
	}
	var target Target
	if err := json.Unmarshal(raw, &target); err != nil {
		t.Fatalf("unmarshal target: %v", err)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodPatch, h.server.URL+"/api/targets/"+strconv.FormatInt(target.ID, 10), map[string]any{
		"auto_update_enabled": true,
	}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enable auto update status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	payload := map[string]any{
		"entry": map[string]any{
			"image": map[string]any{
				"name": "ghcr.io/acme/window:latest",
			},
		},
	}
	resp, raw = doJSONRequest(t, h.client, http.MethodPost, h.server.URL+"/api/diun/webhook", payload, map[string]string{
		"X-DIUN-SECRET": h.app.webhookSecret,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var webhookResp diunWebhookResponse
	if err := json.Unmarshal(raw, &webhookResp); err != nil {
		t.Fatalf("unmarshal webhook resp: %v", err)
	}
	if webhookResp.WindowOpen {
		t.Fatalf("expected window closed response")
	}
	if webhookResp.QueuedCount != 0 {
		t.Fatalf("expected queued count 0 when window closed, got %d", webhookResp.QueuedCount)
	}

	resp, raw = doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/diun/receipts?limit=1", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("receipts status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	var receipts struct {
		Receipts []WebhookReceiptSummary `json:"receipts"`
	}
	if err := json.Unmarshal(raw, &receipts); err != nil {
		t.Fatalf("unmarshal receipts: %v", err)
	}
	if len(receipts.Receipts) == 0 {
		t.Fatal("expected at least one webhook receipt")
	}
	if receipts.Receipts[0].ReasonCode != webhookReasonOutsideMaintenanceWindow {
		t.Fatalf("unexpected receipt reason_code = %q, want %q", receipts.Receipts[0].ReasonCode, webhookReasonOutsideMaintenanceWindow)
	}
}

func TestIntegrationJobsExportCSV(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	now := time.Now().Unix()
	targetRes, err := h.app.db.ExecContext(context.Background(), `
		INSERT INTO targets (name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 1, ?, ?, ?)`,
		"csv-app", filepath.Join(h.app.cfg.ContainerRoot, "csv-app"), "docker-compose.yml", "ghcr.io/acme/csv-app", 600, now, now,
	)
	if err != nil {
		t.Fatalf("insert target: %v", err)
	}
	targetID, err := targetRes.LastInsertId()
	if err != nil {
		t.Fatalf("target last insert id: %v", err)
	}

	job1Res, err := h.app.db.ExecContext(context.Background(), `
		INSERT INTO jobs (type, trigger, status, created_at, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		jobTypeUpdate, triggerManual, statusSuccess, now, now, now+25,
	)
	if err != nil {
		t.Fatalf("insert job1: %v", err)
	}
	job1ID, err := job1Res.LastInsertId()
	if err != nil {
		t.Fatalf("job1 last insert id: %v", err)
	}
	if _, err := h.app.db.ExecContext(context.Background(),
		`INSERT INTO job_targets (job_id, target_id, position) VALUES (?, ?, 0)`,
		job1ID, targetID,
	); err != nil {
		t.Fatalf("insert job1 target: %v", err)
	}

	job2Res, err := h.app.db.ExecContext(context.Background(), `
		INSERT INTO jobs (type, trigger, status, error_message, created_at, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		jobTypeUpdate, triggerAuto, statusFailed, "pull failed", now+1, now+1, now+2,
	)
	if err != nil {
		t.Fatalf("insert job2: %v", err)
	}
	job2ID, err := job2Res.LastInsertId()
	if err != nil {
		t.Fatalf("job2 last insert id: %v", err)
	}
	if _, err := h.app.db.ExecContext(context.Background(),
		`INSERT INTO job_targets (job_id, target_id, position) VALUES (?, ?, 0)`,
		job2ID, targetID,
	); err != nil {
		t.Fatalf("insert job2 target: %v", err)
	}

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/jobs/export.csv?limit=10", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}
	if got := resp.Header.Get("Content-Type"); len(got) < 8 || got[:8] != "text/csv" {
		t.Fatalf("unexpected content-type: %q", got)
	}

	records, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("csv rows = %d, want 3", len(records))
	}
	header := records[0]
	wantHeader := []string{"id", "type", "trigger", "status", "target_ids", "duration_sec", "error_message", "created_at", "started_at", "ended_at"}
	if len(header) != len(wantHeader) {
		t.Fatalf("header len = %d, want %d", len(header), len(wantHeader))
	}
	for i := range header {
		if header[i] != wantHeader[i] {
			t.Fatalf("header[%d] = %q, want %q", i, header[i], wantHeader[i])
		}
	}
	if records[1][4] != strconv.FormatInt(targetID, 10) {
		t.Fatalf("row1 target_ids = %q, want %d", records[1][4], targetID)
	}
	if records[2][4] != strconv.FormatInt(targetID, 10) {
		t.Fatalf("row2 target_ids = %q, want %d", records[2][4], targetID)
	}
}

func TestIntegrationTargetAuditSummary(t *testing.T) {
	h := newTestHarness(t, false)
	loginAsAdmin(t, h)

	now := time.Now().Unix()
	target1Res, err := h.app.db.ExecContext(context.Background(), `
		INSERT INTO targets (name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at, last_success_at)
		VALUES (?, ?, ?, ?, 1, 1, ?, ?, ?, ?)`,
		"audit-app-1", filepath.Join(h.app.cfg.ContainerRoot, "audit-app-1"), "docker-compose.yml", "ghcr.io/acme/audit-1", 600, now, now, now-300,
	)
	if err != nil {
		t.Fatalf("insert target1: %v", err)
	}
	target1ID, err := target1Res.LastInsertId()
	if err != nil {
		t.Fatalf("target1 last insert id: %v", err)
	}
	target2Res, err := h.app.db.ExecContext(context.Background(), `
		INSERT INTO targets (name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 1, ?, ?, ?)`,
		"audit-app-2", filepath.Join(h.app.cfg.ContainerRoot, "audit-app-2"), "docker-compose.yml", "ghcr.io/acme/audit-2", 600, now, now,
	)
	if err != nil {
		t.Fatalf("insert target2: %v", err)
	}
	target2ID, err := target2Res.LastInsertId()
	if err != nil {
		t.Fatalf("target2 last insert id: %v", err)
	}

	insertJob := func(jobType, status string, created int64, started, ended *int64, targetID int64) int64 {
		res, err := h.app.db.ExecContext(context.Background(), `
			INSERT INTO jobs (type, trigger, status, created_at, started_at, ended_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			jobType, triggerManual, status, created, started, ended,
		)
		if err != nil {
			t.Fatalf("insert job (%s/%s): %v", jobType, status, err)
		}
		jobID, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("job last insert id: %v", err)
		}
		if _, err := h.app.db.ExecContext(context.Background(),
			`INSERT INTO job_targets (job_id, target_id, position) VALUES (?, ?, 0)`,
			jobID, targetID,
		); err != nil {
			t.Fatalf("insert job target: %v", err)
		}
		return jobID
	}

	start1 := now + 10
	end1 := now + 40
	start2 := now + 20
	end2 := now + 30
	start3 := now + 40
	end3 := now + 60
	insertJob(jobTypeUpdate, statusSuccess, now+10, &start1, &end1, target1ID) // 30s
	insertJob(jobTypeUpdate, statusFailed, now+20, &start2, &end2, target1ID)  // 10s
	insertJob(jobTypeUpdate, statusBlocked, now+30, nil, nil, target1ID)
	insertJob(jobTypePrune, statusSuccess, now+40, &start3, &end3, target1ID) // should be excluded

	resp, raw := doJSONRequest(t, h.client, http.MethodGet, h.server.URL+"/api/audit/targets", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("audit status = %d, want %d body=%s", resp.StatusCode, http.StatusOK, string(raw))
	}

	var result struct {
		Summaries []TargetAuditSummary `json:"summaries"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal audit response: %v", err)
	}
	if len(result.Summaries) != 2 {
		t.Fatalf("summary len = %d, want 2", len(result.Summaries))
	}

	summaryByID := map[int64]TargetAuditSummary{}
	for _, s := range result.Summaries {
		summaryByID[s.TargetID] = s
	}

	s1, ok := summaryByID[target1ID]
	if !ok {
		t.Fatalf("missing summary for target1 %d", target1ID)
	}
	if s1.TotalRuns != 3 {
		t.Fatalf("target1 total_runs = %d, want 3", s1.TotalRuns)
	}
	if s1.SuccessRuns != 1 || s1.FailedRuns != 1 || s1.BlockedRuns != 1 {
		t.Fatalf("target1 run counts unexpected: %+v", s1)
	}
	if s1.LastRunAt == nil || *s1.LastRunAt != now+30 {
		t.Fatalf("target1 last_run_at unexpected: %+v", s1.LastRunAt)
	}
	if s1.LastSuccessAt == nil || *s1.LastSuccessAt != now-300 {
		t.Fatalf("target1 last_success_at unexpected: %+v", s1.LastSuccessAt)
	}
	if s1.AvgDurationSec != 20 {
		t.Fatalf("target1 avg_duration_sec = %.2f, want 20", s1.AvgDurationSec)
	}

	s2, ok := summaryByID[target2ID]
	if !ok {
		t.Fatalf("missing summary for target2 %d", target2ID)
	}
	if s2.TotalRuns != 0 || s2.SuccessRuns != 0 || s2.FailedRuns != 0 || s2.BlockedRuns != 0 {
		t.Fatalf("target2 should be zero summary: %+v", s2)
	}
	if s2.LastRunAt != nil {
		t.Fatalf("target2 last_run_at expected nil: %+v", s2.LastRunAt)
	}
}

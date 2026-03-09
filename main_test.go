package main

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestValidateComposeDir(t *testing.T) {
	root := "/share/Container"

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: "/share/Container/app1", wantErr: false},
		{name: "nested invalid", input: "/share/Container/app1/sub", wantErr: true},
		{name: "outside invalid", input: "/share/Other/app1", wantErr: true},
		{name: "root invalid", input: "/share/Container", wantErr: true},
		{name: "relative invalid", input: "share/Container/app1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateComposeDir(root, tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateComposeDir() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeImageRepo(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "ghcr.io/owner/app:1.2.3", want: "ghcr.io/owner/app"},
		{in: "ghcr.io:5000/owner/app:latest", want: "ghcr.io:5000/owner/app"},
		{in: "nginx:latest", want: "docker.io/library/nginx"},
		{in: "library/nginx:latest", want: "docker.io/library/nginx"},
		{in: "docker.io/nginx:latest", want: "docker.io/library/nginx"},
		{in: "index.docker.io/library/nginx:latest", want: "docker.io/library/nginx"},
		{in: "myorg/app:latest", want: "docker.io/myorg/app"},
		{in: "docker.io/library/nginx@sha256:abcd", want: "docker.io/library/nginx"},
		{in: "  DOCKER.IO/LIBRARY/NGINX:latest  ", want: "docker.io/library/nginx"},
	}

	for _, tt := range tests {
		got := normalizeImageRepo(tt.in)
		if got != tt.want {
			t.Fatalf("normalizeImageRepo(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMigrateNormalizesLegacyTargetImageRepos(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")
	db, err := openDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		t.Fatalf("initial migrate: %v", err)
	}

	now := time.Now().Unix()
	res, err := db.Exec(`
		INSERT INTO targets (name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, 1, 600, ?, ?)`,
		"legacy-nginx",
		"/share/Container/legacy-nginx",
		"docker-compose.yml",
		"nginx",
		now,
		now,
	)
	if err != nil {
		t.Fatalf("insert target: %v", err)
	}
	targetID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO target_image_repos (target_id, image_repo, created_at)
		VALUES (?, ?, ?)`,
		targetID,
		"nginx",
		now,
	); err != nil {
		t.Fatalf("insert raw target_image_repo: %v", err)
	}

	if err := migrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var primaryRepo string
	if err := db.QueryRow(`SELECT image_repo FROM targets WHERE id = ?`, targetID).Scan(&primaryRepo); err != nil {
		t.Fatalf("query target image_repo: %v", err)
	}
	if primaryRepo != "docker.io/library/nginx" {
		t.Fatalf("primary image_repo = %q, want docker.io/library/nginx", primaryRepo)
	}

	rows, err := db.Query(`
		SELECT image_repo
		FROM target_image_repos
		WHERE target_id = ?
		ORDER BY image_repo ASC`,
		targetID,
	)
	if err != nil {
		t.Fatalf("query target_image_repos: %v", err)
	}
	defer rows.Close()

	var repos []string
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			t.Fatalf("scan target_image_repo: %v", err)
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("target_image_repos rows err: %v", err)
	}

	want := []string{"docker.io/library/nginx"}
	if !reflect.DeepEqual(repos, want) {
		t.Fatalf("target_image_repos = %+v, want %+v", repos, want)
	}
}

func TestExtractDiunFields(t *testing.T) {
	payload := map[string]any{
		"entry": map[string]any{
			"image": map[string]any{
				"name": "ghcr.io/acme/myapp:2.0.1",
			},
			"tag":    "2.0.1",
			"digest": "sha256:1234",
		},
	}

	repo, tag, digest := extractDiunFields(payload)
	if repo != "ghcr.io/acme/myapp" {
		t.Fatalf("repo = %q", repo)
	}
	if tag != "2.0.1" {
		t.Fatalf("tag = %q", tag)
	}
	if digest != "sha256:1234" {
		t.Fatalf("digest = %q", digest)
	}
}

func TestValidateComposeFile(t *testing.T) {
	valid := []string{"docker-compose.yml", "compose.yaml", "stack.yml"}
	for _, v := range valid {
		if err := validateComposeFile(v); err != nil {
			t.Fatalf("validateComposeFile(%q) unexpected error: %v", v, err)
		}
	}

	invalid := []string{"../docker-compose.yml", "a/b.yml", ""}
	for _, v := range invalid {
		if err := validateComposeFile(v); err == nil {
			t.Fatalf("validateComposeFile(%q) expected error", v)
		}
	}
}

func TestIsAllowedDBPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/data/app.db", want: true},
		{path: "/data/nested/app.db", want: true},
		{path: "/share/Container/app.db", want: false},
		{path: "data/app.db", want: false},
		{path: "/data", want: false},
	}

	for _, tt := range tests {
		got := isAllowedDBPath(tt.path)
		if got != tt.want {
			t.Fatalf("isAllowedDBPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSessionStoreCreateValidateDelete(t *testing.T) {
	store := newSessionStore(2 * time.Second)
	token, expires, err := store.Create("admin")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if token == "" {
		t.Fatal("Create() token is empty")
	}
	if time.Until(expires) <= 0 {
		t.Fatal("Create() expiration not in future")
	}

	username, ok := store.Validate(token)
	if !ok {
		t.Fatal("Validate() expected valid token")
	}
	if username != "admin" {
		t.Fatalf("Validate() username = %q, want admin", username)
	}

	store.Delete(token)
	if _, ok := store.Validate(token); ok {
		t.Fatal("Validate() expected deleted token to be invalid")
	}
}

func TestSessionStoreExpires(t *testing.T) {
	store := newSessionStore(1 * time.Millisecond)
	token, _, err := store.Create("admin")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	time.Sleep(3 * time.Millisecond)
	if _, ok := store.Validate(token); ok {
		t.Fatal("Validate() expected expired token to be invalid")
	}
}

func TestValidateCredentials(t *testing.T) {
	app := &App{
		cfg: Config{
			AdminUsername: "admin",
		},
		adminPasswordHash: hashString("secret-password"),
	}

	if !app.validateCredentials("admin", "secret-password") {
		t.Fatal("expected valid credentials")
	}
	if app.validateCredentials("admin", "wrong") {
		t.Fatal("expected invalid password")
	}
	if app.validateCredentials("other", "secret-password") {
		t.Fatal("expected invalid username")
	}
}

func TestRandomToken(t *testing.T) {
	token1, err := randomToken(24)
	if err != nil {
		t.Fatalf("randomToken() error: %v", err)
	}
	token2, err := randomToken(24)
	if err != nil {
		t.Fatalf("randomToken() error: %v", err)
	}
	if token1 == "" || token2 == "" {
		t.Fatal("expected non-empty tokens")
	}
	if token1 == token2 {
		t.Fatal("expected different random tokens")
	}
}

func TestNormalizeWebPushSubject(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trim mailto spaces", in: "mailto: admin@example.com ", want: "mailto:admin@example.com"},
		{name: "plain email to mailto", in: "admin@example.com", want: "mailto:admin@example.com"},
		{name: "https unchanged", in: "https://example.com/contact", want: "https://example.com/contact"},
		{name: "empty", in: "   ", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeWebPushSubject(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeWebPushSubject(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSSELimiter(t *testing.T) {
	limiter := newSSELimiter(2)
	if !limiter.Acquire() {
		t.Fatal("first acquire should succeed")
	}
	if !limiter.Acquire() {
		t.Fatal("second acquire should succeed")
	}
	if limiter.Acquire() {
		t.Fatal("third acquire should be rejected")
	}
	active, rejected := limiter.Stats()
	if active != 2 {
		t.Fatalf("active = %d, want 2", active)
	}
	if rejected != 1 {
		t.Fatalf("rejected = %d, want 1", rejected)
	}

	limiter.Release()
	limiter.Release()
	active, _ = limiter.Stats()
	if active != 0 {
		t.Fatalf("active after release = %d, want 0", active)
	}
}

func TestWithinAutoUpdateWindow(t *testing.T) {
	app := &App{cfg: Config{AutoWindowStart: -1, AutoWindowEnd: -1}}
	if !app.withinAutoUpdateWindow(time.Date(2026, 2, 26, 3, 0, 0, 0, time.UTC)) {
		t.Fatal("disabled window should allow updates")
	}

	app.cfg.AutoWindowStart = 2
	app.cfg.AutoWindowEnd = 5
	if !app.withinAutoUpdateWindow(time.Date(2026, 2, 26, 3, 0, 0, 0, time.UTC)) {
		t.Fatal("expected inside daytime window")
	}
	if app.withinAutoUpdateWindow(time.Date(2026, 2, 26, 1, 0, 0, 0, time.UTC)) {
		t.Fatal("expected outside daytime window")
	}

	app.cfg.AutoWindowStart = 22
	app.cfg.AutoWindowEnd = 3
	if !app.withinAutoUpdateWindow(time.Date(2026, 2, 26, 23, 0, 0, 0, time.UTC)) {
		t.Fatal("expected inside overnight window at 23")
	}
	if !app.withinAutoUpdateWindow(time.Date(2026, 2, 26, 1, 0, 0, 0, time.UTC)) {
		t.Fatal("expected inside overnight window at 01")
	}
	if app.withinAutoUpdateWindow(time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("expected outside overnight window at 12")
	}
}

func TestParseComposeImageValue(t *testing.T) {
	tests := []struct {
		line string
		ok   bool
		want string
	}{
		{line: "image: ghcr.io/acme/app:latest", ok: true, want: "ghcr.io/acme/app:latest"},
		{line: `  image: "ghcr.io/acme/app:1.2.3"`, ok: true, want: "ghcr.io/acme/app:1.2.3"},
		{line: "image: ghcr.io/acme/app:latest # comment", ok: true, want: "ghcr.io/acme/app:latest"},
		{line: "build: .", ok: false, want: ""},
		{line: "image: ${APP_IMAGE}", ok: true, want: "${APP_IMAGE}"},
	}
	for _, tt := range tests {
		got, ok := parseComposeImageValue(tt.line)
		if ok != tt.ok {
			t.Fatalf("parseComposeImageValue(%q) ok=%v, want %v", tt.line, ok, tt.ok)
		}
		if got != tt.want {
			t.Fatalf("parseComposeImageValue(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestResolveComposeVariables(t *testing.T) {
	vars := map[string]string{
		"REPO":  "ghcr.io/acme/app",
		"TAG":   "1.2.3",
		"EMPTY": "",
	}

	tests := []struct {
		name string
		in   string
		ok   bool
		want string
	}{
		{
			name: "simple variable",
			in:   "${REPO}:${TAG}",
			ok:   true,
			want: "ghcr.io/acme/app:1.2.3",
		},
		{
			name: "default value for missing variable",
			in:   "${MISSING:-ghcr.io/acme/fallback}:latest",
			ok:   true,
			want: "ghcr.io/acme/fallback:latest",
		},
		{
			name: "default value for empty variable",
			in:   "${EMPTY:-ghcr.io/acme/fallback}:latest",
			ok:   true,
			want: "ghcr.io/acme/fallback:latest",
		},
		{
			name: "optional alt when set",
			in:   "${TAG:+stable}",
			ok:   true,
			want: "stable",
		},
		{
			name: "required variable missing",
			in:   "${MISSING?required}",
			ok:   false,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolveComposeVariables(tt.in, vars)
			if ok != tt.ok {
				t.Fatalf("resolveComposeVariables(%q) ok=%v, want %v", tt.in, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("resolveComposeVariables(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseComposeImageReposWithVariableResolution(t *testing.T) {
	tempDir := t.TempDir()
	composePath := filepath.Join(tempDir, "docker-compose.yml")
	envPath := filepath.Join(tempDir, ".env")

	if err := os.WriteFile(composePath, []byte(`
services:
  app:
    image: ${APP_REPO}:${APP_TAG}
  worker:
    image: ${WORKER_REPO:-ghcr.io/acme/worker}:${WORKER_TAG-default}
  sidecar:
    image: "${SIDECAR_REPO:?required}:latest"
  skipped:
    image: ${UNSET_REPO}
`), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	if err := os.WriteFile(envPath, []byte(`
APP_REPO=ghcr.io/acme/app
APP_TAG=1.0.0
WORKER_TAG=2.0.0
`), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	t.Setenv("APP_TAG", "9.9.9")
	t.Setenv("SIDECAR_REPO", "ghcr.io/acme/sidecar")

	repos, err := parseComposeImageRepos(composePath)
	if err != nil {
		t.Fatalf("parseComposeImageRepos() error: %v", err)
	}

	want := []string{
		"ghcr.io/acme/app",
		"ghcr.io/acme/sidecar",
		"ghcr.io/acme/worker",
	}
	if !reflect.DeepEqual(repos, want) {
		t.Fatalf("parseComposeImageRepos() = %+v, want %+v", repos, want)
	}
}

func TestParseComposeImageReposCanonicalizesDockerHubShorthand(t *testing.T) {
	tempDir := t.TempDir()
	composePath := filepath.Join(tempDir, "docker-compose.yml")

	if err := os.WriteFile(composePath, []byte(`
services:
  app:
    image: nginx:latest
  worker:
    image: myorg/worker:stable
`), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}

	repos, err := parseComposeImageRepos(composePath)
	if err != nil {
		t.Fatalf("parseComposeImageRepos() error: %v", err)
	}

	want := []string{
		"docker.io/library/nginx",
		"docker.io/myorg/worker",
	}
	if !reflect.DeepEqual(repos, want) {
		t.Fatalf("parseComposeImageRepos() = %+v, want %+v", repos, want)
	}
}

func TestIsDiunTestPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload map[string]any
		want    bool
	}{
		{
			name:    "status test",
			payload: map[string]any{"status": "test"},
			want:    true,
		},
		{
			name:    "type test",
			payload: map[string]any{"type": "test"},
			want:    true,
		},
		{
			name:    "normal event",
			payload: map[string]any{"status": "new"},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDiunTestPayload(tt.payload)
			if got != tt.want {
				t.Fatalf("isDiunTestPayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoginRateLimiter(t *testing.T) {
	limiter := newLoginRateLimiter(LoginRateLimiterConfig{
		WindowSeconds: 600,
		MaxAttempts:   5,
		LockSeconds:   900,
	})
	key := buildLoginRateLimitKey("192.168.0.10", "admin")
	now := time.Unix(1_700_000_000, 0)

	if blocked, _ := limiter.Check(key, now); blocked {
		t.Fatal("expected empty limiter state to allow login")
	}
	for i := 0; i < 5; i++ {
		limiter.RecordFailure(key, now.Add(time.Duration(i)*time.Second))
	}
	blocked, retryAfter := limiter.Check(key, now.Add(6*time.Second))
	if !blocked {
		t.Fatal("expected limiter to block after max failures")
	}
	if retryAfter <= 0 {
		t.Fatalf("retryAfter should be positive, got %d", retryAfter)
	}

	limiter.Reset(key)
	if blocked, _ := limiter.Check(key, now.Add(7*time.Second)); blocked {
		t.Fatal("expected limiter reset to clear lock")
	}
}

func TestParseRetryAfterToken(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantOK    bool
		wantDelay time.Duration
	}{
		{name: "duration", token: "183.761µs", wantOK: true, wantDelay: 183761 * time.Nanosecond},
		{name: "seconds float", token: "3.5", wantOK: true, wantDelay: 3500 * time.Millisecond},
		{name: "seconds int", token: "7", wantOK: true, wantDelay: 7 * time.Second},
		{name: "invalid", token: "abc", wantOK: false, wantDelay: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseRetryAfterToken(tt.token)
			if ok != tt.wantOK {
				t.Fatalf("parseRetryAfterToken(%q) ok=%v, want %v", tt.token, ok, tt.wantOK)
			}
			if ok && got != tt.wantDelay {
				t.Fatalf("parseRetryAfterToken(%q) delay=%s, want %s", tt.token, got, tt.wantDelay)
			}
		})
	}
}

func TestComputePullRetryDelayTooManyRequestsClamp(t *testing.T) {
	err := &commandRunError{
		err:         errors.New("exit status 1"),
		outputLines: []string{"Error response from daemon: toomanyrequests: retry-after: 183.761µs, allowed: 44000/minute"},
	}
	got := computePullRetryDelay(err, 0, 2)
	if got != 2*time.Second {
		t.Fatalf("computePullRetryDelay() = %s, want 2s", got)
	}

	err = &commandRunError{
		err:         errors.New("exit status 1"),
		outputLines: []string{"toomanyrequests: retry-after: 9999s"},
	}
	got = computePullRetryDelay(err, 0, 2)
	if got != 120*time.Second {
		t.Fatalf("computePullRetryDelay() = %s, want 120s", got)
	}
}

func TestClientIPFromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	if got := clientIPFromRequest(req); got != "203.0.113.1" {
		t.Fatalf("clientIPFromRequest() = %q, want 203.0.113.1", got)
	}

	req = httptest.NewRequest("GET", "http://example.com", nil)
	req.RemoteAddr = "10.0.0.2:4321"
	if got := clientIPFromRequest(req); got != "10.0.0.2" {
		t.Fatalf("clientIPFromRequest() remote fallback = %q, want 10.0.0.2", got)
	}
}

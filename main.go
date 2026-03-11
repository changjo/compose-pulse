package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"crypto/subtle"

	"github.com/SherClockHolmes/webpush-go"
	_ "github.com/mattn/go-sqlite3"
)

const (
	jobTypeUpdate = "update"
	jobTypePrune  = "prune"

	triggerManual = "manual"
	triggerAuto   = "diun-auto"

	statusQueued  = "queued"
	statusRunning = "running"
	statusSuccess = "success"
	statusFailed  = "failed"
	statusBlocked = "blocked"

	sessionCookieName = "composepulse_session"

	settingKeyWebhookSecret   = "diun_webhook_secret"
	settingKeyAdminPassSHA256 = "admin_password_sha256"

	webhookReasonSecretMismatch           = "secret_mismatch"
	webhookReasonPayloadInvalid           = "payload_invalid"
	webhookReasonNoMatch                  = "no_match"
	webhookReasonQueued                   = "queued"
	webhookReasonQueueFull                = "queue_full"
	webhookReasonInternalError            = "internal_error"
	webhookReasonAutoDisabled             = "auto_disabled"
	webhookReasonCooldownBlocked          = "cooldown_blocked"
	webhookReasonOutsideMaintenanceWindow = "outside_maintenance_window"

	dashboardSectionTargets  = "targets"
	dashboardSectionJobs     = "jobs"
	dashboardSectionMetrics  = "metrics"
	dashboardSectionAudit    = "audit"
	dashboardSectionEvents   = "events"
	dashboardSectionReceipts = "receipts"

	eventTypeTargetCreated   = "target_created"
	eventTypeTargetUpdated   = "target_updated"
	eventTypeTargetDeleted   = "target_deleted"
	eventTypeJobQueued       = "job_queued"
	eventTypeJobRunning      = "job_running"
	eventTypeJobSuccess      = "job_success"
	eventTypeJobFailed       = "job_failed"
	eventTypeJobBlocked      = "job_blocked"
	eventTypeWebhookReceipt  = "webhook_receipt"
	eventTypeDiunEvent       = "diun_event"
	eventTypeAuthRateLimited = "auth_rate_limited"
)

//go:embed web/*
var webFS embed.FS

var buildVersion = "dev"

type Config struct {
	Port                 string
	DBPath               string
	WebDir               string
	DiunWebhookSecret    string
	ShowWebhookSecret    bool
	DefaultCooldown      int
	ContainerRoot        string
	AdminUsername        string
	AdminPassword        string
	SessionTTLSeconds    int
	RememberMeTTLSeconds int
	APITimeoutSeconds    int
	SSEMaxConnections    int
	DashboardSSEMax      int
	PullRetryMaxAttempts int
	PullRetryDelaySec    int
	AutoWindowStart      int
	AutoWindowEnd        int
	LoginRateLimit       LoginRateLimiterConfig
	WebPushEnabled       bool
	WebPushVAPIDPublic   string
	WebPushVAPIDPrivate  string
	WebPushSubject       string
}

type App struct {
	cfg               Config
	db                *sql.DB
	logBroker         *LogBroker
	dashboardBroker   *DashboardEventBroker
	queue             chan int64
	logMu             sync.Mutex
	sessions          *SessionStore
	adminPasswordHash string
	webhookSecret     string
	sseLimiter        *SSELimiter
	dashboardLimiter  *SSELimiter
	loginLimiter      *LoginRateLimiter
	pushDedupeMu      sync.Mutex
	pushDedupe        map[string]int64
	assetVersion      string
	appVersion        string
}

type LogBroker struct {
	mu   sync.Mutex
	subs map[int64]map[chan string]struct{}
}

type DashboardEventBroker struct {
	mu   sync.Mutex
	subs map[chan DashboardPatchEvent]struct{}
	seq  atomic.Int64
}

type SessionStore struct {
	mu       sync.Mutex
	ttl      time.Duration
	sessions map[string]SessionEntry
}

type SessionEntry struct {
	Username string
	Expires  time.Time
}

type SSELimiter struct {
	max      int64
	active   atomic.Int64
	rejected atomic.Int64
}

type LoginRateLimiterConfig struct {
	WindowSeconds int `json:"window_seconds"`
	MaxAttempts   int `json:"max_attempts"`
	LockSeconds   int `json:"lock_seconds"`
}

type loginRateEntry struct {
	Failures    []int64
	LockedUntil int64
}

type LoginRateLimiter struct {
	mu      sync.Mutex
	cfg     LoginRateLimiterConfig
	entries map[string]loginRateEntry
}

type Target struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	ComposeDir        string   `json:"compose_dir"`
	ComposeFile       string   `json:"compose_file"`
	ImageRepo         string   `json:"image_repo"`
	ImageRepos        []string `json:"image_repos,omitempty"`
	AutoUpdateEnabled bool     `json:"auto_update_enabled"`
	Enabled           bool     `json:"enabled"`
	CooldownSeconds   int      `json:"cooldown_seconds"`
	CreatedAt         int64    `json:"created_at"`
	UpdatedAt         int64    `json:"updated_at"`
	LastSuccessAt     *int64   `json:"last_success_at,omitempty"`
}

type Job struct {
	ID           int64   `json:"id"`
	Type         string  `json:"type"`
	Trigger      string  `json:"trigger"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message,omitempty"`
	CreatedAt    int64   `json:"created_at"`
	StartedAt    *int64  `json:"started_at,omitempty"`
	EndedAt      *int64  `json:"ended_at,omitempty"`
	TargetIDs    []int64 `json:"target_ids"`
	DurationSec  float64 `json:"duration_sec,omitempty"`
}

type DashboardPatchEvent struct {
	Seq        int64    `json:"seq"`
	EventType  string   `json:"event_type"`
	Sections   []string `json:"sections"`
	JobID      *int64   `json:"job_id,omitempty"`
	TargetIDs  []int64  `json:"target_ids,omitempty"`
	ReasonCode string   `json:"reason_code,omitempty"`
	OccurredAt int64    `json:"occurred_at"`
}

type DiunEvent struct {
	ID               int64   `json:"id"`
	ImageRepo        string  `json:"image_repo"`
	Tag              string  `json:"tag,omitempty"`
	Digest           string  `json:"digest,omitempty"`
	MatchedTargetIDs []int64 `json:"matched_target_ids"`
	QueuedTargetIDs  []int64 `json:"queued_target_ids"`
	ReceivedAt       int64   `json:"received_at"`
}

type Metrics struct {
	FailedJobsLast24h            int64   `json:"failed_jobs_last_24h"`
	AvgDurationSec24h            float64 `json:"avg_duration_sec_24h"`
	WebhookFailureRate           float64 `json:"webhook_failure_rate_24h"`
	WebhookFailures24h           int64   `json:"webhook_failures_24h"`
	WebhookTotalLast24h          int64   `json:"webhook_total_24h"`
	LoginFailures24h             int64   `json:"login_failures_24h"`
	LoginRateLimited24h          int64   `json:"login_rate_limited_24h"`
	SSEActiveConnections         int64   `json:"sse_active_connections"`
	SSERejectedTotal             int64   `json:"sse_rejected_total"`
	DashboardStreamActive        int64   `json:"dashboard_stream_active"`
	DashboardStreamRejectedTotal int64   `json:"dashboard_stream_rejected_total"`
	PushSent24h                  int64   `json:"push_sent_24h"`
	PushFailed24h                int64   `json:"push_failed_24h"`
}

type TargetAuditSummary struct {
	TargetID       int64   `json:"target_id"`
	Name           string  `json:"name"`
	ImageRepo      string  `json:"image_repo"`
	TotalRuns      int64   `json:"total_runs"`
	SuccessRuns    int64   `json:"success_runs"`
	FailedRuns     int64   `json:"failed_runs"`
	BlockedRuns    int64   `json:"blocked_runs"`
	LastRunAt      *int64  `json:"last_run_at,omitempty"`
	LastSuccessAt  *int64  `json:"last_success_at,omitempty"`
	AvgDurationSec float64 `json:"avg_duration_sec"`
}

type DiscoveredContainer struct {
	Name                string   `json:"name"`
	ComposeDir          string   `json:"compose_dir"`
	ComposeFile         string   `json:"compose_file"`
	ImageRepoCandidates []string `json:"image_repo_candidates"`
}

type apiErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type createTargetRequest struct {
	Name            string   `json:"name"`
	ComposeDir      string   `json:"compose_dir"`
	ComposeFile     string   `json:"compose_file"`
	ImageRepo       string   `json:"image_repo"`
	ImageRepos      []string `json:"image_repos,omitempty"`
	CooldownSeconds *int     `json:"cooldown_seconds,omitempty"`
}

type patchTargetRequest struct {
	AutoUpdateEnabled *bool `json:"auto_update_enabled,omitempty"`
	Enabled           *bool `json:"enabled,omitempty"`
	CooldownSeconds   *int  `json:"cooldown_seconds,omitempty"`
}

type createUpdateJobRequest struct {
	TargetIDs []int64 `json:"target_ids"`
}

type createPruneJobRequest struct {
	Confirm bool `json:"confirm"`
}

type deleteAllJobsRequest struct {
	Confirm bool `json:"confirm"`
}

type authLoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me,omitempty"`
}

type authMeResponse struct {
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
}

type authRateLimitedResponse struct {
	Error             string `json:"error"`
	Code              string `json:"code"`
	RetryAfterSeconds int    `json:"retry_after_seconds"`
}

type diunWebhookResponse struct {
	ImageRepo    string  `json:"image_repo"`
	MatchedCount int     `json:"matched_count"`
	QueuedCount  int     `json:"queued_count"`
	JobID        *int64  `json:"job_id,omitempty"`
	SkippedIDs   []int64 `json:"skipped_ids,omitempty"`
	WindowOpen   bool    `json:"window_open"`
}

type webhookConfigResponse struct {
	Header       string `json:"header"`
	Secret       string `json:"secret,omitempty"`
	SecretMasked string `json:"secret_masked,omitempty"`
	Path         string `json:"path"`
}

type WebhookReceiptSummary struct {
	ID          int64  `json:"id"`
	StatusCode  int    `json:"status_code"`
	ReasonCode  string `json:"reason_code"`
	ReceivedAt  int64  `json:"received_at"`
	QueuedJobID *int64 `json:"queued_job_id,omitempty"`
}

type PushSubscription struct {
	ID            int64  `json:"id"`
	Endpoint      string `json:"endpoint"`
	P256DH        string `json:"p256dh"`
	Auth          string `json:"auth"`
	UA            string `json:"ua"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	LastSuccessAt *int64 `json:"last_success_at,omitempty"`
	LastErrorAt   *int64 `json:"last_error_at,omitempty"`
	Disabled      bool   `json:"disabled"`
}

type PushConfigResponse struct {
	Enabled             bool   `json:"enabled"`
	VAPIDPublicKey      string `json:"vapid_public_key"`
	Subscribed          bool   `json:"subscribed"`
	HasAnySubscriptions bool   `json:"has_any_subscriptions"`
}

type pushSubscriptionRequest struct {
	Endpoint       string `json:"endpoint"`
	ExpirationTime any    `json:"expirationTime,omitempty"`
	Keys           struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type pushTestRequest struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	URL   string `json:"url,omitempty"`
}

func main() {
	cfg := loadConfig()
	if !isAllowedDBPath(cfg.DBPath) {
		log.Fatal("DB_PATH must stay under /data to avoid writing NAS host paths")
	}

	db, err := openDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
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
		assetVersion:     newAssetVersion(),
		appVersion:       resolveBuildVersion(),
	}

	if err := app.initAuthAndWebhookSecret(context.Background()); err != nil {
		log.Fatalf("init auth/webhook secret: %v", err)
	}

	if err := app.recoverInterruptedJobs(); err != nil {
		log.Fatalf("recover interrupted jobs: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go app.worker(ctx)

	mux := http.NewServeMux()
	app.routes(mux)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("composepulse listening on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}

func loadConfig() Config {
	defaultCooldown := 600
	if raw := strings.TrimSpace(os.Getenv("COOLDOWN_SECONDS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			defaultCooldown = v
		}
	}

	sessionTTL := 12 * 60 * 60
	if raw := strings.TrimSpace(os.Getenv("SESSION_TTL_SECONDS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			sessionTTL = v
		}
	}
	rememberMeTTL := 30 * 24 * 60 * 60
	if raw := strings.TrimSpace(os.Getenv("REMEMBER_ME_TTL_SECONDS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			rememberMeTTL = v
		}
	}

	apiTimeout := 15
	if raw := strings.TrimSpace(os.Getenv("API_TIMEOUT_SECONDS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			apiTimeout = v
		}
	}

	sseMax := 20
	if raw := strings.TrimSpace(os.Getenv("SSE_MAX_CONNECTIONS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			sseMax = v
		}
	}
	dashboardSSEMax := 20
	if raw := strings.TrimSpace(os.Getenv("DASHBOARD_SSE_MAX_CONNECTIONS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			dashboardSSEMax = v
		}
	}

	pullRetryMaxAttempts := 3
	if raw := strings.TrimSpace(os.Getenv("PULL_RETRY_MAX_ATTEMPTS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 1 && v <= 10 {
			pullRetryMaxAttempts = v
		}
	}

	pullRetryDelaySec := 2
	if raw := strings.TrimSpace(os.Getenv("PULL_RETRY_DELAY_SECONDS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 && v <= 300 {
			pullRetryDelaySec = v
		}
	}

	autoStart := -1
	if raw := strings.TrimSpace(os.Getenv("AUTO_UPDATE_WINDOW_START_HOUR")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 && v <= 23 {
			autoStart = v
		}
	}
	autoEnd := -1
	if raw := strings.TrimSpace(os.Getenv("AUTO_UPDATE_WINDOW_END_HOUR")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 && v <= 23 {
			autoEnd = v
		}
	}

	loginRateLimit := LoginRateLimiterConfig{
		WindowSeconds: 600,
		MaxAttempts:   5,
		LockSeconds:   900,
	}
	if raw := strings.TrimSpace(os.Getenv("LOGIN_RATE_LIMIT_WINDOW_SECONDS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 24*60*60 {
			loginRateLimit.WindowSeconds = v
		}
	}
	if raw := strings.TrimSpace(os.Getenv("LOGIN_RATE_LIMIT_MAX_ATTEMPTS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 50 {
			loginRateLimit.MaxAttempts = v
		}
	}
	if raw := strings.TrimSpace(os.Getenv("LOGIN_RATE_LIMIT_LOCK_SECONDS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 24*60*60 {
			loginRateLimit.LockSeconds = v
		}
	}

	webPushEnabled := parseBoolEnv(os.Getenv("WEB_PUSH_ENABLED"))
	webPushPublic := strings.TrimSpace(os.Getenv("WEB_PUSH_VAPID_PUBLIC_KEY"))
	webPushPrivate := strings.TrimSpace(os.Getenv("WEB_PUSH_VAPID_PRIVATE_KEY"))
	webPushSubject := normalizeWebPushSubject(os.Getenv("WEB_PUSH_SUBJECT"))
	if webPushSubject == "" {
		webPushSubject = "mailto:admin@example.com"
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	dbPath := strings.TrimSpace(os.Getenv("DB_PATH"))
	if dbPath == "" {
		dbPath = "/data/app.db"
	}

	containerRoot := strings.TrimSpace(os.Getenv("CONTAINER_ROOT"))
	if containerRoot == "" {
		containerRoot = "/share/Container"
	}

	adminUsername := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
	if adminUsername == "" {
		adminUsername = "admin"
	}

	return Config{
		Port:                 port,
		DBPath:               dbPath,
		WebDir:               strings.TrimSpace(os.Getenv("WEB_DIR")),
		DiunWebhookSecret:    strings.TrimSpace(os.Getenv("DIUN_WEBHOOK_SECRET")),
		ShowWebhookSecret:    parseBoolEnv(os.Getenv("WEBHOOK_CONFIG_SHOW_SECRET")),
		DefaultCooldown:      defaultCooldown,
		ContainerRoot:        filepath.Clean(containerRoot),
		AdminUsername:        adminUsername,
		AdminPassword:        strings.TrimSpace(os.Getenv("ADMIN_PASSWORD")),
		SessionTTLSeconds:    sessionTTL,
		RememberMeTTLSeconds: rememberMeTTL,
		APITimeoutSeconds:    apiTimeout,
		SSEMaxConnections:    sseMax,
		DashboardSSEMax:      dashboardSSEMax,
		PullRetryMaxAttempts: pullRetryMaxAttempts,
		PullRetryDelaySec:    pullRetryDelaySec,
		AutoWindowStart:      autoStart,
		AutoWindowEnd:        autoEnd,
		LoginRateLimit:       loginRateLimit,
		WebPushEnabled:       webPushEnabled,
		WebPushVAPIDPublic:   webPushPublic,
		WebPushVAPIDPrivate:  webPushPrivate,
		WebPushSubject:       webPushSubject,
	}
}

func parseBoolEnv(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeWebPushSubject(raw string) string {
	subject := strings.TrimSpace(raw)
	if subject == "" {
		return ""
	}
	lower := strings.ToLower(subject)
	if strings.HasPrefix(lower, "mailto:") {
		addr := strings.TrimSpace(subject[len("mailto:"):])
		addr = strings.ReplaceAll(addr, " ", "")
		if addr == "" {
			return ""
		}
		return "mailto:" + addr
	}
	if strings.Contains(subject, "@") && !strings.Contains(subject, "://") {
		addr := strings.ReplaceAll(strings.TrimSpace(subject), " ", "")
		return "mailto:" + addr
	}
	return subject
}

func openDB(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir %s: %w", dir, err)
	}
	if st, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("stat db dir %s: %w", dir, err)
	} else if !st.IsDir() {
		return nil, fmt.Errorf("db dir is not directory: %s", dir)
	}

	// Preflight write check to produce a clearer startup error on host-mounted volumes.
	probePath := filepath.Join(dir, ".db-write-probe")
	probe, err := os.OpenFile(probePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("db dir not writable: %s: %w", dir, err)
	}
	_ = probe.Close()
	_ = os.Remove(probePath)

	// Ensure DB file path itself can be created before SQLite initialization.
	dbFile, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create/open db file %s: %w", path, err)
	}
	_ = dbFile.Close()

	dsn := fmt.Sprintf("%s?_busy_timeout=5000&_foreign_keys=1", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if _, err := db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite init journal_mode WAL %s: %w", path, err)
	}
	if _, err := db.Exec(`PRAGMA synchronous=NORMAL;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite init synchronous NORMAL %s: %w", path, err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS targets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			compose_dir TEXT NOT NULL UNIQUE,
			compose_file TEXT NOT NULL,
			image_repo TEXT NOT NULL,
			auto_update_enabled INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			cooldown_seconds INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_success_at INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_targets_image_repo ON targets(image_repo);`,
		`CREATE TABLE IF NOT EXISTS target_image_repos (
			target_id INTEGER NOT NULL,
			image_repo TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			PRIMARY KEY (target_id, image_repo),
			FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_target_image_repos_repo ON target_image_repos(image_repo);`,
		`CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			trigger TEXT NOT NULL,
			status TEXT NOT NULL,
			error_message TEXT,
			created_at INTEGER NOT NULL,
			started_at INTEGER,
			ended_at INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS job_targets (
			job_id INTEGER NOT NULL,
			target_id INTEGER NOT NULL,
			position INTEGER NOT NULL,
			PRIMARY KEY (job_id, target_id),
			FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE,
			FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_job_targets_target_id ON job_targets(target_id);`,
		`CREATE TABLE IF NOT EXISTS job_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			line TEXT NOT NULL,
			FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_job_logs_job_id_id ON job_logs(job_id, id);`,
		`CREATE TABLE IF NOT EXISTS diun_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			image_repo TEXT NOT NULL,
			tag TEXT,
			digest TEXT,
			matched_target_ids TEXT NOT NULL,
			queued_target_ids TEXT NOT NULL,
			received_at INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS webhook_receipts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				received_at INTEGER NOT NULL,
				success INTEGER NOT NULL,
				error TEXT NOT NULL
			);`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_receipts_received_at ON webhook_receipts(received_at DESC);`,
		`CREATE TABLE IF NOT EXISTS auth_login_events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				created_at INTEGER NOT NULL,
				success INTEGER NOT NULL,
				rate_limited INTEGER NOT NULL,
				username TEXT NOT NULL,
				client_ip TEXT NOT NULL
			);`,
		`CREATE INDEX IF NOT EXISTS idx_auth_login_events_created_at ON auth_login_events(created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS push_subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				endpoint TEXT NOT NULL UNIQUE,
				p256dh TEXT NOT NULL,
				auth TEXT NOT NULL,
				ua TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL,
				last_success_at INTEGER,
				last_error_at INTEGER,
				disabled INTEGER NOT NULL DEFAULT 0
			);`,
		`CREATE INDEX IF NOT EXISTS idx_push_subscriptions_disabled ON push_subscriptions(disabled);`,
		`CREATE TABLE IF NOT EXISTS push_delivery_logs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				created_at INTEGER NOT NULL,
				success INTEGER NOT NULL,
				error TEXT NOT NULL
			);`,
		`CREATE INDEX IF NOT EXISTS idx_push_delivery_logs_created_at ON push_delivery_logs(created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS app_settings (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL,
				updated_at INTEGER NOT NULL
			);`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := ensureColumnExists(db, "webhook_receipts", "status_code", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumnExists(db, "webhook_receipts", "reason_code", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumnExists(db, "webhook_receipts", "queued_job_id", "INTEGER"); err != nil {
		return err
	}
	if _, err := db.Exec(`
			INSERT INTO target_image_repos (target_id, image_repo, created_at)
			SELECT t.id, t.image_repo, t.created_at
			FROM targets t
		WHERE TRIM(t.image_repo) <> ''
			AND NOT EXISTS (
				SELECT 1
				FROM target_image_repos tir
				WHERE tir.target_id = t.id AND tir.image_repo = t.image_repo
			)`); err != nil {
		return err
	}
	if err := normalizeStoredTargetImageRepos(db); err != nil {
		return err
	}
	return nil
}

func normalizeStoredTargetImageRepos(db *sql.DB) error {
	type targetRow struct {
		ID        int64
		ImageRepo string
		CreatedAt int64
	}
	type repoRow struct {
		TargetID  int64
		ImageRepo string
		CreatedAt int64
	}

	targets := make([]targetRow, 0, 32)
	rows, err := db.Query(`SELECT id, image_repo, created_at FROM targets`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var row targetRow
		if err := rows.Scan(&row.ID, &row.ImageRepo, &row.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		targets = append(targets, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	repos := make([]repoRow, 0, 64)
	rows, err = db.Query(`SELECT target_id, image_repo, created_at FROM target_image_repos`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var row repoRow
		if err := rows.Scan(&row.TargetID, &row.ImageRepo, &row.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		repos = append(repos, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, target := range targets {
		normRepo := normalizeImageRepo(target.ImageRepo)
		if normRepo == "" || normRepo == target.ImageRepo {
			continue
		}
		if _, err := tx.Exec(`UPDATE targets SET image_repo = ? WHERE id = ?`, normRepo, target.ID); err != nil {
			return err
		}
	}

	for _, repo := range repos {
		normRepo := normalizeImageRepo(repo.ImageRepo)
		if normRepo == "" {
			continue
		}
		if normRepo != repo.ImageRepo {
			if _, err := tx.Exec(`
				INSERT OR IGNORE INTO target_image_repos (target_id, image_repo, created_at)
				VALUES (?, ?, ?)`,
				repo.TargetID, normRepo, repo.CreatedAt,
			); err != nil {
				return err
			}
			if _, err := tx.Exec(`
				DELETE FROM target_image_repos
				WHERE target_id = ? AND image_repo = ?`,
				repo.TargetID, repo.ImageRepo,
			); err != nil {
				return err
			}
		}
	}

	for _, target := range targets {
		normRepo := normalizeImageRepo(target.ImageRepo)
		if normRepo == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO target_image_repos (target_id, image_repo, created_at)
			VALUES (?, ?, ?)`,
			target.ID, normRepo, target.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func ensureColumnExists(db *sql.DB, tableName, columnName, columnDef string) error {
	exists, err := columnExists(db, tableName, columnName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef)
	if _, err := db.Exec(stmt); err != nil {
		return err
	}
	return nil
}

func columnExists(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notNull int
		var dfltValue any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(columnName)) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func (a *App) routes(mux *http.ServeMux) {
	staticFS, err := resolveWebFS(a.cfg.WebDir)
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	if strings.TrimSpace(a.cfg.WebDir) != "" {
		log.Printf("serving web assets from WEB_DIR=%s", a.cfg.WebDir)
	}
	staticHandler := http.FileServer(http.FS(staticFS))
	staticHandler = withStaticAssetCacheHeaders(staticHandler)
	apiTimeout := time.Duration(a.cfg.APITimeoutSeconds) * time.Second

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeAPIError(w, http.StatusNotFound, "not_found", "not found")
			return
		}
		if r.URL.Path == "/" {
			if _, ok := a.currentSessionUser(r); !ok {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			if err := serveVersionedHTML(w, r, staticFS, "index.html", a.assetVersion, a.appVersion); err != nil {
				writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			}
			return
		}
		if r.URL.Path == "/login" || r.URL.Path == "/login/" {
			if _, ok := a.currentSessionUser(r); ok {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
			if err := serveVersionedHTML(w, r, staticFS, "login.html", a.assetVersion, a.appVersion); err != nil {
				writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			}
			return
		}
		staticHandler.ServeHTTP(w, r)
	})

	mux.Handle("/api/health", withAPITimeout(http.HandlerFunc(a.handleHealth), apiTimeout))
	mux.Handle("/api/auth/login", withAPITimeout(http.HandlerFunc(a.handleAuthLogin), apiTimeout))
	mux.Handle("/api/auth/logout", withAPITimeout(http.HandlerFunc(a.handleAuthLogout), apiTimeout))
	mux.Handle("/api/auth/me", withAPITimeout(http.HandlerFunc(a.handleAuthMe), apiTimeout))
	mux.Handle("/api/metrics", withAPITimeout(http.HandlerFunc(a.handleMetrics), apiTimeout))
	mux.Handle("/api/containers/discover", withAPITimeout(http.HandlerFunc(a.handleDiscoverContainers), apiTimeout))
	mux.Handle("/api/targets", withAPITimeout(http.HandlerFunc(a.handleTargets), apiTimeout))
	mux.Handle("/api/targets/", withAPITimeout(http.HandlerFunc(a.handleTargetByID), apiTimeout))
	mux.Handle("/api/jobs/update", withAPITimeout(http.HandlerFunc(a.handleCreateUpdateJob), apiTimeout))
	mux.Handle("/api/jobs/prune", withAPITimeout(http.HandlerFunc(a.handleCreatePruneJob), apiTimeout))
	mux.Handle("/api/jobs/delete-all", withAPITimeout(http.HandlerFunc(a.handleDeleteAllJobs), apiTimeout))
	mux.Handle("/api/jobs/export.csv", withAPITimeout(http.HandlerFunc(a.handleJobsExportCSV), apiTimeout))
	mux.Handle("/api/jobs", withAPITimeout(http.HandlerFunc(a.handleJobs), apiTimeout))
	mux.HandleFunc("/api/jobs/", a.handleJobByIDOrStream)
	mux.HandleFunc("/api/stream/dashboard", a.handleDashboardStream)
	mux.Handle("/api/audit/targets", withAPITimeout(http.HandlerFunc(a.handleTargetAudit), apiTimeout))
	mux.Handle("/api/diun/events", withAPITimeout(http.HandlerFunc(a.handleDiunEvents), apiTimeout))
	mux.Handle("/api/diun/receipts", withAPITimeout(http.HandlerFunc(a.handleDiunReceipts), apiTimeout))
	mux.Handle("/api/diun/webhook/config", withAPITimeout(http.HandlerFunc(a.handleDiunWebhookConfig), apiTimeout))
	mux.Handle("/api/diun/webhook", withAPITimeout(http.HandlerFunc(a.handleDiunWebhook), apiTimeout))
	mux.Handle("/api/push/config", withAPITimeout(http.HandlerFunc(a.handlePushConfig), apiTimeout))
	mux.Handle("/api/push/subscriptions", withAPITimeout(http.HandlerFunc(a.handlePushSubscriptions), apiTimeout))
	mux.Handle("/api/push/test", withAPITimeout(http.HandlerFunc(a.handlePushTest), apiTimeout))
}

func resolveWebFS(webDir string) (fs.FS, error) {
	if strings.TrimSpace(webDir) == "" {
		staticFS, err := fs.Sub(webFS, "web")
		if err != nil {
			return nil, err
		}
		return staticFS, nil
	}

	clean := filepath.Clean(webDir)
	st, err := os.Stat(clean)
	if err != nil {
		return nil, fmt.Errorf("WEB_DIR stat failed: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("WEB_DIR is not a directory: %s", clean)
	}
	return os.DirFS(clean), nil
}

func serveVersionedHTML(w http.ResponseWriter, r *http.Request, staticFS fs.FS, name, assetVersion, appVersion string) error {
	raw, err := fs.ReadFile(staticFS, name)
	if err != nil {
		return err
	}
	asset := strings.TrimSpace(assetVersion)
	if asset == "" {
		asset = "dev"
	}
	version := html.EscapeString(resolveBuildVersionValue(appVersion))
	body := bytes.ReplaceAll(raw, []byte("__ASSET_VERSION__"), []byte(asset))
	body = bytes.ReplaceAll(body, []byte("__APP_VERSION__"), []byte(version))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, name, time.Now(), bytes.NewReader(body))
	return nil
}

func withStaticAssetCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.ToLower(strings.TrimSpace(r.URL.Path))
		switch {
		case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".css"), strings.HasSuffix(path, ".webmanifest"), strings.HasSuffix(path, ".svg"), strings.HasSuffix(path, ".png"), strings.HasSuffix(path, ".ico"):
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if err := a.db.PingContext(r.Context()); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "db_unavailable", "db unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req authLoginRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
		return
	}
	username := strings.TrimSpace(req.Username)
	clientIP := clientIPFromRequest(r)
	limiterKey := buildLoginRateLimitKey(clientIP, username)

	now := time.Now()
	if a.loginLimiter != nil {
		if blocked, retryAfterSec := a.loginLimiter.Check(limiterKey, now); blocked {
			if err := a.recordAuthLoginEvent(r.Context(), now.Unix(), false, true, username, clientIP); err != nil {
				log.Printf("record auth login rate-limited event failed: %v", err)
			}
			a.emitDashboardEvent(DashboardPatchEvent{
				EventType:  eventTypeAuthRateLimited,
				Sections:   []string{dashboardSectionMetrics},
				ReasonCode: "auth_rate_limited",
			})
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
			writeJSON(w, http.StatusTooManyRequests, authRateLimitedResponse{
				Error:             "too many login attempts, try again later",
				Code:              "auth_rate_limited",
				RetryAfterSeconds: retryAfterSec,
			})
			return
		}
	}

	if !a.validateCredentials(req.Username, req.Password) {
		if a.loginLimiter != nil {
			a.loginLimiter.RecordFailure(limiterKey, now)
		}
		if err := a.recordAuthLoginEvent(r.Context(), now.Unix(), false, false, username, clientIP); err != nil {
			log.Printf("record auth login failure event failed: %v", err)
		}
		writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	if a.loginLimiter != nil {
		a.loginLimiter.Reset(limiterKey)
	}
	if err := a.recordAuthLoginEvent(r.Context(), now.Unix(), true, false, username, clientIP); err != nil {
		log.Printf("record auth login success event failed: %v", err)
	}

	ttl := time.Duration(a.cfg.SessionTTLSeconds) * time.Second
	if req.RememberMe {
		ttl = time.Duration(a.cfg.RememberMeTTLSeconds) * time.Second
	}
	sessionToken, expiresAt, err := a.sessions.CreateWithTTL(a.cfg.AdminUsername, ttl)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "session_create_failed", "failed to create session")
		return
	}
	setSessionCookie(w, r, sessionToken, expiresAt)
	writeJSON(w, http.StatusOK, authMeResponse{
		Authenticated: true,
		Username:      a.cfg.AdminUsername,
	})
}

func (a *App) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.sessions.Delete(cookie.Value)
	}
	clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	username, ok := a.currentSessionUser(r)
	if !ok {
		writeJSON(w, http.StatusOK, authMeResponse{Authenticated: false})
		return
	}
	writeJSON(w, http.StatusOK, authMeResponse{
		Authenticated: true,
		Username:      username,
	})
}

func (a *App) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	since := time.Now().Unix() - 24*60*60
	var failedJobs int64
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM jobs WHERE created_at >= ? AND status IN (?, ?)`,
		since, statusFailed, statusBlocked,
	).Scan(&failedJobs); err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}

	var avgDuration sql.NullFloat64
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT AVG(ended_at - started_at) FROM jobs WHERE created_at >= ? AND status = ? AND started_at IS NOT NULL AND ended_at IS NOT NULL`,
		since, statusSuccess,
	).Scan(&avgDuration); err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}

	var webhookTotal int64
	var webhookFailed int64
	if err := queryCountWithMissingTableFallback(r.Context(), a.db, &webhookTotal,
		`SELECT COUNT(*) FROM webhook_receipts WHERE received_at >= ?`,
		since,
	); err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	if err := queryCountWithMissingTableFallback(r.Context(), a.db, &webhookFailed,
		`SELECT COUNT(*) FROM webhook_receipts WHERE received_at >= ? AND success = 0`,
		since,
	); err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}

	rate := 0.0
	if webhookTotal > 0 {
		rate = float64(webhookFailed) / float64(webhookTotal)
	}

	var loginFailures int64
	var loginRateLimited int64
	if err := queryCountWithMissingTableFallback(r.Context(), a.db, &loginFailures,
		`SELECT COUNT(*) FROM auth_login_events WHERE created_at >= ? AND success = 0 AND rate_limited = 0`,
		since,
	); err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	if err := queryCountWithMissingTableFallback(r.Context(), a.db, &loginRateLimited,
		`SELECT COUNT(*) FROM auth_login_events WHERE created_at >= ? AND rate_limited = 1`,
		since,
	); err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}

	avg := 0.0
	if avgDuration.Valid {
		avg = math.Round(avgDuration.Float64*100) / 100
	}
	sseActive, sseRejected := int64(0), int64(0)
	if a.sseLimiter != nil {
		sseActive, sseRejected = a.sseLimiter.Stats()
	}
	dashboardActive, dashboardRejected := int64(0), int64(0)
	if a.dashboardLimiter != nil {
		dashboardActive, dashboardRejected = a.dashboardLimiter.Stats()
	}

	var pushSent int64
	var pushFailed int64
	if err := queryCountWithMissingTableFallback(r.Context(), a.db, &pushSent,
		`SELECT COUNT(*) FROM push_delivery_logs WHERE created_at >= ? AND success = 1`,
		since,
	); err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	if err := queryCountWithMissingTableFallback(r.Context(), a.db, &pushFailed,
		`SELECT COUNT(*) FROM push_delivery_logs WHERE created_at >= ? AND success = 0`,
		since,
	); err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}

	writeJSON(w, http.StatusOK, Metrics{
		FailedJobsLast24h:            failedJobs,
		AvgDurationSec24h:            avg,
		WebhookFailureRate:           rate,
		WebhookFailures24h:           webhookFailed,
		WebhookTotalLast24h:          webhookTotal,
		LoginFailures24h:             loginFailures,
		LoginRateLimited24h:          loginRateLimited,
		SSEActiveConnections:         sseActive,
		SSERejectedTotal:             sseRejected,
		DashboardStreamActive:        dashboardActive,
		DashboardStreamRejectedTotal: dashboardRejected,
		PushSent24h:                  pushSent,
		PushFailed24h:                pushFailed,
	})
}

func queryCountWithMissingTableFallback(ctx context.Context, db *sql.DB, dest *int64, query string, args ...any) error {
	err := db.QueryRowContext(ctx, query, args...).Scan(dest)
	if err == nil {
		return nil
	}
	if isSQLiteMissingTableErr(err) {
		*dest = 0
		log.Printf("metrics fallback: treating missing table as zero for query %q: %v", query, err)
		return nil
	}
	return err
}

func (a *App) handleDiscoverContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	items, err := a.discoverContainers()
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) handleTargets(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		targets, err := a.listTargets(r.Context())
		if err != nil {
			writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"targets": targets})
	case http.MethodPost:
		var req createTargetRequest
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
			return
		}

		target, err := a.createTarget(r.Context(), req)
		if err != nil {
			if errors.Is(err, errConflict) {
				writeAPIErrorFromErr(w, http.StatusConflict, "conflict", err)
				return
			}
			writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
			return
		}
		a.emitDashboardEvent(DashboardPatchEvent{
			EventType: eventTypeTargetCreated,
			Sections:  []string{dashboardSectionTargets, dashboardSectionAudit, dashboardSectionMetrics},
			TargetIDs: []int64{target.ID},
		})
		writeJSON(w, http.StatusCreated, target)
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *App) handleTargetByID(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}

	id, ok := parseIDPath(r.URL.Path, "/api/targets/")
	if !ok {
		writeAPIError(w, http.StatusNotFound, "not_found", "not found")
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var req patchTargetRequest
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
			return
		}
		target, err := a.patchTarget(r.Context(), id, req)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeAPIError(w, http.StatusNotFound, "not_found", "not found")
				return
			}
			writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
			return
		}
		a.emitDashboardEvent(DashboardPatchEvent{
			EventType: eventTypeTargetUpdated,
			Sections:  []string{dashboardSectionTargets, dashboardSectionAudit, dashboardSectionMetrics},
			TargetIDs: []int64{id},
		})
		writeJSON(w, http.StatusOK, target)
	case http.MethodDelete:
		if err := a.deleteTarget(r.Context(), id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeAPIError(w, http.StatusNotFound, "not_found", "not found")
				return
			}
			if errors.Is(err, errTargetBusy) {
				writeAPIErrorFromErr(w, http.StatusConflict, "target_has_active_jobs", err)
				return
			}
			writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			return
		}
		a.emitDashboardEvent(DashboardPatchEvent{
			EventType: eventTypeTargetDeleted,
			Sections:  []string{dashboardSectionTargets, dashboardSectionAudit, dashboardSectionMetrics},
			TargetIDs: []int64{id},
		})
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *App) handleCreateUpdateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	var req createUpdateJobRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
		return
	}
	if len(req.TargetIDs) == 0 {
		writeAPIError(w, http.StatusBadRequest, "missing_target_ids", "target_ids is required")
		return
	}

	targetIDs, err := a.filterEnabledTargets(r.Context(), req.TargetIDs)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
		return
	}
	if len(targetIDs) == 0 {
		writeAPIError(w, http.StatusBadRequest, "no_enabled_targets", "no enabled targets selected")
		return
	}

	jobID, err := a.createJob(r.Context(), jobTypeUpdate, triggerManual, targetIDs)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	if err := a.enqueueJob(jobID); err != nil {
		_ = a.failJob(context.Background(), jobID, "queue full")
		writeAPIErrorFromErr(w, http.StatusServiceUnavailable, "queue_full", err)
		return
	}
	a.emitDashboardEvent(DashboardPatchEvent{
		EventType: eventTypeJobQueued,
		Sections:  []string{dashboardSectionJobs, dashboardSectionMetrics},
		JobID:     &jobID,
		TargetIDs: targetIDs,
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": statusQueued})
}

func (a *App) handleCreatePruneJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	var req createPruneJobRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
		return
	}
	if !req.Confirm {
		writeAPIError(w, http.StatusBadRequest, "missing_confirmation", "confirm=true required")
		return
	}

	jobID, err := a.createJob(r.Context(), jobTypePrune, triggerManual, nil)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	if err := a.enqueueJob(jobID); err != nil {
		_ = a.failJob(context.Background(), jobID, "queue full")
		writeAPIErrorFromErr(w, http.StatusServiceUnavailable, "queue_full", err)
		return
	}
	a.emitDashboardEvent(DashboardPatchEvent{
		EventType: eventTypeJobQueued,
		Sections:  []string{dashboardSectionJobs, dashboardSectionMetrics},
		JobID:     &jobID,
	})
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": statusQueued})
}

func (a *App) handleDeleteAllJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	var req deleteAllJobsRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
		return
	}
	if !req.Confirm {
		writeAPIError(w, http.StatusBadRequest, "missing_confirmation", "confirm=true required")
		return
	}

	deletedCount, err := a.deleteAllJobs(r.Context())
	if err != nil {
		if errors.Is(err, errJobBusy) {
			writeAPIErrorFromErr(w, http.StatusConflict, "job_active", err)
			return
		}
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	a.emitDashboardEvent(DashboardPatchEvent{
		EventType: "jobs_deleted_all",
		Sections:  []string{dashboardSectionJobs, dashboardSectionMetrics, dashboardSectionAudit},
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":       true,
		"deleted_count": deletedCount,
	})
}

func (a *App) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 200 {
			limit = v
		}
	}
	var cursor *int64
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_cursor", "cursor must be positive integer")
			return
		}
		cursor = &v
	}

	jobs, nextCursor, err := a.listJobs(r.Context(), limit, cursor)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	resp := map[string]any{"jobs": jobs}
	if nextCursor != nil {
		resp["next_cursor"] = *nextCursor
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleJobByIDOrStream(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/stream") {
		a.handleJobStream(w, r)
		return
	}
	a.handleJobByID(w, r)
}

func (a *App) handleJobByID(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}

	id, ok := parseIDPath(r.URL.Path, "/api/jobs/")
	if !ok {
		writeAPIError(w, http.StatusNotFound, "not_found", "not found")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if err := a.deleteJob(r.Context(), id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeAPIError(w, http.StatusNotFound, "not_found", "not found")
				return
			}
			if errors.Is(err, errJobBusy) {
				writeAPIErrorFromErr(w, http.StatusConflict, "job_active", err)
				return
			}
			writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			return
		}
		a.emitDashboardEvent(DashboardPatchEvent{
			EventType: "job_deleted",
			Sections:  []string{dashboardSectionJobs, dashboardSectionMetrics, dashboardSectionAudit},
			JobID:     &id,
		})
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *App) handleJobsExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	limit := 1000
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 || v > 5000 {
			writeAPIError(w, http.StatusBadRequest, "invalid_limit", "limit must be 1..5000")
			return
		}
		limit = v
	}

	jobs, _, err := a.listJobs(r.Context(), limit, nil)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}

	fileName := fmt.Sprintf("jobs-%s.csv", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))

	writer := csv.NewWriter(w)
	header := []string{
		"id",
		"type",
		"trigger",
		"status",
		"target_ids",
		"duration_sec",
		"error_message",
		"created_at",
		"started_at",
		"ended_at",
	}
	if err := writer.Write(header); err != nil {
		return
	}

	for _, job := range jobs {
		startedAt := ""
		if job.StartedAt != nil {
			startedAt = strconv.FormatInt(*job.StartedAt, 10)
		}
		endedAt := ""
		if job.EndedAt != nil {
			endedAt = strconv.FormatInt(*job.EndedAt, 10)
		}
		duration := ""
		if job.StartedAt != nil && job.EndedAt != nil && *job.EndedAt >= *job.StartedAt {
			duration = strconv.FormatFloat(float64(*job.EndedAt-*job.StartedAt), 'f', 1, 64)
		}
		errorMessage := ""
		if job.ErrorMessage != nil {
			errorMessage = *job.ErrorMessage
		}
		row := []string{
			strconv.FormatInt(job.ID, 10),
			job.Type,
			job.Trigger,
			job.Status,
			joinInt64(job.TargetIDs, ","),
			duration,
			errorMessage,
			strconv.FormatInt(job.CreatedAt, 10),
			startedAt,
			endedAt,
		}
		if err := writer.Write(row); err != nil {
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Printf("csv export flush failed: %v", err)
	}
}

func (a *App) handleTargetAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	summaries, err := a.listTargetAuditSummaries(r.Context())
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"summaries": summaries})
}

func (a *App) handleJobStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	jobID, ok := parseStreamJobID(r.URL.Path)
	if !ok {
		writeAPIError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	exists, err := a.jobExists(r.Context(), jobID)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	if !exists {
		writeAPIError(w, http.StatusNotFound, "not_found", "not found")
		return
	}

	if !a.sseLimiter.Acquire() {
		writeAPIError(w, http.StatusTooManyRequests, "sse_limit_reached", "too many active stream connections")
		return
	}
	log.Printf("sse stream opened: job=%d active=%d", jobID, a.sseLimiter.Active())
	defer func() {
		a.sseLimiter.Release()
		log.Printf("sse stream closed: job=%d active=%d", jobID, a.sseLimiter.Active())
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "stream_unsupported", "stream unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	logs, err := a.getJobLogs(r.Context(), jobID)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}

	for _, line := range logs {
		if err := writeSSE(w, "log", map[string]string{"line": line}); err != nil {
			return
		}
	}
	flusher.Flush()

	done, status := a.jobDone(r.Context(), jobID)
	if done {
		_ = writeSSE(w, "done", map[string]string{"status": status})
		flusher.Flush()
		return
	}

	sub, unsubscribe := a.logBroker.Subscribe(jobID)
	defer unsubscribe()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case line := <-sub:
			if err := writeSSE(w, "log", map[string]string{"line": line}); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			doneNow, st := a.jobDone(r.Context(), jobID)
			if doneNow {
				_ = writeSSE(w, "done", map[string]string{"status": st})
				flusher.Flush()
				return
			}
			flusher.Flush()
		}
	}
}

func (a *App) handleDashboardStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}
	if !a.dashboardLimiter.Acquire() {
		writeAPIError(w, http.StatusTooManyRequests, "dashboard_stream_limit_reached", "too many active dashboard streams")
		return
	}
	log.Printf("dashboard stream opened: active=%d", a.dashboardLimiter.Active())
	defer func() {
		a.dashboardLimiter.Release()
		log.Printf("dashboard stream closed: active=%d", a.dashboardLimiter.Active())
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "stream_unsupported", "stream unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	initial := DashboardPatchEvent{
		EventType: "snapshot",
		Sections: []string{
			dashboardSectionTargets,
			dashboardSectionJobs,
			dashboardSectionMetrics,
			dashboardSectionAudit,
			dashboardSectionEvents,
			dashboardSectionReceipts,
		},
		OccurredAt: time.Now().Unix(),
	}
	if err := writeSSE(w, "patch", initial); err != nil {
		return
	}
	flusher.Flush()

	sub, unsubscribe := a.dashboardBroker.Subscribe()
	defer unsubscribe()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt := <-sub:
			if err := writeSSE(w, "patch", evt); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (a *App) handlePushConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}
	subscribed := false
	hasAnySubscriptions := false
	if a.webPushConfigured() {
		var err error
		hasAnySubscriptions, err = a.hasActivePushSubscription(r.Context())
		if err != nil {
			writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			return
		}
		endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))
		if endpoint != "" {
			subscribed, err = a.hasActivePushSubscriptionByEndpoint(r.Context(), endpoint)
			if err != nil {
				writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, PushConfigResponse{
		Enabled:             a.webPushConfigured(),
		VAPIDPublicKey:      a.cfg.WebPushVAPIDPublic,
		Subscribed:          subscribed,
		HasAnySubscriptions: hasAnySubscriptions,
	})
}

func (a *App) handlePushSubscriptions(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}
	if !a.webPushConfigured() {
		writeAPIError(w, http.StatusServiceUnavailable, "push_disabled", "web push is disabled")
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req pushSubscriptionRequest
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
			return
		}
		endpoint := strings.TrimSpace(req.Endpoint)
		p256dh := strings.TrimSpace(req.Keys.P256DH)
		auth := strings.TrimSpace(req.Keys.Auth)
		if endpoint == "" || p256dh == "" || auth == "" {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "endpoint and keys are required")
			return
		}
		sub, err := a.upsertPushSubscription(r.Context(), endpoint, p256dh, auth, strings.TrimSpace(r.UserAgent()))
		if err != nil {
			writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscribed": true, "subscription": sub})
	case http.MethodDelete:
		endpoint := strings.TrimSpace(r.URL.Query().Get("endpoint"))
		if endpoint == "" && r.ContentLength > 0 {
			var req pushSubscriptionRequest
			if err := decodeJSON(r.Body, &req); err != nil {
				writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
				return
			}
			endpoint = strings.TrimSpace(req.Endpoint)
		}
		if endpoint == "" {
			disabledCount, err := a.disableAllPushSubscriptions(r.Context())
			if err != nil {
				writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"subscribed": false, "disabled_count": disabledCount})
			return
		}
		disabledCount, err := a.disablePushSubscriptionByEndpoint(r.Context(), endpoint)
		if err != nil {
			writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscribed": false, "disabled_count": disabledCount})
	default:
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (a *App) handlePushTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}
	if !a.webPushConfigured() {
		writeAPIError(w, http.StatusServiceUnavailable, "push_disabled", "web push is disabled")
		return
	}

	req := pushTestRequest{}
	if r.ContentLength > 0 {
		if err := decodeJSON(r.Body, &req); err != nil {
			writeAPIErrorFromErr(w, http.StatusBadRequest, "invalid_request", err)
			return
		}
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "ComposePulse"
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		body = "Push test notification"
	}
	urlPath := strings.TrimSpace(req.URL)
	if urlPath == "" {
		urlPath = "/"
	}

	sent, failed, err := a.sendPushNotification(r.Context(), pushMessage{
		Title:     title,
		Body:      body,
		URL:       urlPath,
		EventType: "push_test",
		Tag:       "push-test",
	})
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	lastError := ""
	if failed > 0 {
		if v, ferr := a.latestPushFailure(r.Context()); ferr == nil {
			lastError = v
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"sent_count":   sent,
		"failed_count": failed,
		"last_error":   lastError,
	})
}

func (a *App) handleDiunEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}
	events, err := a.listDiunEvents(r.Context(), 30)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (a *App) handleDiunReceipts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 || v > 200 {
			writeAPIError(w, http.StatusBadRequest, "invalid_limit", "limit must be 1..200")
			return
		}
		limit = v
	}
	receipts, err := a.listWebhookReceipts(r.Context(), limit)
	if err != nil {
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"receipts": receipts})
}

func (a *App) handleDiunWebhookConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !a.requireSession(w, r) {
		return
	}
	resp := webhookConfigResponse{
		Header:       "X-DIUN-SECRET",
		SecretMasked: maskSecret(a.webhookSecret),
		Path:         "/api/diun/webhook",
	}
	if a.cfg.ShowWebhookSecret {
		resp.Secret = a.webhookSecret
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *App) handleDiunWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	now := time.Now().Unix()
	recordReceipt := func(success bool, statusCode int, reasonCode, message string, queuedJobID *int64) {
		if err := a.recordWebhookReceipt(r.Context(), now, success, statusCode, reasonCode, message, queuedJobID); err != nil {
			log.Printf("record webhook receipt failed: %v", err)
		}
	}

	if a.webhookSecret != "" {
		incoming := strings.TrimSpace(r.Header.Get("X-DIUN-SECRET"))
		if subtleCompare(incoming, a.webhookSecret) == 0 {
			recordReceipt(false, http.StatusUnauthorized, webhookReasonSecretMismatch, "invalid secret", nil)
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
			return
		}
	}

	var payload map[string]any
	if err := decodeJSON(r.Body, &payload); err != nil {
		recordReceipt(false, http.StatusBadRequest, webhookReasonPayloadInvalid, "invalid json", nil)
		writeAPIError(w, http.StatusBadRequest, "invalid_payload", "invalid payload")
		return
	}

	imageRepo, tag, digest := extractDiunFields(payload)
	if imageRepo == "" {
		if isDiunTestPayload(payload) {
			testRepo := "__diun_test__"
			if err := a.recordDiunEvent(r.Context(), testRepo, tag, digest, nil, nil, now); err != nil {
				log.Printf("record diun test event failed: %v", err)
			}
			recordReceipt(true, http.StatusOK, webhookReasonNoMatch, "test payload", nil)
			writeJSON(w, http.StatusOK, diunWebhookResponse{
				ImageRepo:    testRepo,
				MatchedCount: 0,
				QueuedCount:  0,
				SkippedIDs:   nil,
				WindowOpen:   a.withinAutoUpdateWindow(time.Now()),
			})
			return
		}
		recordReceipt(false, http.StatusBadRequest, webhookReasonPayloadInvalid, "missing image repository", nil)
		writeAPIError(w, http.StatusBadRequest, "missing_image_repository", "image repository is required")
		return
	}

	targets, err := a.findTargetsByImageRepo(r.Context(), imageRepo)
	if err != nil {
		recordReceipt(false, http.StatusInternalServerError, webhookReasonInternalError, err.Error(), nil)
		writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
		return
	}

	matchedIDs := make([]int64, 0, len(targets))
	queuedIDs := make([]int64, 0, len(targets))
	skippedIDs := make([]int64, 0, len(targets))
	eligibleTargets := make([]Target, 0, len(targets))
	windowOpen := a.withinAutoUpdateWindow(time.Now())
	for _, t := range targets {
		matchedIDs = append(matchedIDs, t.ID)
		if !t.AutoUpdateEnabled || !t.Enabled {
			skippedIDs = append(skippedIDs, t.ID)
			continue
		}
		eligibleTargets = append(eligibleTargets, t)
	}

	reasonCode := webhookReasonNoMatch
	if len(matchedIDs) > 0 {
		reasonCode = webhookReasonAutoDisabled
	}
	if len(eligibleTargets) > 0 && !windowOpen {
		reasonCode = webhookReasonOutsideMaintenanceWindow
		for _, t := range eligibleTargets {
			skippedIDs = append(skippedIDs, t.ID)
		}
	}
	if len(eligibleTargets) > 0 && windowOpen {
		reasonCode = webhookReasonCooldownBlocked
		for _, t := range eligibleTargets {
			cooldown := t.CooldownSeconds
			if cooldown <= 0 {
				cooldown = a.cfg.DefaultCooldown
			}
			inCooldown, err := a.targetInCooldown(r.Context(), t.ID, cooldown)
			if err != nil {
				recordReceipt(false, http.StatusInternalServerError, webhookReasonInternalError, err.Error(), nil)
				writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
				return
			}
			if inCooldown {
				skippedIDs = append(skippedIDs, t.ID)
				continue
			}
			queuedIDs = append(queuedIDs, t.ID)
		}
	}

	var jobID *int64
	if len(queuedIDs) > 0 {
		id, err := a.createJob(r.Context(), jobTypeUpdate, triggerAuto, queuedIDs)
		if err != nil {
			recordReceipt(false, http.StatusInternalServerError, webhookReasonInternalError, err.Error(), nil)
			writeAPIErrorFromErr(w, http.StatusInternalServerError, "internal_error", err)
			return
		}
		if err := a.enqueueJob(id); err != nil {
			_ = a.failJob(context.Background(), id, "queue full")
			recordReceipt(false, http.StatusServiceUnavailable, webhookReasonQueueFull, "queue full", nil)
			writeAPIErrorFromErr(w, http.StatusServiceUnavailable, "queue_full", err)
			return
		}
		jobID = &id
		reasonCode = webhookReasonQueued
		a.emitDashboardEvent(DashboardPatchEvent{
			EventType:  eventTypeJobQueued,
			Sections:   []string{dashboardSectionJobs, dashboardSectionMetrics},
			JobID:      jobID,
			TargetIDs:  append([]int64(nil), queuedIDs...),
			ReasonCode: webhookReasonQueued,
		})
	}

	if err := a.recordDiunEvent(r.Context(), imageRepo, tag, digest, matchedIDs, queuedIDs, now); err != nil {
		log.Printf("record diun event failed: %v", err)
	}
	a.emitDashboardEvent(DashboardPatchEvent{
		EventType:  eventTypeDiunEvent,
		Sections:   []string{dashboardSectionEvents},
		TargetIDs:  append([]int64(nil), matchedIDs...),
		ReasonCode: reasonCode,
	})
	recordReceipt(true, http.StatusOK, reasonCode, "", jobID)

	writeJSON(w, http.StatusOK, diunWebhookResponse{
		ImageRepo:    imageRepo,
		MatchedCount: len(matchedIDs),
		QueuedCount:  len(queuedIDs),
		JobID:        jobID,
		SkippedIDs:   skippedIDs,
		WindowOpen:   windowOpen,
	})
}

func isDiunTestPayload(payload map[string]any) bool {
	status := strings.TrimSpace(strings.ToLower(getPathString(payload, "status")))
	if status == "test" {
		return true
	}
	kind := strings.TrimSpace(strings.ToLower(getPathString(payload, "type")))
	return kind == "test"
}

func (a *App) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case jobID := <-a.queue:
			a.runJob(ctx, jobID)
		}
	}
}

func (a *App) runJob(ctx context.Context, jobID int64) {
	startedAt := time.Now().Unix()
	if _, err := a.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, started_at = ?, error_message = NULL WHERE id = ?`,
		statusRunning, startedAt, jobID,
	); err != nil {
		log.Printf("set running failed for job %d: %v", jobID, err)
		return
	}
	a.emitDashboardEvent(DashboardPatchEvent{
		EventType: eventTypeJobRunning,
		Sections:  []string{dashboardSectionJobs, dashboardSectionMetrics},
		JobID:     &jobID,
	})
	a.logJob(jobID, "job started")

	jobType, err := a.jobType(ctx, jobID)
	if err != nil {
		_ = a.finishJob(ctx, jobID, statusFailed, fmt.Sprintf("read job type: %v", err))
		return
	}

	switch jobType {
	case jobTypeUpdate:
		blocked, runErr := a.runUpdateJob(ctx, jobID)
		if runErr != nil {
			if blocked {
				_ = a.finishJob(ctx, jobID, statusBlocked, runErr.Error())
			} else {
				_ = a.finishJob(ctx, jobID, statusFailed, runErr.Error())
			}
			return
		}
		if err := a.finishJob(ctx, jobID, statusSuccess, ""); err != nil {
			log.Printf("finish job %d failed: %v", jobID, err)
		}
	case jobTypePrune:
		if err := a.runPruneJob(ctx, jobID); err != nil {
			_ = a.finishJob(ctx, jobID, statusFailed, err.Error())
			return
		}
		if err := a.finishJob(ctx, jobID, statusSuccess, ""); err != nil {
			log.Printf("finish job %d failed: %v", jobID, err)
		}
	default:
		_ = a.finishJob(ctx, jobID, statusFailed, "unknown job type")
	}
}

func (a *App) runUpdateJob(ctx context.Context, jobID int64) (bool, error) {
	targetIDs, err := a.jobTargetIDs(ctx, jobID)
	if err != nil {
		return false, err
	}
	if len(targetIDs) == 0 {
		return false, errors.New("no targets in job")
	}

	for idx, targetID := range targetIDs {
		target, err := a.getTarget(ctx, targetID)
		if err != nil {
			remaining := len(targetIDs) - idx - 1
			if remaining > 0 {
				return true, fmt.Errorf("target %d lookup failed: %w", targetID, err)
			}
			return false, fmt.Errorf("target %d lookup failed: %w", targetID, err)
		}
		if !target.Enabled {
			a.logJob(jobID, fmt.Sprintf("target %s disabled, skipped", target.Name))
			continue
		}

		a.logJob(jobID, fmt.Sprintf("updating %s (%s)", target.Name, target.ComposeDir))

		if err := a.runComposeCommand(ctx, jobID, target, "pull"); err != nil {
			remaining := len(targetIDs) - idx - 1
			if remaining > 0 {
				a.logJob(jobID, fmt.Sprintf("failed on %s, blocking %d remaining target(s)", target.Name, remaining))
				return true, fmt.Errorf("%s pull failed: %w", target.Name, err)
			}
			return false, fmt.Errorf("%s pull failed: %w", target.Name, err)
		}
		if err := a.runComposeCommand(ctx, jobID, target, "up"); err != nil {
			remaining := len(targetIDs) - idx - 1
			if remaining > 0 {
				a.logJob(jobID, fmt.Sprintf("failed on %s, blocking %d remaining target(s)", target.Name, remaining))
				return true, fmt.Errorf("%s up failed: %w", target.Name, err)
			}
			return false, fmt.Errorf("%s up failed: %w", target.Name, err)
		}

		now := time.Now().Unix()
		if _, err := a.db.ExecContext(ctx,
			`UPDATE targets SET updated_at = ?, last_success_at = ? WHERE id = ?`,
			now, now, target.ID,
		); err != nil {
			a.logJob(jobID, fmt.Sprintf("warning: update success timestamp failed for %s: %v", target.Name, err))
		}
		a.logJob(jobID, fmt.Sprintf("target %s updated successfully", target.Name))
	}
	return false, nil
}

func (a *App) runPruneJob(ctx context.Context, jobID int64) error {
	a.logJob(jobID, "running docker image prune -f")
	return a.runAndStream(ctx, jobID, "", "docker", "image", "prune", "-f")
}

func (a *App) runComposeCommand(ctx context.Context, jobID int64, target Target, action string) error {
	var args []string
	switch action {
	case "pull":
		args = []string{"compose", "-f", target.ComposeFile, "pull"}
	case "up":
		args = []string{"compose", "-f", target.ComposeFile, "up", "-d"}
	default:
		return errors.New("unsupported compose action")
	}

	a.logJob(jobID, "docker "+strings.Join(args, " "))
	if action != "pull" || a.cfg.PullRetryMaxAttempts <= 1 {
		return a.runAndStream(ctx, jobID, target.ComposeDir, "docker", args...)
	}

	attempts := a.cfg.PullRetryMaxAttempts
	delayStep := time.Duration(a.cfg.PullRetryDelaySec) * time.Second
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			delay := computePullRetryDelay(lastErr, delayStep, attempt)
			if delay > 0 {
				a.logJob(jobID, fmt.Sprintf("retrying pull attempt %d/%d after %s", attempt, attempts, delay))
				if err := sleepWithContext(ctx, delay); err != nil {
					return err
				}
			} else {
				a.logJob(jobID, fmt.Sprintf("retrying pull attempt %d/%d", attempt, attempts))
			}
		}

		err := a.runAndStream(ctx, jobID, target.ComposeDir, "docker", args...)
		if err == nil {
			return nil
		}
		lastErr = err
		a.logJob(jobID, fmt.Sprintf("pull attempt %d/%d failed: %v", attempt, attempts, err))
	}
	return lastErr
}

func (a *App) runAndStream(ctx context.Context, jobID int64, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var linesMu sync.Mutex
	lines := make([]string, 0, 32)
	appendLine := func(line string) {
		linesMu.Lock()
		defer linesMu.Unlock()
		lines = append(lines, line)
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanPipe(stdout, func(line string) {
			if strings.TrimSpace(line) == "" {
				return
			}
			a.logJob(jobID, line)
			appendLine(line)
		})
	}()
	go func() {
		defer wg.Done()
		scanPipe(stderr, func(line string) {
			if strings.TrimSpace(line) == "" {
				return
			}
			a.logJob(jobID, line)
			appendLine(line)
		})
	}()

	waitErr := cmd.Wait()
	wg.Wait()
	if waitErr != nil {
		return &commandRunError{err: waitErr, outputLines: lines}
	}
	return nil
}

type commandRunError struct {
	err         error
	outputLines []string
}

func (e *commandRunError) Error() string {
	if e == nil || e.err == nil {
		return "command failed"
	}
	return e.err.Error()
}

func (e *commandRunError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func computePullRetryDelay(lastErr error, baseStep time.Duration, attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}
	baseDelay := baseStep * time.Duration(attempt-1)
	if isTooManyRequestsError(lastErr) {
		if parsed, ok := parseRetryAfterDelay(lastErr); ok {
			return clampDuration(parsed, 2*time.Second, 120*time.Second)
		}
		if baseDelay <= 0 {
			baseDelay = 2 * time.Second
		}
		return clampDuration(baseDelay, 2*time.Second, 120*time.Second)
	}
	return baseDelay
}

func isTooManyRequestsError(err error) bool {
	if err == nil {
		return false
	}
	for _, part := range errorTextParts(err) {
		if strings.Contains(strings.ToLower(part), "toomanyrequests") {
			return true
		}
	}
	return false
}

func parseRetryAfterDelay(err error) (time.Duration, bool) {
	for _, part := range errorTextParts(err) {
		matches := retryAfterPattern.FindStringSubmatch(part)
		if len(matches) != 2 {
			continue
		}
		token := strings.TrimSpace(matches[1])
		if token == "" {
			continue
		}
		if d, ok := parseRetryAfterToken(token); ok {
			return d, true
		}
	}
	return 0, false
}

func parseRetryAfterToken(token string) (time.Duration, bool) {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, `"'`)
	if token == "" {
		return 0, false
	}
	if d, err := time.ParseDuration(token); err == nil {
		return d, true
	}
	if secs, err := strconv.ParseFloat(token, 64); err == nil {
		return time.Duration(secs * float64(time.Second)), true
	}
	return 0, false
}

func errorTextParts(err error) []string {
	if err == nil {
		return nil
	}
	parts := []string{err.Error()}
	var cmdErr *commandRunError
	if errors.As(err, &cmdErr) {
		parts = append(parts, cmdErr.outputLines...)
	}
	return parts
}

func clampDuration(value, minValue, maxValue time.Duration) time.Duration {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func scanPipe(r io.Reader, onLine func(string)) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 1024*64)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		onLine(scanner.Text())
	}
}

func (a *App) listTargets(ctx context.Context) ([]Target, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at, last_success_at
		FROM targets
		ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []Target
	targetIDs := make([]int64, 0, 16)
	for rows.Next() {
		var t Target
		var autoUpdate, enabled int
		var lastSuccess sql.NullInt64
		if err := rows.Scan(
			&t.ID, &t.Name, &t.ComposeDir, &t.ComposeFile, &t.ImageRepo,
			&autoUpdate, &enabled, &t.CooldownSeconds, &t.CreatedAt, &t.UpdatedAt, &lastSuccess,
		); err != nil {
			return nil, err
		}
		t.AutoUpdateEnabled = autoUpdate == 1
		t.Enabled = enabled == 1
		if lastSuccess.Valid {
			v := lastSuccess.Int64
			t.LastSuccessAt = &v
		}
		targets = append(targets, t)
		targetIDs = append(targetIDs, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	repoMap, err := a.loadTargetImageRepoMap(ctx, targetIDs)
	if err != nil {
		return nil, err
	}
	for i := range targets {
		repos := repoMap[targets[i].ID]
		if len(repos) == 0 && targets[i].ImageRepo != "" {
			repos = []string{targets[i].ImageRepo}
		}
		targets[i].ImageRepos = repos
	}
	return targets, nil
}

func (a *App) discoverContainers() ([]DiscoveredContainer, error) {
	entries, err := os.ReadDir(a.cfg.ContainerRoot)
	if err != nil {
		return nil, err
	}

	composeCandidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}
	items := []DiscoveredContainer{}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}

		composeDir := filepath.Join(a.cfg.ContainerRoot, name)
		composeFile := ""
		for _, candidate := range composeCandidates {
			fullPath := filepath.Join(composeDir, candidate)
			st, err := os.Stat(fullPath)
			if err == nil && !st.IsDir() {
				composeFile = candidate
				break
			}
		}
		if composeFile == "" {
			continue
		}

		repos, err := parseComposeImageRepos(filepath.Join(composeDir, composeFile))
		if err != nil {
			return nil, err
		}

		items = append(items, DiscoveredContainer{
			Name:                name,
			ComposeDir:          composeDir,
			ComposeFile:         composeFile,
			ImageRepoCandidates: repos,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func parseComposeImageRepos(composePath string) ([]string, error) {
	vars, err := buildComposeVariables(composePath)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(composePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	unique := map[string]struct{}{}
	repos := []string{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		value, ok := parseComposeImageValue(line)
		if !ok {
			continue
		}
		resolvedValue, ok := resolveComposeVariables(value, vars)
		if !ok {
			continue
		}
		repo := normalizeImageRepo(resolvedValue)
		if repo == "" {
			continue
		}
		if _, exists := unique[repo]; exists {
			continue
		}
		unique[repo] = struct{}{}
		repos = append(repos, repo)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Strings(repos)
	return repos, nil
}

var composeVarExprPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(?:(:?[-+?])(.*))?$`)
var composeInterpolationPattern = regexp.MustCompile(`\$\{[^}]+\}`)
var retryAfterPattern = regexp.MustCompile(`(?i)retry-after:\s*([^,\s]+)`)

const composeEscapedDollarPlaceholder = "\x00_composepulse_dollar_\x00"
const maxComposeResolveDepth = 10

func parseComposeImageValue(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "image:") {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, "image:"))
	if value == "" {
		return "", false
	}
	value = trimUnquotedInlineComment(value)
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) && len(value) >= 2 {
		value = value[1 : len(value)-1]
	}
	if strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`) && len(value) >= 2 {
		value = value[1 : len(value)-1]
	}
	return strings.TrimSpace(value), true
}

func buildComposeVariables(composePath string) (map[string]string, error) {
	values := map[string]string{}

	envPath := filepath.Join(filepath.Dir(composePath), ".env")
	fileValues, err := parseDotEnvFile(envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for key, value := range fileValues {
		values[key] = value
	}

	for _, raw := range os.Environ() {
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	return values, nil
}

func parseDotEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !isValidEnvKey(key) {
			continue
		}
		value = strings.TrimSpace(value)
		value = trimUnquotedInlineComment(value)
		if len(value) >= 2 {
			if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
				(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
				value = value[1 : len(value)-1]
			}
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, ch := range key {
		if i == 0 {
			if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') && ch != '_' {
				return false
			}
			continue
		}
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '_' {
			return false
		}
	}
	return true
}

func trimUnquotedInlineComment(value string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if inSingle || inDouble {
				continue
			}
			if i == 0 || value[i-1] == ' ' || value[i-1] == '\t' {
				return strings.TrimSpace(value[:i])
			}
		}
	}
	return strings.TrimSpace(value)
}

func resolveComposeVariables(value string, vars map[string]string) (string, bool) {
	return resolveComposeVariablesDepth(value, vars, 0)
}

func resolveComposeVariablesDepth(value string, vars map[string]string, depth int) (string, bool) {
	if depth > maxComposeResolveDepth {
		return "", false
	}

	processed := strings.ReplaceAll(value, "$$", composeEscapedDollarPlaceholder)
	unresolved := false
	processed = composeInterpolationPattern.ReplaceAllStringFunc(processed, func(match string) string {
		expr := strings.TrimSpace(match[2 : len(match)-1])
		resolved, ok := resolveComposeExpression(expr, vars, depth+1)
		if !ok {
			unresolved = true
			return ""
		}
		return resolved
	})
	if unresolved {
		return "", false
	}
	processed = strings.ReplaceAll(processed, composeEscapedDollarPlaceholder, "$")
	return strings.TrimSpace(processed), true
}

func resolveComposeExpression(expr string, vars map[string]string, depth int) (string, bool) {
	if depth > maxComposeResolveDepth {
		return "", false
	}

	matches := composeVarExprPattern.FindStringSubmatch(expr)
	if len(matches) != 4 {
		return "", false
	}
	name := matches[1]
	op := matches[2]
	arg := matches[3]

	value, isSet := vars[name]
	switch op {
	case "":
		if !isSet {
			return "", true
		}
		return value, true
	case "-":
		if isSet {
			return value, true
		}
		return resolveComposeOperatorArg(arg, vars, depth+1)
	case ":-":
		if isSet && value != "" {
			return value, true
		}
		return resolveComposeOperatorArg(arg, vars, depth+1)
	case "+":
		if !isSet {
			return "", true
		}
		return resolveComposeOperatorArg(arg, vars, depth+1)
	case ":+":
		if !isSet || value == "" {
			return "", true
		}
		return resolveComposeOperatorArg(arg, vars, depth+1)
	case "?":
		if !isSet {
			return "", false
		}
		return value, true
	case ":?":
		if !isSet || value == "" {
			return "", false
		}
		return value, true
	default:
		return "", false
	}
}

func resolveComposeOperatorArg(arg string, vars map[string]string, depth int) (string, bool) {
	if strings.Contains(arg, "${") {
		return resolveComposeVariablesDepth(arg, vars, depth+1)
	}
	return arg, true
}

var errConflict = errors.New("conflict")
var errTargetBusy = errors.New("target is used by queued or running jobs")
var errJobBusy = errors.New("job is queued or running")

func (a *App) createTarget(ctx context.Context, req createTargetRequest) (Target, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Target{}, errors.New("name is required")
	}

	composeDir, err := validateComposeDir(a.cfg.ContainerRoot, req.ComposeDir)
	if err != nil {
		return Target{}, err
	}

	composeFile := strings.TrimSpace(req.ComposeFile)
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}
	if err := validateComposeFile(composeFile); err != nil {
		return Target{}, err
	}
	if st, err := os.Stat(composeDir); err != nil || !st.IsDir() {
		return Target{}, errors.New("compose_dir does not exist or is not a directory")
	}
	composePath := filepath.Join(composeDir, composeFile)
	if st, err := os.Stat(composePath); err != nil || st.IsDir() {
		return Target{}, errors.New("compose_file not found in compose_dir")
	}

	imageRepo := normalizeImageRepo(req.ImageRepo)
	imageRepos := normalizeImageRepos(req.ImageRepos)
	if imageRepo == "" && len(imageRepos) > 0 {
		imageRepo = imageRepos[0]
	}
	if imageRepo == "" {
		return Target{}, errors.New("image_repo is required")
	}
	if !containsString(imageRepos, imageRepo) {
		imageRepos = append(imageRepos, imageRepo)
		sort.Strings(imageRepos)
	}
	if len(imageRepos) == 0 {
		imageRepos = []string{imageRepo}
	}

	cooldown := a.cfg.DefaultCooldown
	if req.CooldownSeconds != nil {
		cooldown = *req.CooldownSeconds
	}
	if cooldown <= 0 {
		return Target{}, errors.New("cooldown_seconds must be > 0")
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return Target{}, err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO targets (name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 1, ?, ?, ?)`,
		name, composeDir, composeFile, imageRepo, cooldown, now, now,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Target{}, fmt.Errorf("%w: name/compose_dir already exists", errConflict)
		}
		return Target{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Target{}, err
	}
	if err := insertTargetImageRepos(ctx, tx, id, imageRepos, now); err != nil {
		return Target{}, err
	}
	if err := tx.Commit(); err != nil {
		return Target{}, err
	}
	return a.getTarget(ctx, id)
}

func (a *App) patchTarget(ctx context.Context, id int64, req patchTargetRequest) (Target, error) {
	target, err := a.getTarget(ctx, id)
	if err != nil {
		return Target{}, err
	}

	changed := false
	if req.AutoUpdateEnabled != nil {
		target.AutoUpdateEnabled = *req.AutoUpdateEnabled
		changed = true
	}
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
		changed = true
	}
	if req.CooldownSeconds != nil {
		if *req.CooldownSeconds <= 0 {
			return Target{}, errors.New("cooldown_seconds must be > 0")
		}
		target.CooldownSeconds = *req.CooldownSeconds
		changed = true
	}
	if !changed {
		return target, nil
	}

	now := time.Now().Unix()
	_, err = a.db.ExecContext(ctx, `
		UPDATE targets
		SET auto_update_enabled = ?, enabled = ?, cooldown_seconds = ?, updated_at = ?
		WHERE id = ?`,
		boolToInt(target.AutoUpdateEnabled), boolToInt(target.Enabled), target.CooldownSeconds, now, id,
	)
	if err != nil {
		return Target{}, err
	}
	return a.getTarget(ctx, id)
}

func (a *App) deleteTarget(ctx context.Context, id int64) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM targets WHERE id = ?`, id).Scan(&exists); err != nil {
		return err
	}

	var activeRef int
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM jobs j
		INNER JOIN job_targets jt ON jt.job_id = j.id
		WHERE jt.target_id = ? AND j.status IN (?, ?)
		LIMIT 1`,
		id, statusQueued, statusRunning,
	).Scan(&activeRef)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && activeRef == 1 {
		return errTargetBusy
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM targets WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) deleteJob(ctx context.Context, id int64) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id = ?`, id).Scan(&status); err != nil {
		return err
	}
	if status == statusQueued || status == statusRunning {
		return errJobBusy
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (a *App) deleteAllJobs(ctx context.Context) (int64, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var activeCount int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM jobs WHERE status IN (?, ?)`,
		statusQueued, statusRunning,
	).Scan(&activeCount); err != nil {
		return 0, err
	}
	if activeCount > 0 {
		return 0, errJobBusy
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM jobs`)
	if err != nil {
		return 0, err
	}
	deletedCount, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return deletedCount, nil
}

func (a *App) getTarget(ctx context.Context, id int64) (Target, error) {
	var t Target
	var autoUpdate, enabled int
	var lastSuccess sql.NullInt64
	err := a.db.QueryRowContext(ctx, `
		SELECT id, name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at, last_success_at
		FROM targets
		WHERE id = ?`,
		id,
	).Scan(
		&t.ID, &t.Name, &t.ComposeDir, &t.ComposeFile, &t.ImageRepo,
		&autoUpdate, &enabled, &t.CooldownSeconds, &t.CreatedAt, &t.UpdatedAt, &lastSuccess,
	)
	if err != nil {
		return Target{}, err
	}
	t.AutoUpdateEnabled = autoUpdate == 1
	t.Enabled = enabled == 1
	if lastSuccess.Valid {
		v := lastSuccess.Int64
		t.LastSuccessAt = &v
	}
	repos, err := a.loadTargetImageRepos(ctx, t.ID)
	if err != nil {
		return Target{}, err
	}
	if len(repos) == 0 && t.ImageRepo != "" {
		repos = []string{t.ImageRepo}
	}
	t.ImageRepos = repos
	return t, nil
}

func (a *App) filterEnabledTargets(ctx context.Context, requested []int64) ([]int64, error) {
	seen := map[int64]struct{}{}
	filtered := make([]int64, 0, len(requested))

	for _, id := range requested {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		var exists int
		var enabled int
		err := a.db.QueryRowContext(ctx,
			`SELECT 1, enabled FROM targets WHERE id = ?`,
			id,
		).Scan(&exists, &enabled)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("target id %d not found", id)
			}
			return nil, err
		}
		if enabled == 1 {
			filtered = append(filtered, id)
		}
	}
	return filtered, nil
}

func (a *App) createJob(ctx context.Context, jobType, trigger string, targetIDs []int64) (int64, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO jobs (type, trigger, status, created_at) VALUES (?, ?, ?, ?)`,
		jobType, trigger, statusQueued, now,
	)
	if err != nil {
		return 0, err
	}
	jobID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for i, targetID := range targetIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO job_targets (job_id, target_id, position) VALUES (?, ?, ?)`,
			jobID, targetID, i,
		); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return jobID, nil
}

func (a *App) enqueueJob(jobID int64) error {
	select {
	case a.queue <- jobID:
		return nil
	default:
		return errors.New("job queue is full")
	}
}

func (a *App) listJobs(ctx context.Context, limit int, cursor *int64) ([]Job, *int64, error) {
	fetchLimit := limit + 1

	query := `
		SELECT id, type, trigger, status, error_message, created_at, started_at, ended_at
		FROM jobs`
	args := []any{}
	if cursor != nil {
		query += ` WHERE id < ?`
		args = append(args, *cursor)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, fetchLimit)

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	jobs := make([]Job, 0, fetchLimit)
	for rows.Next() {
		var j Job
		var errMsg sql.NullString
		var startedAt sql.NullInt64
		var endedAt sql.NullInt64

		if err := rows.Scan(
			&j.ID, &j.Type, &j.Trigger, &j.Status, &errMsg, &j.CreatedAt, &startedAt, &endedAt,
		); err != nil {
			return nil, nil, err
		}
		if errMsg.Valid {
			msg := errMsg.String
			j.ErrorMessage = &msg
		}
		if startedAt.Valid {
			v := startedAt.Int64
			j.StartedAt = &v
		}
		if endedAt.Valid {
			v := endedAt.Int64
			j.EndedAt = &v
		}
		if startedAt.Valid && endedAt.Valid && endedAt.Int64 >= startedAt.Int64 {
			j.DurationSec = float64(endedAt.Int64 - startedAt.Int64)
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var nextCursor *int64
	if len(jobs) > limit {
		last := jobs[limit-1].ID
		nextCursor = &last
		jobs = jobs[:limit]
	}

	// sqlite is configured with a single open connection; resolve target ids
	// after closing the main rows cursor to avoid nested query deadlocks.
	for i := range jobs {
		targetIDs, err := a.jobTargetIDs(ctx, jobs[i].ID)
		if err != nil {
			return nil, nil, err
		}
		jobs[i].TargetIDs = targetIDs
	}
	return jobs, nextCursor, nil
}

func (a *App) listTargetAuditSummaries(ctx context.Context) ([]TargetAuditSummary, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT
			t.id,
			t.name,
			t.image_repo,
			t.last_success_at,
			COALESCE(SUM(CASE WHEN j.type = ? THEN 1 ELSE 0 END), 0) AS total_runs,
			COALESCE(SUM(CASE WHEN j.type = ? AND j.status = ? THEN 1 ELSE 0 END), 0) AS success_runs,
			COALESCE(SUM(CASE WHEN j.type = ? AND j.status = ? THEN 1 ELSE 0 END), 0) AS failed_runs,
			COALESCE(SUM(CASE WHEN j.type = ? AND j.status = ? THEN 1 ELSE 0 END), 0) AS blocked_runs,
			MAX(CASE WHEN j.type = ? THEN j.created_at ELSE NULL END) AS last_run_at,
			AVG(CASE WHEN j.type = ? AND j.started_at IS NOT NULL AND j.ended_at IS NOT NULL AND j.ended_at >= j.started_at THEN (j.ended_at - j.started_at) ELSE NULL END) AS avg_duration_sec
		FROM targets t
		LEFT JOIN job_targets jt ON jt.target_id = t.id
		LEFT JOIN jobs j ON j.id = jt.job_id
		GROUP BY t.id, t.name, t.image_repo, t.last_success_at
		ORDER BY t.id ASC`,
		jobTypeUpdate,
		jobTypeUpdate, statusSuccess,
		jobTypeUpdate, statusFailed,
		jobTypeUpdate, statusBlocked,
		jobTypeUpdate,
		jobTypeUpdate,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []TargetAuditSummary{}
	for rows.Next() {
		var item TargetAuditSummary
		var lastSuccess sql.NullInt64
		var lastRun sql.NullInt64
		var avg sql.NullFloat64
		if err := rows.Scan(
			&item.TargetID,
			&item.Name,
			&item.ImageRepo,
			&lastSuccess,
			&item.TotalRuns,
			&item.SuccessRuns,
			&item.FailedRuns,
			&item.BlockedRuns,
			&lastRun,
			&avg,
		); err != nil {
			return nil, err
		}
		if lastSuccess.Valid {
			v := lastSuccess.Int64
			item.LastSuccessAt = &v
		}
		if lastRun.Valid {
			v := lastRun.Int64
			item.LastRunAt = &v
		}
		if avg.Valid {
			item.AvgDurationSec = math.Round(avg.Float64*100) / 100
		}
		summaries = append(summaries, item)
	}
	return summaries, rows.Err()
}

func (a *App) getJobLogs(ctx context.Context, jobID int64) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT line
		FROM job_logs
		WHERE job_id = ?
		ORDER BY id ASC`,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, rows.Err()
}

func (a *App) jobTargetIDs(ctx context.Context, jobID int64) ([]int64, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT target_id
		FROM job_targets
		WHERE job_id = ?
		ORDER BY position ASC`,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (a *App) jobType(ctx context.Context, jobID int64) (string, error) {
	var t string
	err := a.db.QueryRowContext(ctx, `SELECT type FROM jobs WHERE id = ?`, jobID).Scan(&t)
	return t, err
}

func (a *App) jobExists(ctx context.Context, jobID int64) (bool, error) {
	var exists int
	err := a.db.QueryRowContext(ctx, `SELECT 1 FROM jobs WHERE id = ?`, jobID).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists == 1, nil
}

func (a *App) finishJob(ctx context.Context, jobID int64, status, message string) error {
	now := time.Now().Unix()
	var errMsg any = nil
	if strings.TrimSpace(message) != "" {
		errMsg = message
		a.logJob(jobID, "error: "+message)
	}
	if _, err := a.db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, error_message = ?, ended_at = ? WHERE id = ?`,
		status, errMsg, now, jobID,
	); err != nil {
		return err
	}

	eventType := eventTypeJobFailed
	switch status {
	case statusSuccess:
		eventType = eventTypeJobSuccess
	case statusBlocked:
		eventType = eventTypeJobBlocked
	case statusFailed:
		eventType = eventTypeJobFailed
	}
	a.emitDashboardEvent(DashboardPatchEvent{
		EventType: eventType,
		Sections:  []string{dashboardSectionJobs, dashboardSectionMetrics, dashboardSectionAudit, dashboardSectionTargets},
		JobID:     &jobID,
	})

	a.logJob(jobID, "job finished with status="+status)
	return nil
}

func (a *App) failJob(ctx context.Context, jobID int64, message string) error {
	return a.finishJob(ctx, jobID, statusFailed, message)
}

func (a *App) logJob(jobID int64, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	now := time.Now().Unix()
	fullLine := fmt.Sprintf("[%s] %s", time.Unix(now, 0).Format(time.RFC3339), line)

	a.logMu.Lock()
	_, err := a.db.ExecContext(context.Background(),
		`INSERT INTO job_logs (job_id, created_at, line) VALUES (?, ?, ?)`,
		jobID, now, fullLine,
	)
	a.logMu.Unlock()
	if err != nil {
		log.Printf("insert log failed for job %d: %v", jobID, err)
	}

	a.logBroker.Publish(jobID, fullLine)
	log.Printf("job[%d] %s", jobID, line)
}

func (a *App) jobDone(ctx context.Context, jobID int64) (bool, string) {
	var status string
	err := a.db.QueryRowContext(ctx, `SELECT status FROM jobs WHERE id = ?`, jobID).Scan(&status)
	if err != nil {
		return true, statusFailed
	}
	switch status {
	case statusSuccess, statusFailed, statusBlocked:
		return true, status
	default:
		return false, status
	}
}

func (a *App) recoverInterruptedJobs() error {
	now := time.Now().Unix()
	_, err := a.db.Exec(`
		UPDATE jobs
		SET status = ?, error_message = ?, ended_at = ?
		WHERE status IN (?, ?)`,
		statusFailed, "interrupted by process restart", now, statusQueued, statusRunning,
	)
	return err
}

func (a *App) findTargetsByImageRepo(ctx context.Context, imageRepo string) ([]Target, error) {
	normRepo := normalizeImageRepo(imageRepo)
	if normRepo == "" {
		return []Target{}, nil
	}

	rows, err := a.db.QueryContext(ctx, `
			SELECT id, name, compose_dir, compose_file, image_repo, auto_update_enabled, enabled, cooldown_seconds, created_at, updated_at, last_success_at
			FROM targets
			WHERE (
					EXISTS (
						SELECT 1
						FROM target_image_repos tir
						WHERE tir.target_id = targets.id AND tir.image_repo = ?
				)
				OR (
					targets.image_repo = ?
					AND NOT EXISTS (
						SELECT 1 FROM target_image_repos tir2 WHERE tir2.target_id = targets.id
					)
				)
			)
		ORDER BY id ASC`,
		normRepo,
		normRepo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []Target
	targetIDs := make([]int64, 0, 16)
	for rows.Next() {
		var t Target
		var autoUpdate int
		var enabled int
		var lastSuccess sql.NullInt64
		if err := rows.Scan(
			&t.ID, &t.Name, &t.ComposeDir, &t.ComposeFile, &t.ImageRepo,
			&autoUpdate, &enabled, &t.CooldownSeconds, &t.CreatedAt, &t.UpdatedAt, &lastSuccess,
		); err != nil {
			return nil, err
		}
		t.AutoUpdateEnabled = autoUpdate == 1
		t.Enabled = enabled == 1
		if lastSuccess.Valid {
			v := lastSuccess.Int64
			t.LastSuccessAt = &v
		}
		targets = append(targets, t)
		targetIDs = append(targetIDs, t.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	repoMap, err := a.loadTargetImageRepoMap(ctx, targetIDs)
	if err != nil {
		return nil, err
	}
	for i := range targets {
		repos := repoMap[targets[i].ID]
		if len(repos) == 0 && targets[i].ImageRepo != "" {
			repos = []string{targets[i].ImageRepo}
		}
		targets[i].ImageRepos = repos
	}
	return targets, nil
}

func normalizeImageRepos(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		repo := normalizeImageRepo(raw)
		if repo == "" {
			continue
		}
		if _, exists := unique[repo]; exists {
			continue
		}
		unique[repo] = struct{}{}
		out = append(out, repo)
	}
	sort.Strings(out)
	return out
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func insertTargetImageRepos(ctx context.Context, tx *sql.Tx, targetID int64, repos []string, createdAt int64) error {
	if len(repos) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO target_image_repos (target_id, image_repo, created_at)
		VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, repo := range repos {
		if _, err := stmt.ExecContext(ctx, targetID, repo, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) loadTargetImageRepos(ctx context.Context, targetID int64) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT image_repo
		FROM target_image_repos
		WHERE target_id = ?
		ORDER BY image_repo ASC`,
		targetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	repos := []string{}
	for rows.Next() {
		var repo string
		if err := rows.Scan(&repo); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}
	return repos, rows.Err()
}

func (a *App) loadTargetImageRepoMap(ctx context.Context, targetIDs []int64) (map[int64][]string, error) {
	result := make(map[int64][]string, len(targetIDs))
	if len(targetIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, 0, len(targetIDs))
	args := make([]any, 0, len(targetIDs))
	for _, id := range targetIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	query := fmt.Sprintf(`
		SELECT target_id, image_repo
		FROM target_image_repos
		WHERE target_id IN (%s)
		ORDER BY target_id ASC, image_repo ASC`,
		strings.Join(placeholders, ","),
	)

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var targetID int64
		var repo string
		if err := rows.Scan(&targetID, &repo); err != nil {
			return nil, err
		}
		result[targetID] = append(result[targetID], repo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) targetInCooldown(ctx context.Context, targetID int64, cooldownSec int) (bool, error) {
	threshold := time.Now().Unix() - int64(cooldownSec)
	var exists int
	err := a.db.QueryRowContext(ctx, `
		SELECT 1
		FROM jobs j
		INNER JOIN job_targets jt ON jt.job_id = j.id
		WHERE jt.target_id = ?
			AND j.trigger = ?
			AND j.status IN (?, ?, ?)
			AND j.created_at >= ?
		LIMIT 1`,
		targetID, triggerAuto, statusQueued, statusRunning, statusSuccess, threshold,
	).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists == 1, nil
}

func (a *App) recordDiunEvent(ctx context.Context, imageRepo, tag, digest string, matchedIDs, queuedIDs []int64, ts int64) error {
	matchedJSON, _ := json.Marshal(matchedIDs)
	queuedJSON, _ := json.Marshal(queuedIDs)
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO diun_events (image_repo, tag, digest, matched_target_ids, queued_target_ids, received_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		imageRepo, tag, digest, string(matchedJSON), string(queuedJSON), ts,
	)
	return err
}

func (a *App) listDiunEvents(ctx context.Context, limit int) ([]DiunEvent, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, image_repo, tag, digest, matched_target_ids, queued_target_ids, received_at
		FROM diun_events
		ORDER BY id DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]DiunEvent, 0, limit)
	for rows.Next() {
		var e DiunEvent
		var matchedRaw string
		var queuedRaw string
		if err := rows.Scan(&e.ID, &e.ImageRepo, &e.Tag, &e.Digest, &matchedRaw, &queuedRaw, &e.ReceivedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(matchedRaw), &e.MatchedTargetIDs)
		_ = json.Unmarshal([]byte(queuedRaw), &e.QueuedTargetIDs)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (a *App) listWebhookReceipts(ctx context.Context, limit int) ([]WebhookReceiptSummary, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, status_code, reason_code, received_at, queued_job_id
		FROM webhook_receipts
		ORDER BY id DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]WebhookReceiptSummary, 0, limit)
	for rows.Next() {
		var item WebhookReceiptSummary
		var queued sql.NullInt64
		if err := rows.Scan(&item.ID, &item.StatusCode, &item.ReasonCode, &item.ReceivedAt, &queued); err != nil {
			return nil, err
		}
		if queued.Valid {
			v := queued.Int64
			item.QueuedJobID = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (a *App) recordWebhookReceipt(ctx context.Context, ts int64, success bool, statusCode int, reasonCode, message string, queuedJobID *int64) error {
	var queued any = nil
	if queuedJobID != nil && *queuedJobID > 0 {
		queued = *queuedJobID
	}
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO webhook_receipts (received_at, success, error, status_code, reason_code, queued_job_id)
		VALUES (?, ?, ?, ?, ?, ?)`,
		ts, boolToInt(success), message, statusCode, strings.TrimSpace(reasonCode), queued,
	)
	if err != nil {
		return err
	}
	a.emitDashboardEvent(DashboardPatchEvent{
		EventType:  eventTypeWebhookReceipt,
		Sections:   []string{dashboardSectionReceipts, dashboardSectionMetrics},
		JobID:      queuedJobID,
		ReasonCode: strings.TrimSpace(reasonCode),
		OccurredAt: ts,
	})
	return nil
}

func (a *App) recordAuthLoginEvent(ctx context.Context, ts int64, success bool, rateLimited bool, username, clientIP string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		username = "-"
	}
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		clientIP = "-"
	}
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO auth_login_events (created_at, success, rate_limited, username, client_ip)
		VALUES (?, ?, ?, ?, ?)`,
		ts, boolToInt(success), boolToInt(rateLimited), username, clientIP,
	)
	return err
}

type pushMessage struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	Tag       string `json:"tag"`
	EventType string `json:"event_type"`
}

func (a *App) emitDashboardEvent(evt DashboardPatchEvent) {
	if len(evt.Sections) == 0 {
		return
	}
	evt.Sections = uniqueStringSlice(evt.Sections)
	if evt.OccurredAt <= 0 {
		evt.OccurredAt = time.Now().Unix()
	}
	if a.dashboardBroker != nil {
		a.dashboardBroker.Publish(evt)
	}
	if a.shouldPushEvent(evt) {
		if !a.shouldEmitPushByDedupe(evt, 30*time.Second) {
			return
		}
		go func(copyEvt DashboardPatchEvent) {
			if err := a.sendPushForEvent(context.Background(), copyEvt); err != nil {
				log.Printf("send push for event failed: %v", err)
			}
		}(evt)
	}
}

func (a *App) shouldPushEvent(evt DashboardPatchEvent) bool {
	if !a.webPushConfigured() {
		return false
	}
	switch evt.EventType {
	case eventTypeJobQueued, eventTypeJobRunning, eventTypeJobSuccess, eventTypeJobFailed, eventTypeJobBlocked, eventTypeWebhookReceipt, eventTypeAuthRateLimited:
		return true
	default:
		return false
	}
}

func (a *App) shouldEmitPushByDedupe(evt DashboardPatchEvent, window time.Duration) bool {
	if window <= 0 {
		window = 30 * time.Second
	}
	key := pushDedupeKey(evt)
	if key == "" {
		return true
	}
	now := time.Now().Unix()
	minAge := int64(window.Seconds())
	if minAge <= 0 {
		minAge = 30
	}

	a.pushDedupeMu.Lock()
	defer a.pushDedupeMu.Unlock()
	if a.pushDedupe == nil {
		a.pushDedupe = map[string]int64{}
	}
	if last, ok := a.pushDedupe[key]; ok && now-last < minAge {
		return false
	}
	a.pushDedupe[key] = now

	// Keep dedupe map bounded by evicting stale keys.
	for k, ts := range a.pushDedupe {
		if now-ts > 6*60*60 {
			delete(a.pushDedupe, k)
		}
	}
	return true
}

func pushDedupeKey(evt DashboardPatchEvent) string {
	switch evt.EventType {
	case eventTypeJobQueued, eventTypeJobRunning, eventTypeJobSuccess, eventTypeJobFailed, eventTypeJobBlocked:
		if evt.JobID != nil && *evt.JobID > 0 {
			return fmt.Sprintf("job:%s:%d", evt.EventType, *evt.JobID)
		}
		return fmt.Sprintf("job:%s", evt.EventType)
	case eventTypeWebhookReceipt:
		return fmt.Sprintf("webhook:%s", strings.TrimSpace(evt.ReasonCode))
	case eventTypeAuthRateLimited:
		return eventTypeAuthRateLimited
	default:
		return ""
	}
}

func (a *App) webPushConfigured() bool {
	if !a.cfg.WebPushEnabled {
		return false
	}
	if strings.TrimSpace(a.cfg.WebPushVAPIDPublic) == "" {
		return false
	}
	if strings.TrimSpace(a.cfg.WebPushVAPIDPrivate) == "" {
		return false
	}
	if strings.TrimSpace(a.cfg.WebPushSubject) == "" {
		return false
	}
	return true
}

func (a *App) sendPushForEvent(ctx context.Context, evt DashboardPatchEvent) error {
	msg := pushMessage{
		URL:       "/",
		EventType: evt.EventType,
	}
	switch evt.EventType {
	case eventTypeJobQueued:
		msg.Title = "Update job queued"
		msg.Body = fmt.Sprintf("job #%d queued", derefInt64(evt.JobID))
		msg.Tag = "job-queued"
	case eventTypeJobRunning:
		msg.Title = "Update job running"
		msg.Body = fmt.Sprintf("job #%d started", derefInt64(evt.JobID))
		msg.Tag = "job-running"
	case eventTypeJobSuccess:
		msg.Title = "Update job succeeded"
		msg.Body = fmt.Sprintf("job #%d completed successfully", derefInt64(evt.JobID))
		msg.Tag = "job-success"
	case eventTypeJobFailed:
		msg.Title = "Update job failed"
		msg.Body = fmt.Sprintf("job #%d failed", derefInt64(evt.JobID))
		msg.Tag = "job-failed"
	case eventTypeJobBlocked:
		msg.Title = "Update job blocked"
		msg.Body = fmt.Sprintf("job #%d blocked remaining targets", derefInt64(evt.JobID))
		msg.Tag = "job-blocked"
	case eventTypeWebhookReceipt:
		msg.Title = "DIUN webhook received"
		reason := strings.TrimSpace(evt.ReasonCode)
		if reason == "" {
			reason = "unknown"
		}
		msg.Body = "reason=" + reason
		msg.Tag = "webhook-" + reason
	case eventTypeAuthRateLimited:
		msg.Title = "Login rate limited"
		msg.Body = "Too many login attempts detected."
		msg.Tag = "auth-rate-limited"
	default:
		return nil
	}
	_, _, err := a.sendPushNotification(ctx, msg)
	return err
}

func (a *App) sendPushNotification(ctx context.Context, msg pushMessage) (int, int, error) {
	if !a.webPushConfigured() {
		return 0, 0, nil
	}
	payloadRaw, err := json.Marshal(msg)
	if err != nil {
		return 0, 0, err
	}

	subs, err := a.listActivePushSubscriptions(ctx)
	if err != nil {
		return 0, 0, err
	}
	sent := 0
	failed := 0
	for _, sub := range subs {
		ws := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256DH,
				Auth:   sub.Auth,
			},
		}
		resp, sendErr := webpush.SendNotification(payloadRaw, ws, &webpush.Options{
			Subscriber:      a.cfg.WebPushSubject,
			VAPIDPublicKey:  a.cfg.WebPushVAPIDPublic,
			VAPIDPrivateKey: a.cfg.WebPushVAPIDPrivate,
			TTL:             60,
		})
		if sendErr != nil {
			failed++
			_ = a.recordPushDelivery(ctx, false, sendErr.Error(), time.Now().Unix())
			_ = a.markPushSubscriptionError(ctx, sub.ID, false)
			continue
		}

		bodyRaw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			sent++
			_ = a.recordPushDelivery(ctx, true, "", time.Now().Unix())
			_ = a.markPushSubscriptionSuccess(ctx, sub.ID)
			continue
		}

		errText := strings.TrimSpace(string(bodyRaw))
		if errText == "" {
			errText = fmt.Sprintf("web push status %d", resp.StatusCode)
		}
		disable := resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound
		_ = a.recordPushDelivery(ctx, false, errText, time.Now().Unix())
		_ = a.markPushSubscriptionError(ctx, sub.ID, disable)
		failed++
	}
	return sent, failed, nil
}

func (a *App) upsertPushSubscription(ctx context.Context, endpoint, p256dh, auth, ua string) (PushSubscription, error) {
	now := time.Now().Unix()
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (endpoint, p256dh, auth, ua, created_at, updated_at, disabled)
		VALUES (?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(endpoint) DO UPDATE SET
			p256dh = excluded.p256dh,
			auth = excluded.auth,
			ua = excluded.ua,
			updated_at = excluded.updated_at,
			disabled = 0`,
		endpoint, p256dh, auth, ua, now, now,
	)
	if err != nil {
		return PushSubscription{}, err
	}
	return a.getPushSubscriptionByEndpoint(ctx, endpoint)
}

func (a *App) getPushSubscriptionByEndpoint(ctx context.Context, endpoint string) (PushSubscription, error) {
	var item PushSubscription
	var disabled int
	var lastSuccess sql.NullInt64
	var lastError sql.NullInt64
	err := a.db.QueryRowContext(ctx, `
		SELECT id, endpoint, p256dh, auth, ua, created_at, updated_at, last_success_at, last_error_at, disabled
		FROM push_subscriptions
		WHERE endpoint = ?`,
		endpoint,
	).Scan(
		&item.ID, &item.Endpoint, &item.P256DH, &item.Auth, &item.UA, &item.CreatedAt, &item.UpdatedAt, &lastSuccess, &lastError, &disabled,
	)
	if err != nil {
		return PushSubscription{}, err
	}
	if lastSuccess.Valid {
		v := lastSuccess.Int64
		item.LastSuccessAt = &v
	}
	if lastError.Valid {
		v := lastError.Int64
		item.LastErrorAt = &v
	}
	item.Disabled = disabled == 1
	return item, nil
}

func (a *App) disablePushSubscriptionByEndpoint(ctx context.Context, endpoint string) (int64, error) {
	res, err := a.db.ExecContext(ctx, `
		UPDATE push_subscriptions
		SET disabled = 1, updated_at = ?
		WHERE endpoint = ?`,
		time.Now().Unix(), endpoint,
	)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (a *App) disableAllPushSubscriptions(ctx context.Context) (int64, error) {
	res, err := a.db.ExecContext(ctx, `
		UPDATE push_subscriptions
		SET disabled = 1, updated_at = ?
		WHERE disabled = 0`,
		time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (a *App) listActivePushSubscriptions(ctx context.Context) ([]PushSubscription, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, endpoint, p256dh, auth, ua, created_at, updated_at, last_success_at, last_error_at, disabled
		FROM push_subscriptions
		WHERE disabled = 0
		ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]PushSubscription, 0, 16)
	for rows.Next() {
		var item PushSubscription
		var disabled int
		var lastSuccess sql.NullInt64
		var lastError sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.Endpoint, &item.P256DH, &item.Auth, &item.UA, &item.CreatedAt, &item.UpdatedAt, &lastSuccess, &lastError, &disabled,
		); err != nil {
			return nil, err
		}
		if lastSuccess.Valid {
			v := lastSuccess.Int64
			item.LastSuccessAt = &v
		}
		if lastError.Valid {
			v := lastError.Int64
			item.LastErrorAt = &v
		}
		item.Disabled = disabled == 1
		items = append(items, item)
	}
	return items, rows.Err()
}

func (a *App) hasActivePushSubscription(ctx context.Context) (bool, error) {
	var count int64
	if err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM push_subscriptions
		WHERE disabled = 0`,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (a *App) hasActivePushSubscriptionByEndpoint(ctx context.Context, endpoint string) (bool, error) {
	var count int64
	if err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM push_subscriptions
		WHERE disabled = 0 AND endpoint = ?`,
		endpoint,
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (a *App) markPushSubscriptionSuccess(ctx context.Context, id int64) error {
	now := time.Now().Unix()
	_, err := a.db.ExecContext(ctx, `
		UPDATE push_subscriptions
		SET last_success_at = ?, updated_at = ?, disabled = 0
		WHERE id = ?`,
		now, now, id,
	)
	return err
}

func (a *App) markPushSubscriptionError(ctx context.Context, id int64, disable bool) error {
	now := time.Now().Unix()
	_, err := a.db.ExecContext(ctx, `
		UPDATE push_subscriptions
		SET last_error_at = ?, updated_at = ?, disabled = CASE WHEN ? = 1 THEN 1 ELSE disabled END
		WHERE id = ?`,
		now, now, boolToInt(disable), id,
	)
	return err
}

func (a *App) recordPushDelivery(ctx context.Context, success bool, message string, ts int64) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO push_delivery_logs (created_at, success, error)
		VALUES (?, ?, ?)`,
		ts, boolToInt(success), strings.TrimSpace(message),
	)
	return err
}

func (a *App) latestPushFailure(ctx context.Context) (string, error) {
	var message sql.NullString
	err := a.db.QueryRowContext(ctx, `
		SELECT error
		FROM push_delivery_logs
		WHERE success = 0
		ORDER BY id DESC
		LIMIT 1`,
	).Scan(&message)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if !message.Valid {
		return "", nil
	}
	return strings.TrimSpace(message.String), nil
}

func (a *App) initAuthAndWebhookSecret(ctx context.Context) error {
	if err := a.initAdminPasswordHash(ctx); err != nil {
		return err
	}
	secret, err := a.resolveWebhookSecret(ctx)
	if err != nil {
		return err
	}
	a.webhookSecret = secret
	return nil
}

func (a *App) initAdminPasswordHash(ctx context.Context) error {
	if strings.TrimSpace(a.cfg.AdminPassword) != "" {
		a.adminPasswordHash = hashString(a.cfg.AdminPassword)
		return nil
	}

	storedHash, err := a.getAppSetting(ctx, settingKeyAdminPassSHA256)
	if err != nil {
		return err
	}
	if strings.TrimSpace(storedHash) != "" {
		a.adminPasswordHash = storedHash
		return nil
	}

	generated, err := randomToken(24)
	if err != nil {
		return err
	}
	hash := hashString(generated)
	if err := a.setAppSetting(ctx, settingKeyAdminPassSHA256, hash); err != nil {
		return err
	}
	a.adminPasswordHash = hash
	log.Printf("generated ADMIN password for '%s': %s", a.cfg.AdminUsername, generated)
	log.Printf("set ADMIN_PASSWORD env var to override the generated password if needed")
	return nil
}

func (a *App) resolveWebhookSecret(ctx context.Context) (string, error) {
	if strings.TrimSpace(a.cfg.DiunWebhookSecret) != "" {
		return a.cfg.DiunWebhookSecret, nil
	}

	stored, err := a.getAppSetting(ctx, settingKeyWebhookSecret)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(stored) != "" {
		return stored, nil
	}

	secret, err := randomToken(32)
	if err != nil {
		return "", err
	}
	if err := a.setAppSetting(ctx, settingKeyWebhookSecret, secret); err != nil {
		return "", err
	}
	log.Printf("generated DIUN webhook secret and saved into /data app settings")
	return secret, nil
}

func (a *App) getAppSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := a.db.QueryRowContext(ctx,
		`SELECT value FROM app_settings WHERE key = ?`,
		key,
	).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func (a *App) setAppSetting(ctx context.Context, key, value string) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO app_settings (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().Unix(),
	)
	return err
}

func (a *App) validateCredentials(username, password string) bool {
	if subtleCompare(strings.TrimSpace(username), a.cfg.AdminUsername) == 0 {
		return false
	}
	if strings.TrimSpace(password) == "" {
		return false
	}
	return subtleCompare(hashString(password), a.adminPasswordHash) == 1
}

func (a *App) currentSessionUser(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	return a.sessions.Validate(cookie.Value)
}

func (a *App) requireSession(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := a.currentSessionUser(r); !ok {
		writeAPIError(w, http.StatusUnauthorized, "login_required", "login required")
		return false
	}
	return true
}

func (a *App) withinAutoUpdateWindow(now time.Time) bool {
	start := a.cfg.AutoWindowStart
	end := a.cfg.AutoWindowEnd
	if start < 0 || end < 0 {
		return true
	}
	if start == end {
		return true
	}
	hour := now.Hour()
	if start < end {
		return hour >= start && hour < end
	}
	return hour >= start || hour < end
}

func hashString(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func newAssetVersion() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func resolveBuildVersion() string {
	return resolveBuildVersionValue(buildVersion)
}

func resolveBuildVersionValue(raw string) string {
	version := strings.TrimSpace(raw)
	if version == "" {
		return "dev"
	}
	return version
}

func maskSecret(secret string) string {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:4] + "..." + trimmed[len(trimmed)-4:]
}

func randomToken(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be > 0")
	}
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

func buildLoginRateLimitKey(clientIP, username string) string {
	ip := strings.TrimSpace(strings.ToLower(clientIP))
	if ip == "" {
		ip = "-"
	}
	name := strings.TrimSpace(strings.ToLower(username))
	if name == "" {
		name = "-"
	}
	return ip + "|" + name
}

func clientIPFromRequest(r *http.Request) string {
	candidates := []string{
		strings.TrimSpace(r.Header.Get("X-Forwarded-For")),
		strings.TrimSpace(r.Header.Get("X-Real-IP")),
	}
	for _, raw := range candidates {
		if raw == "" {
			continue
		}
		first := strings.TrimSpace(strings.Split(raw, ",")[0])
		if first == "" {
			continue
		}
		if ip := net.ParseIP(first); ip != nil {
			return ip.String()
		}
		if host, _, err := net.SplitHostPort(first); err == nil {
			if ip := net.ParseIP(host); ip != nil {
				return ip.String()
			}
		}
	}
	remote := strings.TrimSpace(r.RemoteAddr)
	if remote == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if ip := net.ParseIP(remote); ip != nil {
		return ip.String()
	}
	return remote
}

func subtleCompare(a, b string) int {
	if len(a) != len(b) {
		return 0
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b))
}

func extractDiunFields(payload map[string]any) (string, string, string) {
	candidates := []string{
		getPathString(payload, "entry", "image", "name"),
		getPathString(payload, "entry", "image", "repository"),
		getPathString(payload, "entry", "image"),
		getPathString(payload, "entry", "repository"),
		getPathString(payload, "image", "name"),
		getPathString(payload, "image", "repository"),
		getPathString(payload, "repository"),
		getPathString(payload, "image"),
	}

	imageRepo := ""
	for _, c := range candidates {
		norm := normalizeImageRepo(c)
		if norm != "" {
			imageRepo = norm
			break
		}
	}

	tag := firstNonEmpty(
		getPathString(payload, "entry", "tag"),
		getPathString(payload, "tag"),
	)
	digest := firstNonEmpty(
		getPathString(payload, "entry", "digest"),
		getPathString(payload, "digest"),
	)
	return imageRepo, strings.TrimSpace(tag), strings.TrimSpace(digest)
}

func getPathString(m map[string]any, keys ...string) string {
	var current any = m
	for _, key := range keys {
		asMap, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = asMap[key]
		if !ok {
			return ""
		}
	}

	switch v := current.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func normalizeImageRepo(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	if at := strings.Index(raw, "@"); at >= 0 {
		raw = raw[:at]
	}
	lastSlash := strings.LastIndex(raw, "/")
	lastColon := strings.LastIndex(raw, ":")
	if lastColon > lastSlash {
		raw = raw[:lastColon]
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parts := strings.Split(raw, "/")
	if len(parts) == 1 {
		return strings.Join([]string{"docker.io", "library", parts[0]}, "/")
	}

	if parts[0] == "index.docker.io" {
		parts[0] = "docker.io"
	}
	if !isExplicitRegistryHost(parts[0]) {
		parts = append([]string{"docker.io"}, parts...)
	}
	if parts[0] == "docker.io" && len(parts) == 2 {
		parts = []string{"docker.io", "library", parts[1]}
	}
	return strings.Join(parts, "/")
}

func isExplicitRegistryHost(host string) bool {
	return host == "localhost" || strings.Contains(host, ".") || strings.Contains(host, ":")
}

func isSQLiteMissingTableErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "no such table")
}

func validateComposeDir(root, dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("compose_dir is required")
	}
	cleanRoot := filepath.Clean(root)
	cleanDir := filepath.Clean(dir)
	if !filepath.IsAbs(cleanDir) {
		return "", errors.New("compose_dir must be absolute")
	}
	rel, err := filepath.Rel(cleanRoot, cleanDir)
	if err != nil {
		return "", errors.New("compose_dir must be under container root")
	}
	if rel == "." || rel == "" {
		return "", errors.New("compose_dir must be /share/Container/<name>")
	}
	if strings.HasPrefix(rel, "..") || strings.Contains(rel, string(filepath.Separator)+"..") {
		return "", errors.New("compose_dir must be under container root")
	}
	if strings.Contains(rel, string(filepath.Separator)) {
		return "", errors.New("compose_dir must be one-level path: /share/Container/<name>")
	}
	return cleanDir, nil
}

func validateComposeFile(composeFile string) error {
	composeFile = strings.TrimSpace(composeFile)
	if composeFile == "" {
		return errors.New("compose_file is required")
	}
	if composeFile != filepath.Base(composeFile) {
		return errors.New("compose_file must be file name only")
	}
	if strings.Contains(composeFile, "..") {
		return errors.New("compose_file cannot include '..'")
	}
	return nil
}

func parseIDPath(path, prefix string) (int64, bool) {
	raw := strings.TrimPrefix(path, prefix)
	if raw == path {
		return 0, false
	}
	if strings.Contains(raw, "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func parseStreamJobID(path string) (int64, bool) {
	raw := strings.TrimPrefix(path, "/api/jobs/")
	if raw == path {
		return 0, false
	}
	parts := strings.Split(raw, "/")
	if len(parts) != 2 || parts[1] != "stream" {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func joinInt64(items []int64, sep string) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, v := range items {
		parts = append(parts, strconv.FormatInt(v, 10))
	}
	return strings.Join(parts, sep)
}

func isAllowedDBPath(dbPath string) bool {
	clean := filepath.Clean(strings.TrimSpace(dbPath))
	if !filepath.IsAbs(clean) {
		return false
	}
	rel, err := filepath.Rel("/data", clean)
	if err != nil {
		return false
	}
	if rel == "." || rel == "" {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func decodeJSON(r io.Reader, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r, 1024*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiErrorResponse{
		Error: message,
		Code:  code,
	})
}

func writeAPIErrorFromErr(w http.ResponseWriter, status int, code string, err error) {
	if err == nil {
		writeAPIError(w, status, code, "unknown error")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		writeAPIError(w, http.StatusGatewayTimeout, "request_timeout", "request timeout")
		return
	}
	if errors.Is(err, context.Canceled) {
		writeAPIError(w, http.StatusRequestTimeout, "request_canceled", "request canceled")
		return
	}
	writeAPIError(w, status, code, err.Error())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeSSE(w io.Writer, event string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(raw))
	return err
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func withAPITimeout(next http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	var reqID int64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := atomic.AddInt64(&reqID, 1)
		started := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		log.Printf("req[%d] %s %s", id, r.Method, r.URL.Path)
		next.ServeHTTP(rec, r)
		log.Printf("req[%d] done status=%d in %s", id, rec.status, time.Since(started).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijacker not supported")
	}
	return h.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	p, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}

func newLogBroker() *LogBroker {
	return &LogBroker{
		subs: map[int64]map[chan string]struct{}{},
	}
}

func newDashboardEventBroker() *DashboardEventBroker {
	return &DashboardEventBroker{
		subs: map[chan DashboardPatchEvent]struct{}{},
	}
}

func newSSELimiter(max int) *SSELimiter {
	if max <= 0 {
		max = 20
	}
	return &SSELimiter{
		max: int64(max),
	}
}

func (l *SSELimiter) Acquire() bool {
	for {
		current := l.active.Load()
		if current >= l.max {
			l.rejected.Add(1)
			return false
		}
		if l.active.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (l *SSELimiter) Release() {
	for {
		current := l.active.Load()
		if current <= 0 {
			return
		}
		if l.active.CompareAndSwap(current, current-1) {
			return
		}
	}
}

func (l *SSELimiter) Active() int64 {
	return l.active.Load()
}

func (l *SSELimiter) Stats() (active, rejected int64) {
	return l.active.Load(), l.rejected.Load()
}

func newLoginRateLimiter(cfg LoginRateLimiterConfig) *LoginRateLimiter {
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 600
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	if cfg.LockSeconds <= 0 {
		cfg.LockSeconds = 900
	}
	return &LoginRateLimiter{
		cfg:     cfg,
		entries: map[string]loginRateEntry{},
	}
}

func (l *LoginRateLimiter) Check(key string, now time.Time) (blocked bool, retryAfterSeconds int) {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return false, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	nowUnix := now.Unix()
	entry, ok := l.entries[key]
	if !ok {
		return false, 0
	}
	entry.Failures = trimOldFailures(entry.Failures, nowUnix, int64(l.cfg.WindowSeconds))
	if entry.LockedUntil <= nowUnix {
		entry.LockedUntil = 0
	}
	if len(entry.Failures) == 0 && entry.LockedUntil == 0 {
		delete(l.entries, key)
		return false, 0
	}
	l.entries[key] = entry
	if entry.LockedUntil > nowUnix {
		return true, int(entry.LockedUntil - nowUnix)
	}
	return false, 0
}

func (l *LoginRateLimiter) RecordFailure(key string, now time.Time) {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	nowUnix := now.Unix()
	entry := l.entries[key]
	entry.Failures = trimOldFailures(entry.Failures, nowUnix, int64(l.cfg.WindowSeconds))
	entry.Failures = append(entry.Failures, nowUnix)
	if len(entry.Failures) >= l.cfg.MaxAttempts {
		entry.LockedUntil = nowUnix + int64(l.cfg.LockSeconds)
		entry.Failures = nil
	}
	l.entries[key] = entry
}

func (l *LoginRateLimiter) Reset(key string) {
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func trimOldFailures(values []int64, nowUnix int64, windowSeconds int64) []int64 {
	if len(values) == 0 {
		return values
	}
	threshold := nowUnix - windowSeconds
	pos := 0
	for pos < len(values) && values[pos] < threshold {
		pos++
	}
	if pos >= len(values) {
		return nil
	}
	return append([]int64(nil), values[pos:]...)
}

func newSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	return &SessionStore{
		ttl:      ttl,
		sessions: map[string]SessionEntry{},
	}
}

func (s *SessionStore) Create(username string) (string, time.Time, error) {
	return s.CreateWithTTL(username, s.ttl)
}

func (s *SessionStore) CreateWithTTL(username string, ttl time.Duration) (string, time.Time, error) {
	if ttl <= 0 {
		ttl = s.ttl
	}
	token, err := randomToken(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	s.sessions[token] = SessionEntry{
		Username: username,
		Expires:  expires,
	}
	return token, expires, nil
}

func (s *SessionStore) Validate(token string) (string, bool) {
	if strings.TrimSpace(token) == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[token]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.Expires) {
		delete(s.sessions, token)
		return "", false
	}
	return entry.Username, true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func (s *SessionStore) cleanupLocked() {
	now := time.Now()
	for token, entry := range s.sessions {
		if now.After(entry.Expires) {
			delete(s.sessions, token)
		}
	}
}

func (b *LogBroker) Subscribe(jobID int64) (<-chan string, func()) {
	ch := make(chan string, 128)
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subs[jobID]; !ok {
		b.subs[jobID] = map[chan string]struct{}{}
	}
	b.subs[jobID][ch] = struct{}{}

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		jobSubs, ok := b.subs[jobID]
		if !ok {
			return
		}
		delete(jobSubs, ch)
		close(ch)
		if len(jobSubs) == 0 {
			delete(b.subs, jobID)
		}
	}
}

func (b *LogBroker) Publish(jobID int64, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[jobID] {
		select {
		case ch <- line:
		default:
		}
	}
}

func (b *DashboardEventBroker) Subscribe() (<-chan DashboardPatchEvent, func()) {
	ch := make(chan DashboardPatchEvent, 128)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs, ch)
		close(ch)
	}
}

func (b *DashboardEventBroker) Publish(evt DashboardPatchEvent) {
	if evt.OccurredAt <= 0 {
		evt.OccurredAt = time.Now().Unix()
	}
	evt.Seq = b.seq.Add(1)

	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func uniqueStringSlice(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

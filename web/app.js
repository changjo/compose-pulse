const authState = document.getElementById("auth-state");
const logoutBtn = document.getElementById("logout-btn");
const langSelect = document.getElementById("lang-select");
const advancedToggleBtn = document.getElementById("advanced-toggle");
const webhookConfig = document.getElementById("webhook-config");
const mainLayout = document.querySelector("main.layout");
const advancedSections = Array.from(document.querySelectorAll(".advanced-only"));

const installBanner = document.getElementById("install-banner");
const installBtn = document.getElementById("install-btn");
const installHelp = document.getElementById("install-help");
const pullRefreshIndicator = document.getElementById("pull-refresh-indicator");
const pullRefreshProgress = document.getElementById("pull-refresh-progress");

const addTargetForm = document.getElementById("add-target-form");
const discoverItemSelect = document.getElementById("discover-item-select");
const discoverImageSelect = document.getElementById("discover-image-select");
const refreshDiscoverBtn = document.getElementById("refresh-discover");
const discoverMeta = document.getElementById("discover-meta");
const registerTargetBtn = addTargetForm.querySelector('button[type="submit"]');
const targetsPanel = document.getElementById("targets-panel");
const targetsContent = document.getElementById("targets-content");
const targetsCollapseBtn = document.getElementById("targets-collapse-btn");
const auditPanel = document.getElementById("audit-panel");
const auditContent = document.getElementById("audit-content");
const auditCollapseBtn = document.getElementById("audit-collapse-btn");

const targetsBody = document.getElementById("targets-body");
const auditBody = document.getElementById("audit-body");
const eventsBody = document.getElementById("events-body");
const receiptsBody = document.getElementById("receipts-body");
const jobsBody = document.getElementById("jobs-body");
const jobsMoreBtn = document.getElementById("jobs-more");
const jobsExportBtn = document.getElementById("jobs-export");
const jobsDeleteAllBtn = document.getElementById("jobs-delete-all");
const toggleSelectAllBtn = document.getElementById("toggle-select-all");
const runUpdateBtn = document.getElementById("run-update");
const runPruneBtn = document.getElementById("run-prune");
const logBox = document.getElementById("log-box");
const streamMeta = document.getElementById("stream-meta");

const metricFailed = document.getElementById("metric-failed");
const metricAvg = document.getElementById("metric-avg");
const metricWebhook = document.getElementById("metric-webhook");
const metricWebhookFailed = document.getElementById("metric-webhook-failed");
const metricLoginFailed = document.getElementById("metric-login-failed");
const metricLoginLimited = document.getElementById("metric-login-limited");
const metricDashboardStream = document.getElementById("metric-dashboard-stream");
const metricPushDelivery = document.getElementById("metric-push-delivery");
const pushStatus = document.getElementById("push-status");
const pushEnableBtn = document.getElementById("push-enable");
const pushDisableBtn = document.getElementById("push-disable");
const pushTestBtn = document.getElementById("push-test");

const I18N = {
  ko: {
    "app.title": "ComposePulse",
    "label.language": "언어",
    "section.metrics": "운영 지표 (24h)",
    "metrics.failed": "실패/차단 작업",
    "metrics.avg_duration": "평균 작업 시간(초)",
    "metrics.webhook": "Webhook 실패율",
    "metrics.webhook_failed": "Webhook 실패 건수",
    "metrics.login_failures": "로그인 실패",
    "metrics.login_limited": "로그인 제한",
    "metrics.dashboard_stream": "Dashboard SSE (active/rejected)",
    "metrics.push_delivery": "Push 전송 성공/실패",
    "section.register": "컨테이너 등록",
    "section.targets": "등록 컨테이너 목록",
    "section.audit": "대상별 감사 요약",
    "section.diun_events": "DIUN 최근 이벤트",
    "section.diun_receipts": "Webhook 수신 로그",
    "section.push": "Push 알림",
    "section.jobs": "작업 이력",
    "section.logs": "실시간 로그",
    "col.name": "Name",
    "col.id": "ID",
    "col.select": "선택",
    "col.type": "Type",
    "col.trigger": "Trigger",
    "col.status": "Status",
    "col.compose_dir": "Compose Dir",
    "col.image_repo": "Image Repo",
    "col.tag": "Tag",
    "col.matched": "Matched",
    "col.queued": "Queued",
    "col.auto": "Auto",
    "col.enabled": "Enabled",
    "col.cooldown": "Cooldown(s)",
    "col.last_success": "Last Success",
    "col.action": "Action",
    "col.total_runs": "Total",
    "col.success_runs": "Success",
    "col.failed_runs": "Failed",
    "col.blocked_runs": "Blocked",
    "col.avg_duration": "Avg Duration",
    "col.last_run": "Last Run",
    "col.received_at": "Received At",
    "col.status_code": "Status",
    "col.reason_code": "Reason",
    "col.job_id": "Job ID",
    "col.targets": "Targets",
    "col.duration": "Duration",
    "col.error": "Error",
    "col.created": "Created",
    "btn.register": "등록",
    "btn.refresh_discover": "목록 새로고침",
    "btn.toggle_select_all": "전체 선택/해제",
    "btn.advanced_on": "고급 보기 ON",
    "btn.advanced_off": "고급 보기 OFF",
    "btn.targets_collapse": "목록 접기",
    "btn.targets_expand": "목록 펼치기",
    "btn.audit_collapse": "목록 접기",
    "btn.audit_expand": "목록 펼치기",
    "btn.run_update": "선택 업데이트 실행",
    "btn.run_prune": "Dangling Image Prune",
    "btn.jobs_delete_all": "작업 이력 전체 삭제",
    "btn.jobs_more": "이전 작업 더 보기",
    "btn.export_csv": "CSV 내보내기",
    "btn.details_expand": "상세",
    "btn.details_collapse": "접기",
    "btn.view_logs": "로그",
    "btn.push_enable": "Push 활성화",
    "btn.push_disable": "Push 비활성화",
    "btn.push_test": "Push 테스트",
    "auth.logout": "Logout",
    "auth.login_required": "로그인이 필요합니다.",
    "discover.loading": "탐색 목록을 불러오는 중...",
    "discover.empty": "등록 가능한 컨테이너를 찾지 못했습니다.",
    "discover.select_first": "먼저 컨테이너를 선택하세요.",
    "discover.no_image": "이미지 후보 없음 (compose image 파싱 실패)",
    "discover.already_registered": "이미 등록됨",
    "discover.none_available": "모든 후보가 이미 등록되어 있습니다.",
    "discover.meta": "compose_dir={dir}, compose_file={file}",
    "discover.load_failed": "컨테이너 탐색 실패: {error}",
    "discover.select_required": "등록할 컨테이너를 선택하세요.",
    "discover.image_required": "이미지 후보를 선택하세요.",
    "target.cooldown.placeholder": "600",
    "target.delete": "Delete",
    "target.select_required": "업데이트할 컨테이너를 선택하세요. (Select 체크박스)",
    "target.update_failed": "대상 업데이트 실패: {error}",
    "target.delete_confirm": "target {id}를 삭제할까요? (실행/대기 작업이 있으면 거부됩니다)",
    "target.delete_failed": "삭제 실패: {error}",
    "target.register_failed": "등록 실패: {error}",
    "job.update_queued": "업데이트 작업이 큐에 등록되었습니다. job_id={jobId}",
    "job.update_queued_partial": "업데이트 작업이 큐에 등록되었습니다. job_id={jobId}, active 작업으로 skipped={count}",
    "job.prune_queued": "prune 작업이 큐에 등록되었습니다. job_id={jobId}",
    "job.delete_all_confirm": "작업 이력을 전체 삭제할까요? (queued/running 작업이 있으면 거부됩니다)",
    "job.delete_all_done": "작업 이력 {count}건을 삭제했습니다.",
    "job.delete_all_failed": "작업 이력 전체 삭제 실패: {error}",
    "job.delete_confirm": "job {jobId}를 삭제할까요? (queued/running은 삭제 불가)",
    "job.delete_failed": "작업 삭제 실패: {error}",
    "job.failed": "실패: {error}",
    "prune.confirm": "Dangling 이미지만 정리(docker image prune -f)합니다. 실행할까요?",
    "webhook.loading": "Webhook 설정 정보를 불러오는 중...",
    "webhook.load_failed": "Webhook 설정 정보를 불러오지 못했습니다.",
    "webhook.config_text": "DIUN webhook: path={path}, header={header}, secret={secret}",
    "webhook.secret_hidden": "(숨김)",
    "audit.empty": "요약 데이터가 없습니다.",
    "csv.download_failed": "CSV 다운로드 실패: {error}",
    "stream.click_job": "작업을 클릭하면 로그가 표시됩니다.",
    "stream.streaming": "job {jobId} 로그 스트리밍 중",
    "stream.done_status": "job {jobId} 완료 (status={status})",
    "stream.done": "job {jobId} 완료",
    "stream.ended": "job {jobId} 스트림 연결이 종료되었습니다.",
    "push.status_loading": "Push 상태를 확인하는 중...",
    "push.not_supported": "이 브라우저는 Push를 지원하지 않습니다.",
    "push.disabled_server": "서버에서 Push 기능이 비활성화되어 있습니다.",
    "push.permission_denied": "브라우저 알림 권한이 거부되어 있습니다.",
    "push.ready_not_subscribed": "Push 사용 가능 (현재 미구독)",
    "push.ready_subscribed": "Push 사용 가능 (구독됨)",
    "push.ready_other_device": "현재 기기는 미구독 (다른 기기는 구독됨)",
    "push.enabled": "Push 구독이 활성화되었습니다.",
    "push.disabled": "Push 구독이 비활성화되었습니다.",
    "push.enable_failed": "Push 활성화 실패: {error}",
    "push.disable_failed": "Push 비활성화 실패: {error}",
    "push.test_sent": "Push 테스트 전송: 성공 {sent}, 실패 {failed}",
    "push.test_sent_with_error": "Push 테스트 전송: 성공 {sent}, 실패 {failed} (최근 오류: {error})",
    "push.test_failed": "Push 테스트 실패: {error}",
    "status.queued": "queued",
    "status.running": "running",
    "status.success": "success",
    "status.failed": "failed",
    "status.blocked": "blocked",
    "install.button": "앱 설치",
    "install.available": "설치 버튼으로 홈 화면에 앱을 추가할 수 있습니다.",
    "install.ios_help": "Safari 공유 버튼을 눌러 '홈 화면에 추가'를 선택하세요.",
    "install.android_help": "브라우저 메뉴에서 '앱 설치' 또는 '홈 화면에 추가'를 선택하세요.",
    "install.browser_help": "이 브라우저에서 앱 설치 메뉴를 사용하세요.",
  },
  en: {
    "app.title": "ComposePulse",
    "label.language": "Language",
    "section.metrics": "Operational Metrics (24h)",
    "metrics.failed": "Failed/Blocked Jobs",
    "metrics.avg_duration": "Average Duration (sec)",
    "metrics.webhook": "Webhook Failure Rate",
    "metrics.webhook_failed": "Webhook Failed Count",
    "metrics.login_failures": "Login Failures",
    "metrics.login_limited": "Rate-Limited Logins",
    "metrics.dashboard_stream": "Dashboard SSE (active/rejected)",
    "metrics.push_delivery": "Push sent/failed",
    "section.register": "Register Container",
    "section.targets": "Registered Containers",
    "section.audit": "Audit Summary by Target",
    "section.diun_events": "Recent DIUN Events",
    "section.diun_receipts": "Webhook Receipts",
    "section.push": "Push Notifications",
    "section.jobs": "Job History",
    "section.logs": "Live Logs",
    "col.name": "Name",
    "col.id": "ID",
    "col.select": "Select",
    "col.type": "Type",
    "col.trigger": "Trigger",
    "col.status": "Status",
    "col.compose_dir": "Compose Dir",
    "col.image_repo": "Image Repo",
    "col.tag": "Tag",
    "col.matched": "Matched",
    "col.queued": "Queued",
    "col.auto": "Auto",
    "col.enabled": "Enabled",
    "col.cooldown": "Cooldown(s)",
    "col.last_success": "Last Success",
    "col.action": "Action",
    "col.total_runs": "Total",
    "col.success_runs": "Success",
    "col.failed_runs": "Failed",
    "col.blocked_runs": "Blocked",
    "col.avg_duration": "Avg Duration",
    "col.last_run": "Last Run",
    "col.received_at": "Received At",
    "col.status_code": "Status",
    "col.reason_code": "Reason",
    "col.job_id": "Job ID",
    "col.targets": "Targets",
    "col.duration": "Duration",
    "col.error": "Error",
    "col.created": "Created",
    "btn.register": "Register",
    "btn.refresh_discover": "Refresh List",
    "btn.toggle_select_all": "Toggle Select All",
    "btn.advanced_on": "Advanced ON",
    "btn.advanced_off": "Advanced OFF",
    "btn.targets_collapse": "Collapse List",
    "btn.targets_expand": "Expand List",
    "btn.audit_collapse": "Collapse List",
    "btn.audit_expand": "Expand List",
    "btn.run_update": "Run Selected Update",
    "btn.run_prune": "Dangling Image Prune",
    "btn.jobs_delete_all": "Delete All Job History",
    "btn.jobs_more": "Load Older Jobs",
    "btn.export_csv": "Export CSV",
    "btn.details_expand": "Details",
    "btn.details_collapse": "Hide",
    "btn.view_logs": "Logs",
    "btn.push_enable": "Enable Push",
    "btn.push_disable": "Disable Push",
    "btn.push_test": "Send Push Test",
    "auth.logout": "Logout",
    "auth.login_required": "Login is required.",
    "discover.loading": "Loading discovered containers...",
    "discover.empty": "No registerable containers found.",
    "discover.select_first": "Select a container first.",
    "discover.no_image": "No image candidates (compose image parse failed)",
    "discover.already_registered": "already registered",
    "discover.none_available": "All discovered items are already registered.",
    "discover.meta": "compose_dir={dir}, compose_file={file}",
    "discover.load_failed": "Container discovery failed: {error}",
    "discover.select_required": "Select a container to register.",
    "discover.image_required": "Select an image candidate.",
    "target.cooldown.placeholder": "600",
    "target.delete": "Delete",
    "target.select_required": "Select at least one container to update (Select checkbox).",
    "target.update_failed": "Target update failed: {error}",
    "target.delete_confirm": "Delete target {id}? (blocked if queued/running jobs exist)",
    "target.delete_failed": "Delete failed: {error}",
    "target.register_failed": "Register failed: {error}",
    "job.update_queued": "Update job queued. job_id={jobId}",
    "job.update_queued_partial": "Update job queued. job_id={jobId}, skipped active targets={count}",
    "job.prune_queued": "Prune job queued. job_id={jobId}",
    "job.delete_all_confirm": "Delete all job history? (rejected while queued/running jobs exist)",
    "job.delete_all_done": "Deleted {count} job history records.",
    "job.delete_all_failed": "Delete all job history failed: {error}",
    "job.delete_confirm": "Delete job {jobId}? (queued/running cannot be deleted)",
    "job.delete_failed": "Delete job failed: {error}",
    "job.failed": "Failed: {error}",
    "prune.confirm": "This runs docker image prune -f (dangling only). Continue?",
    "webhook.loading": "Loading webhook configuration...",
    "webhook.load_failed": "Failed to load webhook configuration.",
    "webhook.config_text": "DIUN webhook: path={path}, header={header}, secret={secret}",
    "webhook.secret_hidden": "(hidden)",
    "audit.empty": "No audit data available.",
    "csv.download_failed": "CSV download failed: {error}",
    "stream.click_job": "Click a job row to view logs.",
    "stream.streaming": "Streaming logs for job {jobId}",
    "stream.done_status": "job {jobId} completed (status={status})",
    "stream.done": "job {jobId} completed",
    "stream.ended": "job {jobId} stream has ended.",
    "push.status_loading": "Checking push status...",
    "push.not_supported": "Push is not supported by this browser.",
    "push.disabled_server": "Push is disabled on the server.",
    "push.permission_denied": "Browser notification permission is denied.",
    "push.ready_not_subscribed": "Push is available (not subscribed).",
    "push.ready_subscribed": "Push is available (subscribed).",
    "push.ready_other_device": "This device is not subscribed (another device is subscribed).",
    "push.enabled": "Push subscription enabled.",
    "push.disabled": "Push subscription disabled.",
    "push.enable_failed": "Enable push failed: {error}",
    "push.disable_failed": "Disable push failed: {error}",
    "push.test_sent": "Push test sent: success {sent}, failed {failed}",
    "push.test_sent_with_error": "Push test sent: success {sent}, failed {failed} (latest error: {error})",
    "push.test_failed": "Push test failed: {error}",
    "status.queued": "queued",
    "status.running": "running",
    "status.success": "success",
    "status.failed": "failed",
    "status.blocked": "blocked",
    "install.button": "Install App",
    "install.available": "Use the install button to add this app to your home screen.",
    "install.ios_help": "Tap Safari Share and choose 'Add to Home Screen'.",
    "install.android_help": "Use browser menu: 'Install app' or 'Add to Home screen'.",
    "install.browser_help": "Use your browser's install/add-to-home-screen menu.",
  },
};

const DEFAULT_LANG = "ko";
let currentLang = DEFAULT_LANG;
let currentUsername = "";
let jobsNextCursor = null;
let streamSource = null;
let dashboardSource = null;
let dashboardReconnectAttempt = 0;
let dashboardReconnectTimer = null;
let fallbackPollingTimer = null;
let dashboardConnected = false;
const DASHBOARD_RECONNECT_DELAYS = [1000, 2000, 5000, 10000, 20000];
const FALLBACK_POLL_INTERVAL_MS = 30000;
const FETCH_TIMEOUT_MS = 15000;
const SERVICE_WORKER_READY_TIMEOUT_MS = 2500;
let deferredInstallPrompt = null;
let serviceWorkerRegistration = null;
let discoveredItems = [];
let currentTargets = [];
let currentAuditSummaries = [];
let currentDiunEvents = [];
let currentWebhookReceipts = [];
let currentJobs = [];
let lastCompactViewport = false;
const compactExpandedBySection = {
  targets: null,
  audit: null,
  events: null,
  receipts: null,
  jobs: null,
};
let selectedTargetIdSet = new Set();
let redirectingToLogin = false;
let showAdvanced = false;
let targetsCollapsed = false;
let auditCollapsed = false;
let pullRefreshing = false;
let lastPullRefreshAt = 0;
let touchStartX = 0;
let touchStartY = 0;
let touchTracking = false;
let touchPullDistance = 0;
let touchPointerId = null;
const ADVANCED_VISIBILITY_KEY = "composepulse:show-advanced";
const TARGETS_COLLAPSED_KEY = "composepulse:targets-collapsed";
const AUDIT_COLLAPSED_KEY = "composepulse:audit-collapsed";
const PULL_REFRESH_THRESHOLD_PX = 72;
const PULL_REFRESH_MIN_INTERVAL_MS = 1500;
const PULL_REFRESH_MIN_SPINNER_MS = 420;
const PULL_REFRESH_RING_CIRCUMFERENCE = 87.9646;

function t(key, vars = {}) {
  const table = I18N[currentLang] || I18N[DEFAULT_LANG];
  let text = table[key] || I18N[DEFAULT_LANG][key] || key;
  for (const [name, value] of Object.entries(vars)) {
    text = text.replaceAll(`{${name}}`, String(value));
  }
  return text;
}

function saveLanguage(lang) {
  try {
    localStorage.setItem("composepulse:lang", lang);
  } catch (_) {
    // ignore localStorage errors
  }
}

function loadLanguage() {
  try {
    const lang = localStorage.getItem("composepulse:lang");
    if (lang === "ko" || lang === "en") {
      return lang;
    }
  } catch (_) {
    // ignore localStorage errors
  }
  return DEFAULT_LANG;
}

function saveAdvancedVisibility(enabled) {
  try {
    localStorage.setItem(ADVANCED_VISIBILITY_KEY, enabled ? "1" : "0");
  } catch (_) {
    // ignore localStorage errors
  }
}

function loadAdvancedVisibility() {
  try {
    const raw = localStorage.getItem(ADVANCED_VISIBILITY_KEY);
    if (raw === "1" || raw === "true") return true;
    if (raw === "0" || raw === "false") return false;
  } catch (_) {
    // ignore localStorage errors
  }
  return false;
}

function saveTargetsCollapsed(collapsed) {
  try {
    localStorage.setItem(TARGETS_COLLAPSED_KEY, collapsed ? "1" : "0");
  } catch (_) {
    // ignore localStorage errors
  }
}

function loadTargetsCollapsed() {
  try {
    const raw = localStorage.getItem(TARGETS_COLLAPSED_KEY);
    if (raw === "1" || raw === "true") return true;
    if (raw === "0" || raw === "false") return false;
  } catch (_) {
    // ignore localStorage errors
  }
  return null;
}

function saveAuditCollapsed(collapsed) {
  try {
    localStorage.setItem(AUDIT_COLLAPSED_KEY, collapsed ? "1" : "0");
  } catch (_) {
    // ignore localStorage errors
  }
}

function loadAuditCollapsed() {
  try {
    const raw = localStorage.getItem(AUDIT_COLLAPSED_KEY);
    if (raw === "1" || raw === "true") return true;
    if (raw === "0" || raw === "false") return false;
  } catch (_) {
    // ignore localStorage errors
  }
  return null;
}

function isNarrowViewport() {
  return window.matchMedia("(max-width: 820px)").matches;
}

function updateTargetsCollapseButton() {
  if (!targetsCollapseBtn) return;
  const key = targetsCollapsed ? "btn.targets_expand" : "btn.targets_collapse";
  targetsCollapseBtn.textContent = t(key);
  targetsCollapseBtn.setAttribute("aria-expanded", String(!targetsCollapsed));
}

function applyTargetsCollapsedState() {
  if (!targetsPanel || !targetsContent) return;
  const shouldCollapse = targetsCollapsed && isNarrowViewport();
  targetsPanel.classList.toggle("targets-collapsed", shouldCollapse);
  updateTargetsCollapseButton();
}

function toggleTargetsCollapsed() {
  targetsCollapsed = !targetsCollapsed;
  saveTargetsCollapsed(targetsCollapsed);
  applyTargetsCollapsedState();
}

function initTargetsCollapse() {
  if (!targetsPanel || !targetsCollapseBtn || !targetsContent) return;
  const saved = loadTargetsCollapsed();
  targetsCollapsed = saved === null ? true : saved;
  applyTargetsCollapsedState();
  targetsCollapseBtn.addEventListener("click", toggleTargetsCollapsed);
  window.addEventListener("resize", applyTargetsCollapsedState, { passive: true });
}

function updateAuditCollapseButton() {
  if (!auditCollapseBtn) return;
  const key = auditCollapsed ? "btn.audit_expand" : "btn.audit_collapse";
  auditCollapseBtn.textContent = t(key);
  auditCollapseBtn.setAttribute("aria-expanded", String(!auditCollapsed));
}

function applyAuditCollapsedState() {
  if (!auditPanel || !auditContent) return;
  const shouldCollapse = auditCollapsed && isNarrowViewport();
  auditPanel.classList.toggle("audit-collapsed", shouldCollapse);
  updateAuditCollapseButton();
}

function toggleAuditCollapsed() {
  auditCollapsed = !auditCollapsed;
  saveAuditCollapsed(auditCollapsed);
  applyAuditCollapsedState();
}

function initAuditCollapse() {
  if (!auditPanel || !auditCollapseBtn || !auditContent) return;
  const saved = loadAuditCollapsed();
  auditCollapsed = saved === null ? true : saved;
  applyAuditCollapsedState();
  auditCollapseBtn.addEventListener("click", toggleAuditCollapsed);
  window.addEventListener("resize", applyAuditCollapsedState, { passive: true });
}

function updateAdvancedToggleButton() {
  if (!advancedToggleBtn) return;
  const key = showAdvanced ? "btn.advanced_on" : "btn.advanced_off";
  advancedToggleBtn.textContent = t(key);
  advancedToggleBtn.setAttribute("aria-pressed", String(showAdvanced));
}

function applyAdvancedVisibility() {
  advancedSections.forEach((section) => {
    section.classList.toggle("hidden", !showAdvanced);
  });
  updateAdvancedToggleButton();
}

function setAdvancedVisibility(enabled, options = {}) {
  showAdvanced = Boolean(enabled);
  if (options.persist !== false) {
    saveAdvancedVisibility(showAdvanced);
  }
  applyAdvancedVisibility();
}

function toggleAdvancedVisibility() {
  setAdvancedVisibility(!showAdvanced);
  void reloadAll({ includeAdvanced: showAdvanced });
}

function applyI18n() {
  document.title = t("app.title");
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const key = el.getAttribute("data-i18n");
    if (!key) return;
    el.textContent = t(key);
  });
  document.querySelectorAll("[data-i18n-placeholder]").forEach((el) => {
    const key = el.getAttribute("data-i18n-placeholder");
    if (!key) return;
    el.setAttribute("placeholder", t(key));
  });
}

function setLanguage(lang) {
  if (lang !== "ko" && lang !== "en") {
    lang = DEFAULT_LANG;
  }
  currentLang = lang;
  langSelect.value = lang;
  saveLanguage(lang);
  applyI18n();
  updateInstallUI();
  renderDiscoverList();
  updateAdvancedToggleButton();
  updateTargetsCollapseButton();
  updateAuditCollapseButton();
}

function redirectToLogin() {
  if (redirectingToLogin) return;
  const path = window.location.pathname || "/";
  if (path === "/login" || path === "/login/") return;
  redirectingToLogin = true;
  window.location.replace("/login");
}

async function fetchJSON(url, init = {}) {
  const supportsAbort = typeof AbortController !== "undefined";
  const controller = supportsAbort ? new AbortController() : null;
  const timeoutId =
    controller &&
    window.setTimeout(() => {
      controller.abort();
    }, FETCH_TIMEOUT_MS);

  try {
    const response = await fetch(url, {
      credentials: "same-origin",
      ...init,
      signal: controller ? controller.signal : init.signal,
    });
    if (!response.ok) {
      const raw = await response.text();
      let message = raw || `${response.status} ${response.statusText}`;
      let code = "";
      try {
        const parsed = JSON.parse(raw);
        if (parsed && typeof parsed.error === "string") {
          message = parsed.error;
        }
        if (parsed && typeof parsed.code === "string") {
          code = parsed.code;
        }
      } catch (_) {
        // keep raw error
      }
      const err = new Error(code ? `${message} (${code})` : message);
      err.status = response.status;
      throw err;
    }
    return response.json();
  } catch (err) {
    if (err && err.name === "AbortError") {
      throw new Error("request timeout");
    }
    throw err;
  } finally {
    if (timeoutId) {
      clearTimeout(timeoutId);
    }
  }
}

async function runLoaderBatch(loaders, context) {
  const settled = await Promise.allSettled(
    loaders.map(async ([name, loader]) => {
      await loader();
      return name;
    })
  );

  settled.forEach((result, index) => {
    if (result.status !== "rejected") {
      return;
    }
    if (handleUnauthorized(result.reason)) {
      return;
    }
    const name = loaders[index] ? loaders[index][0] : `loader-${index}`;
    if (context) {
      console.error(`${context}: ${name} failed`, result.reason);
      return;
    }
    console.error(`${name} failed`, result.reason);
  });
}

function serviceWorkerReadyWithTimeout(timeoutMs) {
  return new Promise((resolve) => {
    let finished = false;
    const finish = (value) => {
      if (finished) return;
      finished = true;
      resolve(value);
    };

    const timer = window.setTimeout(() => {
      finish(null);
    }, timeoutMs);

    navigator.serviceWorker.ready
      .then((registration) => {
        clearTimeout(timer);
        finish(registration || null);
      })
      .catch(() => {
        clearTimeout(timer);
        finish(null);
      });
  });
}

async function lookupExistingServiceWorkerRegistration() {
  try {
    const registration = await navigator.serviceWorker.getRegistration();
    if (registration) {
      serviceWorkerRegistration = registration;
      return registration;
    }
  } catch (_) {
    // ignore service worker lookup failures
  }
  return null;
}

function getLoadersForReload(options = {}) {
  const includeDiscover = options.includeDiscover === true;
  const includeAdvanced = options.includeAdvanced === undefined ? showAdvanced : Boolean(options.includeAdvanced);
  const primaryLoaders = [["targets", loadTargets]];
  const advancedPrimaryLoaders = [];
  const advancedSecondaryLoaders = [];

  if (includeDiscover) {
    primaryLoaders.unshift(["discover", loadDiscoverContainers]);
  }
  if (includeAdvanced) {
    advancedPrimaryLoaders.push(["audit", loadAudit], ["events", loadEvents], ["metrics", loadMetrics]);
    advancedSecondaryLoaders.push(
      ["jobs", loadJobs],
      ["webhook-config", loadWebhookConfig],
      ["webhook-receipts", loadWebhookReceipts],
      ["push-config", loadPushConfig]
    );
  }

  return { primaryLoaders, advancedPrimaryLoaders, advancedSecondaryLoaders };
}

function getLoadersForSections(sections) {
  const loaders = [];
  if (sections.includes("targets")) {
    loaders.push(["targets", loadTargets]);
  }
  if (showAdvanced && sections.includes("jobs")) {
    loaders.push(["jobs", loadJobs]);
  }
  if (showAdvanced && sections.includes("metrics")) {
    loaders.push(["metrics", loadMetrics]);
  }
  if (showAdvanced && sections.includes("audit")) {
    loaders.push(["audit", loadAudit]);
  }
  if (showAdvanced && sections.includes("events")) {
    loaders.push(["events", loadEvents]);
  }
  if (showAdvanced && sections.includes("receipts")) {
    loaders.push(["receipts", loadWebhookReceipts]);
  }
  return loaders;
}

function handleUnauthorized(err) {
  if (err && err.status === 401) {
    redirectToLogin();
    return true;
  }
  return false;
}

function setAuthState(username) {
  currentUsername = username || "";
  authState.textContent = currentUsername;
  if (mainLayout) {
    mainLayout.classList.toggle("hidden", !currentUsername);
  }
}

function isIOS() {
  return /iphone|ipad|ipod/i.test(window.navigator.userAgent);
}

function isAndroid() {
  return /android/i.test(window.navigator.userAgent);
}

function isStandaloneMode() {
  return window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true;
}

function updateInstallUI() {
  if (!installBanner || !installBtn || !installHelp) return;
  if (isStandaloneMode()) {
    installBanner.classList.add("hidden");
    return;
  }
  installBanner.classList.remove("hidden");
  installBtn.classList.add("hidden");

  if (deferredInstallPrompt) {
    installBtn.classList.remove("hidden");
    installHelp.textContent = t("install.available");
    return;
  }
  if (isIOS()) {
    installHelp.textContent = t("install.ios_help");
    return;
  }
  if (isAndroid()) {
    installHelp.textContent = t("install.android_help");
    return;
  }
  installHelp.textContent = t("install.browser_help");
}

function initInstallPromptHandling() {
  window.addEventListener("beforeinstallprompt", (event) => {
    event.preventDefault();
    deferredInstallPrompt = event;
    updateInstallUI();
  });
  window.addEventListener("appinstalled", () => {
    deferredInstallPrompt = null;
    updateInstallUI();
  });
}

async function triggerInstall() {
  if (!deferredInstallPrompt) {
    updateInstallUI();
    return;
  }
  deferredInstallPrompt.prompt();
  try {
    await deferredInstallPrompt.userChoice;
  } catch (_) {
    // ignore cancellation
  }
  deferredInstallPrompt = null;
  updateInstallUI();
}

async function registerServiceWorker() {
  if (!("serviceWorker" in navigator)) return;
  if (window.location.hostname === "localhost" || window.location.hostname === "127.0.0.1" || window.location.hostname === "::1") {
    try {
      const registrations = await navigator.serviceWorker.getRegistrations();
      await Promise.all(registrations.map((registration) => registration.unregister()));
    } catch (_) {
      // ignore unregister errors in local dev mode
    }
    serviceWorkerRegistration = null;
    return;
  }
  try {
    const assetVersion = encodeURIComponent(String(window.__ASSET_VERSION__ || "").trim());
    const serviceWorkerURL = assetVersion ? `/sw.js?v=${assetVersion}` : "/sw.js";
    serviceWorkerRegistration = await navigator.serviceWorker.register(serviceWorkerURL);
  } catch (err) {
    console.error("service worker register failed:", err);
    serviceWorkerRegistration = null;
  }
}

function setupServiceWorkerMessageHandling() {
  if (!("serviceWorker" in navigator)) return;
  navigator.serviceWorker.addEventListener("message", (event) => {
    const payload = event && event.data ? event.data : null;
    if (!payload || typeof payload !== "object") return;
    if (payload.type === "push-event") {
      if (document.visibilityState !== "visible") {
        const badgeCount = Number(payload.badge_count || 1);
        if (Number.isFinite(badgeCount) && badgeCount > 0) {
          void setAppBadgeCount(badgeCount);
        }
      }
      void reloadAll({ includeAdvanced: showAdvanced });
      return;
    }
    if (payload.type === "push-open") {
      const url = String(payload.url || "/");
      if (url && url !== "/") {
        window.location.assign(url);
      } else {
        void clearAppBadgeOnOpen();
        void reloadAll({ includeAdvanced: showAdvanced });
      }
    }
  });
}

async function postServiceWorkerMessage(message) {
  if (!("serviceWorker" in navigator)) return;
  const controller = navigator.serviceWorker.controller;
  if (controller && typeof controller.postMessage === "function") {
    controller.postMessage(message);
    return;
  }
  const reg = await getServiceWorkerRegistration();
  if (reg && reg.active && typeof reg.active.postMessage === "function") {
    reg.active.postMessage(message);
  }
}

async function clearAppBadgeOnOpen() {
  try {
    if (typeof navigator.clearAppBadge === "function") {
      await navigator.clearAppBadge();
    } else if (typeof navigator.setAppBadge === "function") {
      await navigator.setAppBadge(0);
    }
  } catch (_) {
    // ignore unsupported badging in page context
  }
  try {
    await postServiceWorkerMessage({ type: "clear-badge" });
  } catch (_) {
    // ignore service worker message failures
  }
}

async function setAppBadgeCount(count) {
  const value = Number.isFinite(Number(count)) ? Math.max(0, Math.floor(Number(count))) : 0;
  try {
    if (typeof navigator.setAppBadge === "function") {
      if (value > 0) {
        await navigator.setAppBadge(value);
      } else if (typeof navigator.clearAppBadge === "function") {
        await navigator.clearAppBadge();
      }
    }
  } catch (_) {
    // ignore unsupported badging in page context
  }
  try {
    await postServiceWorkerMessage({ type: "set-badge", count: value });
  } catch (_) {
    // ignore service worker message failures
  }
}

function formatTs(sec) {
  if (!sec) return "-";
  const locale = currentLang === "en" ? "en-US" : "ko-KR";
  return new Date(sec * 1000).toLocaleString(locale);
}

function isCompactViewport() {
  return isNarrowViewport();
}

function summarizeImageRepos(imageRepos) {
  if (!Array.isArray(imageRepos) || imageRepos.length === 0) return "-";
  const first = String(imageRepos[0] || "-");
  if (imageRepos.length === 1) return first;
  return `${first} +${imageRepos.length - 1}`;
}

function compactText(parts) {
  return parts
    .map((part) => String(part || "").trim())
    .filter((part) => part.length > 0)
    .join(" · ");
}

function isCompactItemExpanded(section, key) {
  return String(compactExpandedBySection[section] || "") === String(key);
}

function setCompactExpanded(section, key) {
  const normalized = String(key);
  compactExpandedBySection[section] = isCompactItemExpanded(section, normalized) ? null : normalized;
}

function ensureCompactExpandedKey(section, keys) {
  const current = compactExpandedBySection[section];
  if (current == null) {
    return;
  }
  const valid = new Set((keys || []).map((key) => String(key)));
  if (!valid.has(String(current))) {
    compactExpandedBySection[section] = null;
  }
}

function compactDetailRow(label, value, options = {}) {
  const display = value == null || String(value).trim() === "" ? "-" : String(value);
  const className = options.code ? "compact-details-value code" : "compact-details-value";
  const safe = options.code ? `<code>${display}</code>` : display;
  return `
    <div class="compact-details-row">
      <span class="compact-details-label">${label}</span>
      <span class="${className}">${safe}</span>
    </div>
  `;
}

function statusBadge(status) {
  const key = `status.${status}`;
  let label = t(key);
  if (label === key) label = status;
  return `<span class="status-chip status-${status}">${label}</span>`;
}

function selectedTargetIds() {
  const checkboxes = Array.from(document.querySelectorAll('input[name="target-select"]'));
  if (checkboxes.length > 0) {
    selectedTargetIdSet = new Set();
    checkboxes.forEach((el) => {
      const id = Number(el.value);
      if (el.checked && Number.isInteger(id) && id > 0) {
        selectedTargetIdSet.add(id);
      }
    });
  }

  const validIds = new Set((currentTargets || []).map((target) => Number(target.id)));
  const ids = [];
  selectedTargetIdSet.forEach((id) => {
    if (validIds.has(id)) {
      ids.push(id);
    }
  });
  return ids;
}

function toggleSelectAllTargets() {
  const checkboxes = Array.from(document.querySelectorAll('input[name="target-select"]'));
  if (checkboxes.length === 0) return;
  const allSelected = checkboxes.every((el) => el.checked);
  const next = !allSelected;
  checkboxes.forEach((el) => {
    el.checked = next;
    const id = Number(el.value);
    if (!Number.isInteger(id) || id <= 0) return;
    if (next) {
      selectedTargetIdSet.add(id);
    } else {
      selectedTargetIdSet.delete(id);
    }
  });
}

function selectedDiscoverItem() {
  const idx = Number(discoverItemSelect.value);
  if (!Number.isInteger(idx) || idx < 0 || idx >= discoveredItems.length) {
    return null;
  }
  const option = discoverItemSelect.options[discoverItemSelect.selectedIndex];
  if (option && option.disabled) {
    return null;
  }
  return discoveredItems[idx];
}

function renderDiscoverImages(item) {
  discoverImageSelect.innerHTML = "";
  if (!item) {
    discoverImageSelect.innerHTML = `<option value="">${t("discover.select_first")}</option>`;
    discoverImageSelect.disabled = true;
    if (registerTargetBtn) registerTargetBtn.disabled = true;
    discoverMeta.textContent = "";
    return;
  }
  const repos = Array.isArray(item.image_repo_candidates) ? item.image_repo_candidates : [];
  if (repos.length === 0) {
    discoverImageSelect.innerHTML = `<option value="">${t("discover.no_image")}</option>`;
    discoverImageSelect.disabled = true;
    if (registerTargetBtn) registerTargetBtn.disabled = true;
  } else {
    repos.forEach((repo) => {
      const option = document.createElement("option");
      option.value = repo;
      option.textContent = repo;
      discoverImageSelect.appendChild(option);
    });
    discoverImageSelect.disabled = false;
    if (registerTargetBtn) registerTargetBtn.disabled = false;
  }
  discoverMeta.textContent = t("discover.meta", {
    dir: item.compose_dir,
    file: item.compose_file,
  });
}

function renderDiscoverList(preferredComposeDir = "") {
  const currentSelectedIndex = Number(discoverItemSelect.value);
  const currentSelectedComposeDir =
    Number.isInteger(currentSelectedIndex) &&
    currentSelectedIndex >= 0 &&
    currentSelectedIndex < discoveredItems.length
      ? discoveredItems[currentSelectedIndex].compose_dir
      : "";
  const desiredComposeDir = String(preferredComposeDir || currentSelectedComposeDir || "").trim();

  discoverItemSelect.innerHTML = "";
  if (discoveredItems.length === 0) {
    discoverItemSelect.innerHTML = `<option value="">${t("discover.empty")}</option>`;
    discoverItemSelect.disabled = true;
    renderDiscoverImages(null);
    return;
  }

  const registered = new Set((currentTargets || []).map((target) => target.compose_dir));
  let firstAvailable = -1;
  let preferredAvailable = -1;
  discoveredItems.forEach((item, idx) => {
    const option = document.createElement("option");
    option.value = String(idx);
    const isRegistered = registered.has(item.compose_dir);
    option.disabled = isRegistered;
    option.textContent = isRegistered
      ? `${item.name} (${item.compose_file}) - ${t("discover.already_registered")}`
      : `${item.name} (${item.compose_file})`;
    if (!isRegistered && firstAvailable < 0) {
      firstAvailable = idx;
    }
    if (!isRegistered && desiredComposeDir && item.compose_dir === desiredComposeDir && preferredAvailable < 0) {
      preferredAvailable = idx;
    }
    discoverItemSelect.appendChild(option);
  });

  if (firstAvailable < 0) {
    discoverItemSelect.innerHTML = `<option value="">${t("discover.none_available")}</option>`;
    discoverItemSelect.disabled = true;
    renderDiscoverImages(null);
    discoverMeta.textContent = t("discover.none_available");
    return;
  }

  discoverItemSelect.disabled = false;
  const selectedIndex = preferredAvailable >= 0 ? preferredAvailable : firstAvailable;
  discoverItemSelect.value = String(selectedIndex);
  renderDiscoverImages(discoveredItems[selectedIndex]);
}

async function loadDiscoverContainers() {
  const previousItem = selectedDiscoverItem();
  const previousComposeDir = previousItem ? String(previousItem.compose_dir || "") : "";
  discoverItemSelect.innerHTML = `<option value="">${t("discover.loading")}</option>`;
  try {
    const data = await fetchJSON("/api/containers/discover");
    discoveredItems = data.items || [];
    renderDiscoverList(previousComposeDir);
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("discover.load_failed", { error: err.message }));
      discoveredItems = [];
      renderDiscoverList();
    }
  }
}

function renderTargets(targets) {
  targetsBody.innerHTML = "";
  const compact = isCompactViewport();
  ensureCompactExpandedKey("targets", (targets || []).map((target) => target.id));
  for (const target of targets) {
    const imageRepos = Array.isArray(target.image_repos) && target.image_repos.length > 0
      ? target.image_repos
      : (target.image_repo ? [target.image_repo] : []);
    const imageRepoHTML = imageRepos.length > 0
      ? imageRepos.map((repo) => `<code>${repo}</code>`).join("<br>")
      : "-";
    const isSelected = selectedTargetIdSet.has(Number(target.id));
    const tr = document.createElement("tr");
    if (compact) {
      tr.className = "compact-row";
      tr.dataset.compactSection = "targets";
      tr.dataset.compactKey = String(target.id);
      const repoSummary = summarizeImageRepos(imageRepos);
      const expanded = isCompactItemExpanded("targets", target.id);
      const detailRows = [
        compactDetailRow(t("col.compose_dir"), target.compose_dir, { code: true }),
        compactDetailRow(t("col.image_repo"), imageRepos.join(", "), { code: true }),
        compactDetailRow(t("col.auto"), target.auto_update_enabled ? "ON" : "OFF"),
        compactDetailRow(t("col.enabled"), target.enabled ? "ON" : "OFF"),
        compactDetailRow(t("col.cooldown"), `${target.cooldown_seconds}s`),
        compactDetailRow(t("col.last_success"), formatTs(target.last_success_at)),
      ].join("");
      tr.innerHTML = `
        <td data-label="" class="compact-cell">
          <div class="compact-line">
            <input type="checkbox" name="target-select" value="${target.id}" ${isSelected ? "checked" : ""} />
            <span class="compact-primary">#${target.id} ${target.name}</span>
            <span class="compact-repo" title="${repoSummary}">${repoSummary}</span>
            <label class="compact-flag">A <input type="checkbox" data-action="auto-toggle" data-id="${target.id}" ${target.auto_update_enabled ? "checked" : ""} /></label>
            <label class="compact-flag">E <input type="checkbox" data-action="enabled-toggle" data-id="${target.id}" ${target.enabled ? "checked" : ""} /></label>
            <span class="compact-secondary">CD ${target.cooldown_seconds}s</span>
            <button data-action="delete-target" data-id="${target.id}" class="danger compact-btn">${t("target.delete")}</button>
            <button data-action="toggle-details" data-section="targets" data-key="${target.id}" class="secondary compact-btn compact-toggle" aria-expanded="${expanded ? "true" : "false"}">${expanded ? t("btn.details_collapse") : t("btn.details_expand")}</button>
          </div>
          ${expanded ? `<div class="compact-details">${detailRows}</div>` : ""}
        </td>
      `;
      targetsBody.appendChild(tr);
      continue;
    }
    tr.innerHTML = `
      <td data-label="${t("col.select")}"><input type="checkbox" name="target-select" value="${target.id}" ${isSelected ? "checked" : ""} /></td>
      <td data-label="${t("col.id")}">${target.id}</td>
      <td data-label="${t("col.name")}">${target.name}</td>
      <td data-label="${t("col.compose_dir")}"><code>${target.compose_dir}</code></td>
      <td data-label="${t("col.image_repo")}">${imageRepoHTML}</td>
      <td data-label="${t("col.auto")}"><input type="checkbox" data-action="auto-toggle" data-id="${target.id}" ${target.auto_update_enabled ? "checked" : ""} /></td>
      <td data-label="${t("col.enabled")}"><input type="checkbox" data-action="enabled-toggle" data-id="${target.id}" ${target.enabled ? "checked" : ""} /></td>
      <td data-label="${t("col.cooldown")}">${target.cooldown_seconds}</td>
      <td data-label="${t("col.last_success")}">${formatTs(target.last_success_at)}</td>
      <td data-label="${t("col.action")}"><button data-action="delete-target" data-id="${target.id}" class="danger">${t("target.delete")}</button></td>
    `;
    targetsBody.appendChild(tr);
  }
}

function renderAudit(summaries) {
  auditBody.innerHTML = "";
  const compact = isCompactViewport();
  ensureCompactExpandedKey("audit", (summaries || []).map((item) => item.target_id));
  if (!summaries || summaries.length === 0) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td colspan="9" data-label="" class="${compact ? "compact-cell" : ""}">${t("audit.empty")}</td>`;
    auditBody.appendChild(tr);
    return;
  }
  for (const item of summaries) {
    const tr = document.createElement("tr");
    if (compact) {
      tr.className = "compact-row";
      tr.dataset.compactSection = "audit";
      tr.dataset.compactKey = String(item.target_id);
      const expanded = isCompactItemExpanded("audit", item.target_id);
      const summary = compactText([
        `S ${item.success_runs}/${item.total_runs}`,
        `F ${item.failed_runs}`,
        `B ${item.blocked_runs}`,
        `AVG ${Number(item.avg_duration_sec || 0).toFixed(1)}s`,
      ]);
      const detailRows = [
        compactDetailRow(t("col.image_repo"), item.image_repo || "-", { code: true }),
        compactDetailRow(t("col.total_runs"), item.total_runs),
        compactDetailRow(t("col.success_runs"), item.success_runs),
        compactDetailRow(t("col.failed_runs"), item.failed_runs),
        compactDetailRow(t("col.blocked_runs"), item.blocked_runs),
        compactDetailRow(t("col.avg_duration"), `${Number(item.avg_duration_sec || 0).toFixed(2)}s`),
        compactDetailRow(t("col.last_run"), formatTs(item.last_run_at)),
      ].join("");
      tr.innerHTML = `
        <td data-label="" class="compact-cell">
          <div class="compact-line">
            <span class="compact-primary">#${item.target_id} ${item.name}</span>
            <span class="compact-repo" title="${item.image_repo || "-"}">${item.image_repo || "-"}</span>
            <span class="compact-secondary">${summary}</span>
            <button data-action="toggle-details" data-section="audit" data-key="${item.target_id}" class="secondary compact-btn compact-toggle" aria-expanded="${expanded ? "true" : "false"}">${expanded ? t("btn.details_collapse") : t("btn.details_expand")}</button>
          </div>
          ${expanded ? `<div class="compact-details">${detailRows}</div>` : ""}
        </td>
      `;
      auditBody.appendChild(tr);
      continue;
    }
    tr.innerHTML = `
      <td data-label="${t("col.id")}">${item.target_id}</td>
      <td data-label="${t("col.name")}">${item.name}</td>
      <td data-label="${t("col.image_repo")}"><code>${item.image_repo}</code></td>
      <td data-label="${t("col.total_runs")}">${item.total_runs}</td>
      <td data-label="${t("col.success_runs")}">${item.success_runs}</td>
      <td data-label="${t("col.failed_runs")}">${item.failed_runs}</td>
      <td data-label="${t("col.blocked_runs")}">${item.blocked_runs}</td>
      <td data-label="${t("col.avg_duration")}">${Number(item.avg_duration_sec || 0).toFixed(2)}</td>
      <td data-label="${t("col.last_run")}">${formatTs(item.last_run_at)}</td>
    `;
    auditBody.appendChild(tr);
  }
}

function renderEvents(events) {
  eventsBody.innerHTML = "";
  const compact = isCompactViewport();
  ensureCompactExpandedKey("events", (events || []).map((event) => event.id));
  for (const event of events) {
    const tr = document.createElement("tr");
    if (compact) {
      tr.className = "compact-row";
      tr.dataset.compactSection = "events";
      tr.dataset.compactKey = String(event.id);
      const expanded = isCompactItemExpanded("events", event.id);
      const repoTag = event.tag ? `${event.image_repo}:${event.tag}` : event.image_repo;
      const matched = (event.matched_target_ids || []).join(",") || "-";
      const queued = (event.queued_target_ids || []).join(",") || "-";
      const detailRows = [
        compactDetailRow(t("col.image_repo"), event.image_repo || "-", { code: true }),
        compactDetailRow(t("col.tag"), event.tag || "-"),
        compactDetailRow(t("col.matched"), (event.matched_target_ids || []).join(", ") || "-"),
        compactDetailRow(t("col.queued"), (event.queued_target_ids || []).join(", ") || "-"),
        compactDetailRow(t("col.received_at"), formatTs(event.received_at)),
      ].join("");
      tr.innerHTML = `
        <td data-label="" class="compact-cell">
          <div class="compact-line">
            <span class="compact-primary">#${event.id}</span>
            <span class="compact-repo" title="${repoTag || "-"}">${repoTag || "-"}</span>
            <span class="compact-secondary">M ${matched}</span>
            <span class="compact-secondary">Q ${queued}</span>
            <button data-action="toggle-details" data-section="events" data-key="${event.id}" class="secondary compact-btn compact-toggle" aria-expanded="${expanded ? "true" : "false"}">${expanded ? t("btn.details_collapse") : t("btn.details_expand")}</button>
          </div>
          ${expanded ? `<div class="compact-details">${detailRows}</div>` : ""}
        </td>
      `;
      eventsBody.appendChild(tr);
      continue;
    }
    tr.innerHTML = `
      <td data-label="${t("col.id")}">${event.id}</td>
      <td data-label="${t("col.image_repo")}"><code>${event.image_repo}</code></td>
      <td data-label="${t("col.tag")}">${event.tag || "-"}</td>
      <td data-label="${t("col.matched")}">${(event.matched_target_ids || []).join(", ") || "-"}</td>
      <td data-label="${t("col.queued")}">${(event.queued_target_ids || []).join(", ") || "-"}</td>
      <td data-label="${t("col.received_at")}">${formatTs(event.received_at)}</td>
    `;
    eventsBody.appendChild(tr);
  }
}

function renderReceipts(receipts) {
  if (!receiptsBody) return;
  receiptsBody.innerHTML = "";
  const compact = isCompactViewport();
  ensureCompactExpandedKey("receipts", (receipts || []).map((item) => item.id));
  for (const item of receipts || []) {
    const tr = document.createElement("tr");
    if (compact) {
      tr.className = "compact-row";
      tr.dataset.compactSection = "receipts";
      tr.dataset.compactKey = String(item.id);
      const expanded = isCompactItemExpanded("receipts", item.id);
      const detailRows = [
        compactDetailRow(t("col.status_code"), item.status_code),
        compactDetailRow(t("col.reason_code"), item.reason_code || "-", { code: true }),
        compactDetailRow(t("col.job_id"), item.queued_job_id || "-"),
        compactDetailRow(t("col.received_at"), formatTs(item.received_at)),
      ].join("");
      tr.innerHTML = `
        <td data-label="" class="compact-cell">
          <div class="compact-line">
            <span class="compact-primary">#${item.id}</span>
            <span class="compact-secondary">${item.status_code}</span>
            <span class="compact-repo" title="${item.reason_code || "-"}">${item.reason_code || "-"}</span>
            <span class="compact-secondary">Job ${item.queued_job_id || "-"}</span>
            <button data-action="toggle-details" data-section="receipts" data-key="${item.id}" class="secondary compact-btn compact-toggle" aria-expanded="${expanded ? "true" : "false"}">${expanded ? t("btn.details_collapse") : t("btn.details_expand")}</button>
          </div>
          ${expanded ? `<div class="compact-details">${detailRows}</div>` : ""}
        </td>
      `;
      receiptsBody.appendChild(tr);
      continue;
    }
    tr.innerHTML = `
      <td data-label="${t("col.id")}">${item.id}</td>
      <td data-label="${t("col.status_code")}">${item.status_code}</td>
      <td data-label="${t("col.reason_code")}"><code>${item.reason_code || "-"}</code></td>
      <td data-label="${t("col.job_id")}">${item.queued_job_id || "-"}</td>
      <td data-label="${t("col.received_at")}">${formatTs(item.received_at)}</td>
    `;
    receiptsBody.appendChild(tr);
  }
}

function renderJobs(jobs, append = false) {
  if (!append) jobsBody.innerHTML = "";
  const compact = isCompactViewport();
  const keySource = append ? currentJobs : jobs;
  ensureCompactExpandedKey("jobs", (keySource || []).map((job) => job.id));
  for (const job of jobs) {
    const tr = document.createElement("tr");
    tr.dataset.jobId = job.id;
    if (compact) {
      tr.className = "compact-row";
      tr.dataset.compactSection = "jobs";
      tr.dataset.compactKey = String(job.id);
      const expanded = isCompactItemExpanded("jobs", job.id);
      const targets = (job.target_ids || []).join(",") || "-";
      const duration = job.duration_sec || job.duration_sec === 0 ? `${Number(job.duration_sec).toFixed(1)}s` : "-";
      const baseSummary = compactText([job.type, job.trigger, `T ${targets}`, duration]);
      const errorHint = job.error_message ? `ERR ${String(job.error_message).split("\n")[0]}` : "";
      const detailRows = [
        compactDetailRow(t("col.type"), job.type),
        compactDetailRow(t("col.trigger"), job.trigger),
        compactDetailRow(t("col.status"), job.status),
        compactDetailRow(t("col.targets"), (job.target_ids || []).join(", ") || "-"),
        compactDetailRow(t("col.duration"), duration),
        compactDetailRow(t("col.error"), job.error_message || "-"),
        compactDetailRow(t("col.created"), formatTs(job.created_at)),
      ].join("");
      tr.innerHTML = `
        <td data-label="" class="compact-cell">
          <div class="compact-line">
            <span class="compact-primary">#${job.id}</span>
            <span class="compact-status">${statusBadge(job.status)}</span>
            <span class="compact-secondary">${baseSummary}</span>
            ${errorHint ? `<span class="compact-repo" title="${errorHint}">${errorHint}</span>` : ""}
            <button data-action="open-log" data-id="${job.id}" class="secondary compact-btn">${t("btn.view_logs")}</button>
            <button data-action="delete-job" data-id="${job.id}" class="danger compact-btn">${t("target.delete")}</button>
            <button data-action="toggle-details" data-section="jobs" data-key="${job.id}" class="secondary compact-btn compact-toggle" aria-expanded="${expanded ? "true" : "false"}">${expanded ? t("btn.details_collapse") : t("btn.details_expand")}</button>
          </div>
          ${expanded ? `<div class="compact-details">${detailRows}</div>` : ""}
        </td>
      `;
      jobsBody.appendChild(tr);
      continue;
    }
    tr.style.cursor = "pointer";
    tr.innerHTML = `
      <td data-label="${t("col.id")}">${job.id}</td>
      <td data-label="${t("col.type")}">${job.type}</td>
      <td data-label="${t("col.trigger")}">${job.trigger}</td>
      <td data-label="${t("col.status")}">${statusBadge(job.status)}</td>
      <td data-label="${t("col.targets")}">${(job.target_ids || []).join(", ") || "-"}</td>
      <td data-label="${t("col.duration")}">${job.duration_sec || job.duration_sec === 0 ? Number(job.duration_sec).toFixed(1) : "-"}</td>
      <td data-label="${t("col.error")}">${job.error_message || "-"}</td>
      <td data-label="${t("col.created")}">${formatTs(job.created_at)}</td>
      <td data-label="${t("col.action")}"><button data-action="delete-job" data-id="${job.id}" class="danger">${t("target.delete")}</button></td>
    `;
    jobsBody.appendChild(tr);
  }
}

async function loadTargets() {
  const data = await fetchJSON("/api/targets");
  currentTargets = data.targets || [];
  const validIds = new Set(currentTargets.map((target) => Number(target.id)));
  selectedTargetIdSet.forEach((id) => {
    if (!validIds.has(id)) {
      selectedTargetIdSet.delete(id);
    }
  });
  renderTargets(currentTargets);
  renderDiscoverList();
}

async function loadAudit() {
  const data = await fetchJSON("/api/audit/targets");
  currentAuditSummaries = data.summaries || [];
  renderAudit(currentAuditSummaries);
}

async function loadEvents() {
  const data = await fetchJSON("/api/diun/events");
  currentDiunEvents = data.events || [];
  renderEvents(currentDiunEvents);
}

async function loadWebhookReceipts() {
  if (!receiptsBody) return;
  const data = await fetchJSON("/api/diun/receipts?limit=30");
  currentWebhookReceipts = data.receipts || [];
  renderReceipts(currentWebhookReceipts);
}

async function loadJobs() {
  jobsNextCursor = null;
  const data = await fetchJSON("/api/jobs?limit=20");
  currentJobs = data.jobs || [];
  renderJobs(currentJobs, false);
  jobsNextCursor = data.next_cursor || null;
  jobsMoreBtn.disabled = !jobsNextCursor;
}

async function loadMoreJobs() {
  if (!jobsNextCursor) {
    jobsMoreBtn.disabled = true;
    return;
  }
  const data = await fetchJSON(`/api/jobs?limit=20&cursor=${jobsNextCursor}`);
  const moreJobs = data.jobs || [];
  currentJobs = currentJobs.concat(moreJobs);
  renderJobs(moreJobs, true);
  jobsNextCursor = data.next_cursor || null;
  jobsMoreBtn.disabled = !jobsNextCursor;
}

function rerenderCompactSection(section) {
  if (section === "targets") {
    renderTargets(currentTargets);
    return;
  }
  if (section === "audit") {
    renderAudit(currentAuditSummaries);
    return;
  }
  if (section === "events") {
    renderEvents(currentDiunEvents);
    return;
  }
  if (section === "receipts") {
    renderReceipts(currentWebhookReceipts);
    return;
  }
  if (section === "jobs") {
    renderJobs(currentJobs, false);
    if (jobsMoreBtn) {
      jobsMoreBtn.disabled = !jobsNextCursor;
    }
  }
}

function toggleCompactDetails(section, key) {
  if (!isCompactViewport()) {
    return;
  }
  if (!Object.prototype.hasOwnProperty.call(compactExpandedBySection, section)) {
    return;
  }
  setCompactExpanded(section, key);
  rerenderCompactSection(section);
}

function rerenderResponsiveTablesIfNeeded() {
  const compact = isCompactViewport();
  if (compact === lastCompactViewport) {
    return;
  }
  lastCompactViewport = compact;
  renderTargets(currentTargets);
  renderAudit(currentAuditSummaries);
  renderEvents(currentDiunEvents);
  renderReceipts(currentWebhookReceipts);
  renderJobs(currentJobs, false);
  if (jobsMoreBtn) {
    jobsMoreBtn.disabled = !jobsNextCursor;
  }
}

async function loadMetrics() {
  const m = await fetchJSON("/api/metrics");
  setElementText(metricFailed, String(m.failed_jobs_last_24h ?? 0));
  setElementText(metricAvg, Number(m.avg_duration_sec_24h || 0).toFixed(2));
  setElementText(metricWebhook, `${(Number(m.webhook_failure_rate_24h || 0) * 100).toFixed(1)}% (${Number(m.webhook_total_24h || 0)})`);
  setElementText(metricWebhookFailed, String(m.webhook_failures_24h || 0));
  setElementText(metricLoginFailed, String(m.login_failures_24h || 0));
  setElementText(metricLoginLimited, String(m.login_rate_limited_24h || 0));
  setElementText(metricDashboardStream, `${Number(m.dashboard_stream_active || 0)} / ${Number(m.dashboard_stream_rejected_total || 0)}`);
  setElementText(metricPushDelivery, `${Number(m.push_sent_24h || 0)} / ${Number(m.push_failed_24h || 0)}`);
}

function setElementText(el, value) {
  if (!el) return;
  el.textContent = value;
}

async function loadWebhookConfig() {
  if (!webhookConfig) return;
  webhookConfig.textContent = t("webhook.config_text", {
    path: "/api/diun/webhook",
    header: "X-DIUN-SECRET",
    secret: t("webhook.secret_hidden"),
  });
}

function setPushStatus(message) {
  if (!pushStatus) return;
  pushStatus.textContent = message;
}

function urlBase64ToUint8Array(base64String) {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; i += 1) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

async function getServiceWorkerRegistration() {
  if (!("serviceWorker" in navigator)) return null;
  const host = window.location.hostname;
  if (host === "localhost" || host === "127.0.0.1" || host === "::1") {
    return null;
  }
  if (serviceWorkerRegistration) return serviceWorkerRegistration;
  const readyRegistration = await serviceWorkerReadyWithTimeout(SERVICE_WORKER_READY_TIMEOUT_MS);
  if (readyRegistration) {
    serviceWorkerRegistration = readyRegistration;
    return readyRegistration;
  }
  return lookupExistingServiceWorkerRegistration();
}

async function getCurrentPushEndpoint() {
  const reg = await getServiceWorkerRegistration();
  if (!reg || !reg.pushManager) return "";
  const sub = await reg.pushManager.getSubscription();
  return sub && sub.endpoint ? String(sub.endpoint) : "";
}

async function loadPushConfig() {
  if (!pushStatus) return;
  if (!("Notification" in window) || !("serviceWorker" in navigator)) {
    setPushStatus(t("push.not_supported"));
    if (pushEnableBtn) pushEnableBtn.disabled = true;
    if (pushDisableBtn) pushDisableBtn.disabled = true;
    if (pushTestBtn) pushTestBtn.disabled = true;
    return;
  }
  try {
    const endpoint = await getCurrentPushEndpoint();
    const query = endpoint ? `?endpoint=${encodeURIComponent(endpoint)}` : "";
    const cfg = await fetchJSON(`/api/push/config${query}`);
    if (!cfg.enabled) {
      setPushStatus(t("push.disabled_server"));
      if (pushEnableBtn) pushEnableBtn.disabled = true;
      if (pushDisableBtn) pushDisableBtn.disabled = true;
      if (pushTestBtn) pushTestBtn.disabled = true;
      return;
    }
    if (Notification.permission === "denied") {
      setPushStatus(t("push.permission_denied"));
    } else if (cfg.subscribed) {
      setPushStatus(t("push.ready_subscribed"));
    } else if (cfg.has_any_subscriptions) {
      setPushStatus(t("push.ready_other_device"));
    } else {
      setPushStatus(t("push.ready_not_subscribed"));
    }
    if (pushEnableBtn) pushEnableBtn.disabled = false;
    if (pushDisableBtn) pushDisableBtn.disabled = !cfg.subscribed && !cfg.has_any_subscriptions;
    if (pushTestBtn) pushTestBtn.disabled = !cfg.has_any_subscriptions;
  } catch (err) {
    if (!handleUnauthorized(err)) {
      setPushStatus(t("push.enable_failed", { error: err.message }));
    }
  }
}

async function enablePushNotifications() {
  if (!("Notification" in window) || !("serviceWorker" in navigator)) {
    alert(t("push.not_supported"));
    return;
  }
  try {
    const cfg = await fetchJSON("/api/push/config");
    if (!cfg.enabled || !cfg.vapid_public_key) {
      setPushStatus(t("push.disabled_server"));
      return;
    }
    const permission = await Notification.requestPermission();
    if (permission !== "granted") {
      setPushStatus(t("push.permission_denied"));
      return;
    }
    const reg = await getServiceWorkerRegistration();
    if (!reg || !reg.pushManager) {
      throw new Error("service worker unavailable");
    }
    let sub = await reg.pushManager.getSubscription();
    if (!sub) {
      sub = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(cfg.vapid_public_key),
      });
    }
    await fetchJSON("/api/push/subscriptions", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify(sub),
    });
    setPushStatus(t("push.enabled"));
    await loadPushConfig();
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("push.enable_failed", { error: err.message }));
    }
  }
}

async function disablePushNotifications() {
  try {
    const reg = await getServiceWorkerRegistration();
    let endpoint = "";
    if (reg && reg.pushManager) {
      const sub = await reg.pushManager.getSubscription();
      if (sub) {
        endpoint = sub.endpoint || "";
        await sub.unsubscribe();
      }
    }
    const encoded = endpoint ? `?endpoint=${encodeURIComponent(endpoint)}` : "";
    const init = {
      method: "DELETE",
      headers: { "X-Requested-With": "XMLHttpRequest" },
    };
    if (endpoint) {
      init.headers["Content-Type"] = "application/json";
      init.body = JSON.stringify({ endpoint });
    }
    await fetchJSON(`/api/push/subscriptions${encoded}`, init);
    setPushStatus(t("push.disabled"));
    await loadPushConfig();
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("push.disable_failed", { error: err.message }));
    }
  }
}

async function sendPushTest() {
  try {
    const res = await fetchJSON("/api/push/test", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify({}),
    });
    const sent = Number(res.sent_count || 0);
    const failed = Number(res.failed_count || 0);
    const lastError = String(res.last_error || "").trim();
    if (failed > 0 && lastError) {
      alert(t("push.test_sent_with_error", { sent, failed, error: lastError }));
    } else {
      alert(t("push.test_sent", { sent, failed }));
    }
    await loadPushConfig();
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("push.test_failed", { error: err.message }));
    }
  }
}

async function reloadAll(options = {}) {
  const { primaryLoaders, advancedPrimaryLoaders, advancedSecondaryLoaders } = getLoadersForReload(options);
  try {
    await runLoaderBatch(primaryLoaders, "reload primary");
    if (advancedPrimaryLoaders.length > 0) {
      await runLoaderBatch(advancedPrimaryLoaders, "reload advanced-primary");
    }
    if (advancedSecondaryLoaders.length > 0) {
      await runLoaderBatch(advancedSecondaryLoaders, "reload advanced-secondary");
    }
  } catch (err) {
    if (!handleUnauthorized(err)) {
      console.error("reload failed", err);
    }
  }
}

async function reloadFromPatchSections(sections) {
  const uniqueSections = Array.isArray(sections)
    ? Array.from(new Set(sections.map((item) => String(item || "").trim()).filter(Boolean)))
    : [];
  if (uniqueSections.length === 0) {
    await reloadAll({ includeAdvanced: showAdvanced });
    return;
  }

  const loaders = getLoadersForSections(uniqueSections);
  if (loaders.length === 0) {
    if (uniqueSections.includes("targets")) {
      await loadTargets();
    }
    return;
  }
  await runLoaderBatch(loaders, "reload patch");
}

function startFallbackPolling() {
  if (fallbackPollingTimer) return;
  fallbackPollingTimer = setInterval(() => {
    void reloadAll({ includeAdvanced: showAdvanced });
  }, FALLBACK_POLL_INTERVAL_MS);
}

function stopFallbackPolling() {
  if (!fallbackPollingTimer) return;
  clearInterval(fallbackPollingTimer);
  fallbackPollingTimer = null;
}

function closeDashboardStream() {
  if (dashboardSource) {
    dashboardSource.close();
    dashboardSource = null;
  }
  dashboardConnected = false;
}

function scheduleDashboardReconnect() {
  if (dashboardReconnectTimer) return;
  const idx = Math.min(dashboardReconnectAttempt, DASHBOARD_RECONNECT_DELAYS.length - 1);
  const delay = DASHBOARD_RECONNECT_DELAYS[idx];
  dashboardReconnectAttempt += 1;
  dashboardReconnectTimer = window.setTimeout(() => {
    dashboardReconnectTimer = null;
    connectDashboardStream();
  }, delay);
}

function connectDashboardStream() {
  if (dashboardSource) return;
  dashboardSource = new EventSource("/api/stream/dashboard");
  dashboardSource.addEventListener("patch", (event) => {
    try {
      const payload = JSON.parse(event.data);
      const sections = Array.isArray(payload.sections) ? payload.sections : [];
      void reloadFromPatchSections(sections);
    } catch (err) {
      console.error("dashboard patch parse failed", err);
    }
  });
  dashboardSource.onopen = () => {
    dashboardConnected = true;
    dashboardReconnectAttempt = 0;
    if (dashboardReconnectTimer) {
      clearTimeout(dashboardReconnectTimer);
      dashboardReconnectTimer = null;
    }
    stopFallbackPolling();
  };
  dashboardSource.onerror = () => {
    closeDashboardStream();
    startFallbackPolling();
    scheduleDashboardReconnect();
  };
}

async function createUpdateJob() {
  const ids = selectedTargetIds();
  if (ids.length === 0) {
    alert(t("target.select_required"));
    return;
  }
  try {
    const data = await fetchJSON("/api/jobs/update", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify({ target_ids: ids }),
    });
    const skippedCount = Array.isArray(data.skipped_target_ids) ? data.skipped_target_ids.length : 0;
    if (skippedCount > 0) {
      alert(t("job.update_queued_partial", { jobId: data.job_id, count: skippedCount }));
    } else {
      alert(t("job.update_queued", { jobId: data.job_id }));
    }
    await reloadAll();
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("job.failed", { error: err.message }));
    }
  }
}

async function createPruneJob() {
  if (!confirm(t("prune.confirm"))) return;
  try {
    const data = await fetchJSON("/api/jobs/prune", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify({ confirm: true }),
    });
    alert(t("job.prune_queued", { jobId: data.job_id }));
    await reloadAll();
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("job.failed", { error: err.message }));
    }
  }
}

async function deleteAllJobs() {
  if (!confirm(t("job.delete_all_confirm"))) return;
  try {
    const result = await fetchJSON("/api/jobs/delete-all", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify({ confirm: true }),
    });
    const count = Number(result.deleted_count || 0);
    alert(t("job.delete_all_done", { count }));
    if (streamSource) {
      streamSource.close();
      streamSource = null;
      streamMeta.textContent = t("stream.click_job");
      logBox.textContent = "";
    }
    await reloadAll({ includeAdvanced: showAdvanced });
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("job.delete_all_failed", { error: err.message }));
    }
  }
}

async function deleteJob(jobId) {
  if (!confirm(t("job.delete_confirm", { jobId }))) return;
  try {
    await fetchJSON(`/api/jobs/${jobId}`, {
      method: "DELETE",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify({}),
    });
    await reloadAll({ includeAdvanced: showAdvanced });
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("job.delete_failed", { error: err.message }));
    }
  }
}

async function updateTarget(id, payload) {
  try {
    await fetchJSON(`/api/targets/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify(payload),
    });
    if (showAdvanced) {
      await Promise.all([loadTargets(), loadAudit()]);
    } else {
      await loadTargets();
    }
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("target.update_failed", { error: err.message }));
    }
  }
}

async function deleteTarget(id) {
  if (!confirm(t("target.delete_confirm", { id }))) return;
  try {
    await fetchJSON(`/api/targets/${id}`, {
      method: "DELETE",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify({}),
    });
    if (showAdvanced) {
      await Promise.all([loadTargets(), loadAudit(), loadDiscoverContainers()]);
    } else {
      await Promise.all([loadTargets(), loadDiscoverContainers()]);
    }
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("target.delete_failed", { error: err.message }));
    }
  }
}

async function registerSelectedTarget(event) {
  event.preventDefault();
  const item = selectedDiscoverItem();
  if (!item) {
    alert(t("discover.select_required"));
    return;
  }
  const imageRepo = String(discoverImageSelect.value || "").trim();
  if (!imageRepo) {
    alert(t("discover.image_required"));
    return;
  }
  const form = new FormData(addTargetForm);
  const payload = {
    name: item.name,
    compose_dir: item.compose_dir,
    compose_file: item.compose_file,
    image_repo: imageRepo,
  };
  const allRepos = Array.isArray(item.image_repo_candidates)
    ? item.image_repo_candidates.map((repo) => String(repo || "").trim()).filter(Boolean)
    : [];
  if (!allRepos.includes(imageRepo)) {
    allRepos.unshift(imageRepo);
  } else {
    const deduped = [imageRepo, ...allRepos.filter((repo) => repo !== imageRepo)];
    allRepos.splice(0, allRepos.length, ...deduped);
  }
  payload.image_repos = allRepos;
  const cooldownRaw = String(form.get("cooldown_seconds") || "").trim();
  if (cooldownRaw) {
    payload.cooldown_seconds = Number(cooldownRaw);
  }
  try {
    await fetchJSON("/api/targets", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify(payload),
    });
    addTargetForm.reset();
    await reloadAll({ includeDiscover: true });
  } catch (err) {
    if (!handleUnauthorized(err)) {
      alert(t("target.register_failed", { error: err.message }));
    }
  }
}

function openLogStream(jobId) {
  if (streamSource) {
    streamSource.close();
    streamSource = null;
  }
  logBox.textContent = "";
  streamMeta.textContent = t("stream.streaming", { jobId });

  streamSource = new EventSource(`/api/jobs/${jobId}/stream`);
  streamSource.addEventListener("log", (event) => {
    try {
      const payload = JSON.parse(event.data);
      logBox.textContent += `${payload.line}\n`;
      logBox.scrollTop = logBox.scrollHeight;
    } catch (_) {
      // ignore malformed event
    }
  });
  streamSource.addEventListener("done", async (event) => {
    try {
      const payload = JSON.parse(event.data);
      streamMeta.textContent = t("stream.done_status", { jobId, status: payload.status });
    } catch (_) {
      streamMeta.textContent = t("stream.done", { jobId });
    }
    streamSource.close();
    streamSource = null;
    await reloadAll();
  });
  streamSource.onerror = () => {
    streamMeta.textContent = t("stream.ended", { jobId });
    if (streamSource) {
      streamSource.close();
      streamSource = null;
    }
  };
}

async function downloadJobsCSV() {
  try {
    const response = await fetch("/api/jobs/export.csv?limit=1000", { credentials: "same-origin" });
    if (!response.ok) {
      if (response.status === 401) {
        redirectToLogin();
        return;
      }
      const raw = await response.text();
      throw new Error(raw || `${response.status} ${response.statusText}`);
    }
    const blob = await response.blob();
    let filename = "jobs.csv";
    const disposition = response.headers.get("Content-Disposition") || "";
    const match = disposition.match(/filename="?([^"]+)"?/i);
    if (match && match[1]) filename = match[1];

    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
  } catch (err) {
    alert(t("csv.download_failed", { error: err.message }));
  }
}

async function logout() {
  closeDashboardStream();
  stopFallbackPolling();
  if (dashboardReconnectTimer) {
    clearTimeout(dashboardReconnectTimer);
    dashboardReconnectTimer = null;
  }
  try {
    await fetchJSON("/api/auth/logout", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Requested-With": "XMLHttpRequest" },
      body: JSON.stringify({}),
    });
  } catch (_) {
    // ignore logout failure
  }
  redirectToLogin();
}

function setPullRefreshVisibility(visible) {
  if (!pullRefreshIndicator) return;
  pullRefreshIndicator.classList.toggle("hidden", !visible);
  pullRefreshIndicator.setAttribute("aria-hidden", visible ? "false" : "true");
}

function resetPullRefreshIndicator() {
  if (!pullRefreshIndicator || !pullRefreshProgress) return;
  pullRefreshIndicator.classList.remove("is-active", "is-ready", "is-refreshing");
  pullRefreshIndicator.style.setProperty("--pull-progress", "0");
  pullRefreshIndicator.style.setProperty("--pull-offset", "0px");
  pullRefreshIndicator.style.setProperty("--pull-scale", "0.88");
  pullRefreshIndicator.style.setProperty("--pull-opacity", "0");
  pullRefreshProgress.style.strokeDasharray = `${PULL_REFRESH_RING_CIRCUMFERENCE}`;
  pullRefreshProgress.style.strokeDashoffset = `${PULL_REFRESH_RING_CIRCUMFERENCE}`;
  setPullRefreshVisibility(false);
}

function setPullRefreshProgress(distance) {
  if (!pullRefreshIndicator || !pullRefreshProgress) return;
  const rawProgress = distance / PULL_REFRESH_THRESHOLD_PX;
  const progress = Math.max(0, Math.min(rawProgress, 1));
  if (progress <= 0 && !pullRefreshing) {
    resetPullRefreshIndicator();
    return;
  }

  const easedProgress = 1 - Math.pow(1 - progress, 2);
  const offsetPx = Math.min(34, Math.max(8, Math.round(distance * 0.34)));
  const scale = Math.min(1, 0.88 + easedProgress * 0.12);
  const opacity = Math.min(1, 0.45 + easedProgress * 0.55);

  setPullRefreshVisibility(true);
  pullRefreshIndicator.classList.toggle("is-active", !pullRefreshing);
  pullRefreshIndicator.classList.toggle("is-ready", !pullRefreshing && progress >= 1);
  pullRefreshIndicator.style.setProperty("--pull-progress", progress.toFixed(3));
  pullRefreshIndicator.style.setProperty("--pull-offset", `${offsetPx}px`);
  pullRefreshIndicator.style.setProperty("--pull-scale", scale.toFixed(3));
  pullRefreshIndicator.style.setProperty("--pull-opacity", opacity.toFixed(3));
  pullRefreshProgress.style.strokeDasharray = `${PULL_REFRESH_RING_CIRCUMFERENCE}`;
  pullRefreshProgress.style.strokeDashoffset = `${(PULL_REFRESH_RING_CIRCUMFERENCE * (1 - easedProgress)).toFixed(4)}`;
}

async function triggerPullRefresh() {
  const now = Date.now();
  if (pullRefreshing) return;
  if (now - lastPullRefreshAt < PULL_REFRESH_MIN_INTERVAL_MS) {
    resetPullRefreshIndicator();
    return;
  }
  pullRefreshing = true;
  lastPullRefreshAt = now;
  if (pullRefreshIndicator) {
    setPullRefreshVisibility(true);
    pullRefreshIndicator.classList.remove("is-active", "is-ready");
    pullRefreshIndicator.classList.add("is-refreshing");
    pullRefreshIndicator.style.setProperty("--pull-progress", "1");
    pullRefreshIndicator.style.setProperty("--pull-offset", "18px");
    pullRefreshIndicator.style.setProperty("--pull-scale", "1");
    pullRefreshIndicator.style.setProperty("--pull-opacity", "1");
  }
  if (pullRefreshProgress) {
    pullRefreshProgress.style.strokeDasharray = `${(PULL_REFRESH_RING_CIRCUMFERENCE * 0.72).toFixed(4)} ${PULL_REFRESH_RING_CIRCUMFERENCE}`;
    pullRefreshProgress.style.strokeDashoffset = `${(PULL_REFRESH_RING_CIRCUMFERENCE * 0.15).toFixed(4)}`;
  }
  const startedAt = Date.now();
  try {
    await reloadAll({ includeDiscover: true, includeAdvanced: showAdvanced });
  } finally {
    const remaining = PULL_REFRESH_MIN_SPINNER_MS - (Date.now() - startedAt);
    if (remaining > 0) {
      await new Promise((resolve) => window.setTimeout(resolve, remaining));
    }
    pullRefreshing = false;
    resetPullRefreshIndicator();
  }
}

function canStartPullRefresh(target) {
  if (pullRefreshing) return false;
  if (getScrollTop() > 0) return false;
  const elementTarget = target instanceof Element ? target : target && target.parentElement ? target.parentElement : null;
  if (elementTarget && elementTarget.closest("input,select,textarea,button,label,[data-no-pull-refresh]")) {
    return false;
  }
  return true;
}

function beginPullRefreshTracking(target, clientX, clientY, pointerId = null) {
  if (!canStartPullRefresh(target)) return false;
  touchTracking = true;
  touchStartX = clientX;
  touchStartY = clientY;
  touchPullDistance = 0;
  touchPointerId = pointerId;
  setPullRefreshProgress(0);
  return true;
}

function getScrollTop() {
  const scroller = document.scrollingElement || document.documentElement || document.body;
  const raw = scroller ? scroller.scrollTop : window.scrollY || 0;
  return Math.max(0, raw || 0);
}

function updatePullRefreshTracking(clientX, clientY, event = null) {
  if (!touchTracking) return;
  const deltaY = clientY - touchStartY;
  const deltaX = Math.abs(clientX - touchStartX);
  if (deltaY <= 0 || getScrollTop() > 0) {
    touchPullDistance = 0;
    setPullRefreshProgress(0);
    return;
  }
  if (deltaX > 24 && deltaX > deltaY) {
    cancelPullRefreshTracking();
    return;
  }
  if (event && typeof event.preventDefault === "function" && event.cancelable) {
    event.preventDefault();
  }
  touchPullDistance = deltaY;
  setPullRefreshProgress(deltaY);
}

function cancelPullRefreshTracking() {
  touchTracking = false;
  touchStartX = 0;
  touchStartY = 0;
  touchPullDistance = 0;
  touchPointerId = null;
  if (!pullRefreshing) {
    resetPullRefreshIndicator();
  }
}

function endPullRefreshTracking() {
  if (!touchTracking) return;
  const distance = touchPullDistance;
  touchTracking = false;
  touchStartX = 0;
  touchStartY = 0;
  touchPullDistance = 0;
  touchPointerId = null;
  if (distance >= PULL_REFRESH_THRESHOLD_PX && getScrollTop() === 0) {
    void triggerPullRefresh();
  } else if (!pullRefreshing) {
    resetPullRefreshIndicator();
  }
}

function initPullToRefresh() {
  if (!pullRefreshIndicator || !pullRefreshProgress) return;
  resetPullRefreshIndicator();

  const supportsTouch = "ontouchstart" in window || navigator.maxTouchPoints > 0;
  if (supportsTouch) {
    document.addEventListener(
      "touchstart",
      (event) => {
        if (!event.touches || event.touches.length !== 1) return;
        beginPullRefreshTracking(event.target, event.touches[0].clientX, event.touches[0].clientY);
      },
      { passive: true }
    );

    document.addEventListener(
      "touchmove",
      (event) => {
        if (!touchTracking || !event.touches || event.touches.length !== 1) return;
        updatePullRefreshTracking(event.touches[0].clientX, event.touches[0].clientY, event);
      },
      { passive: false }
    );

    document.addEventListener(
      "touchend",
      () => {
        endPullRefreshTracking();
      },
      { passive: true }
    );

    document.addEventListener(
      "touchcancel",
      () => {
        cancelPullRefreshTracking();
      },
      { passive: true }
    );
    return;
  }

  if (typeof window.PointerEvent !== "function") return;
  document.addEventListener(
    "pointerdown",
    (event) => {
      if (event.pointerType === "mouse") return;
      if (event.isPrimary === false) return;
      const pointerId = typeof event.pointerId === "number" ? event.pointerId : 1;
      beginPullRefreshTracking(event.target, event.clientX, event.clientY, pointerId);
    },
    { passive: true }
  );

  document.addEventListener(
    "pointermove",
    (event) => {
      if (event.pointerType === "mouse") return;
      if (event.isPrimary === false) return;
      const pointerId = typeof event.pointerId === "number" ? event.pointerId : 1;
      if (!touchTracking || touchPointerId !== pointerId) return;
      updatePullRefreshTracking(event.clientX, event.clientY, event);
    },
    { passive: false }
  );

  document.addEventListener(
    "pointerup",
    (event) => {
      if (event.pointerType === "mouse") return;
      if (event.isPrimary === false) return;
      const pointerId = typeof event.pointerId === "number" ? event.pointerId : 1;
      if (!touchTracking || touchPointerId !== pointerId) return;
      endPullRefreshTracking();
    },
    { passive: true }
  );

  document.addEventListener(
    "pointercancel",
    (event) => {
      if (event.pointerType === "mouse") return;
      if (event.isPrimary === false) return;
      const pointerId = typeof event.pointerId === "number" ? event.pointerId : 1;
      if (!touchTracking || touchPointerId !== pointerId) return;
      cancelPullRefreshTracking();
    },
    { passive: true }
  );
}

async function initSession() {
  try {
    const me = await fetchJSON("/api/auth/me");
    if (!me || !me.authenticated) {
      redirectToLogin();
      return false;
    }
    setAuthState(me.username || "admin");
    void clearAppBadgeOnOpen();
    return true;
  } catch (_) {
    redirectToLogin();
    return false;
  }
}

langSelect.addEventListener("change", async (event) => {
  setLanguage(event.target.value);
  await reloadAll({ includeAdvanced: showAdvanced });
  if (showAdvanced) {
    await loadPushConfig();
  }
});
logoutBtn.addEventListener("click", logout);
if (advancedToggleBtn) advancedToggleBtn.addEventListener("click", toggleAdvancedVisibility);
if (installBtn) installBtn.addEventListener("click", triggerInstall);
runUpdateBtn.addEventListener("click", createUpdateJob);
runPruneBtn.addEventListener("click", createPruneJob);
if (toggleSelectAllBtn) toggleSelectAllBtn.addEventListener("click", toggleSelectAllTargets);
jobsMoreBtn.addEventListener("click", loadMoreJobs);
jobsExportBtn.addEventListener("click", downloadJobsCSV);
if (jobsDeleteAllBtn) jobsDeleteAllBtn.addEventListener("click", deleteAllJobs);
if (pushEnableBtn) pushEnableBtn.addEventListener("click", enablePushNotifications);
if (pushDisableBtn) pushDisableBtn.addEventListener("click", disablePushNotifications);
if (pushTestBtn) pushTestBtn.addEventListener("click", sendPushTest);
refreshDiscoverBtn.addEventListener("click", loadDiscoverContainers);
discoverItemSelect.addEventListener("change", () => {
  renderDiscoverImages(selectedDiscoverItem());
});
addTargetForm.addEventListener("submit", registerSelectedTarget);
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") {
    void clearAppBadgeOnOpen();
  }
});

targetsBody.addEventListener("change", (event) => {
  const el = event.target;
  if (!(el instanceof HTMLInputElement)) return;
  if (el.name === "target-select") {
    const selectedId = Number(el.value);
    if (!Number.isInteger(selectedId) || selectedId <= 0) return;
    if (el.checked) {
      selectedTargetIdSet.add(selectedId);
    } else {
      selectedTargetIdSet.delete(selectedId);
    }
    return;
  }
  const action = el.dataset.action;
  const id = Number(el.dataset.id);
  if (!id || !action) return;
  if (action === "auto-toggle") {
    updateTarget(id, { auto_update_enabled: el.checked });
  } else if (action === "enabled-toggle") {
    updateTarget(id, { enabled: el.checked });
  }
});

targetsBody.addEventListener("click", (event) => {
  const target = event.target;
  if (!(target instanceof HTMLElement)) return;
  const button = target.closest("button[data-action]");
  if (button instanceof HTMLButtonElement) {
    const action = button.dataset.action;
    if (action === "toggle-details") {
      const key = button.dataset.key;
      if (!key) return;
      event.stopPropagation();
      toggleCompactDetails("targets", key);
      return;
    }
    if (action === "delete-target") {
      const id = Number(button.dataset.id);
      if (!id) return;
      event.stopPropagation();
      deleteTarget(id);
    }
    return;
  }

  if (!isCompactViewport()) return;
  if (target.closest("input,label,a,select,textarea")) return;
  const row = target.closest("tr.compact-row[data-compact-section='targets']");
  if (!row) return;
  const key = row.dataset.compactKey;
  if (!key) return;
  toggleCompactDetails("targets", key);
});

if (auditBody) {
  auditBody.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const button = target.closest("button[data-action]");
    if (button instanceof HTMLButtonElement) {
      if (button.dataset.action !== "toggle-details") return;
      const key = button.dataset.key;
      if (!key) return;
      event.stopPropagation();
      toggleCompactDetails("audit", key);
      return;
    }
    if (!isCompactViewport()) return;
    if (target.closest("input,label,a,select,textarea")) return;
    const row = target.closest("tr.compact-row[data-compact-section='audit']");
    if (!row) return;
    const key = row.dataset.compactKey;
    if (!key) return;
    toggleCompactDetails("audit", key);
  });
}

if (eventsBody) {
  eventsBody.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const button = target.closest("button[data-action]");
    if (button instanceof HTMLButtonElement) {
      if (button.dataset.action !== "toggle-details") return;
      const key = button.dataset.key;
      if (!key) return;
      event.stopPropagation();
      toggleCompactDetails("events", key);
      return;
    }
    if (!isCompactViewport()) return;
    if (target.closest("input,label,a,select,textarea")) return;
    const row = target.closest("tr.compact-row[data-compact-section='events']");
    if (!row) return;
    const key = row.dataset.compactKey;
    if (!key) return;
    toggleCompactDetails("events", key);
  });
}

if (receiptsBody) {
  receiptsBody.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const button = target.closest("button[data-action]");
    if (button instanceof HTMLButtonElement) {
      if (button.dataset.action !== "toggle-details") return;
      const key = button.dataset.key;
      if (!key) return;
      event.stopPropagation();
      toggleCompactDetails("receipts", key);
      return;
    }
    if (!isCompactViewport()) return;
    if (target.closest("input,label,a,select,textarea")) return;
    const row = target.closest("tr.compact-row[data-compact-section='receipts']");
    if (!row) return;
    const key = row.dataset.compactKey;
    if (!key) return;
    toggleCompactDetails("receipts", key);
  });
}

if (jobsBody) {
  jobsBody.addEventListener("click", (event) => {
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const btn = target.closest("button[data-action]");
    if (btn instanceof HTMLButtonElement) {
      const action = btn.dataset.action;
      if (action === "toggle-details") {
        const key = btn.dataset.key;
        if (!key) return;
        event.stopPropagation();
        toggleCompactDetails("jobs", key);
        return;
      }
      if (action === "delete-job") {
        const id = Number(btn.dataset.id);
        if (!id) return;
        event.stopPropagation();
        void deleteJob(id);
        return;
      }
      if (action === "open-log") {
        const id = Number(btn.dataset.id);
        if (!id) return;
        event.stopPropagation();
        void openLogStream(id);
      }
      return;
    }

    if (isCompactViewport()) {
      if (target.closest("input,label,a,select,textarea")) return;
      const compactRow = target.closest("tr.compact-row[data-compact-section='jobs']");
      if (!compactRow) {
        return;
      }
      const key = compactRow.dataset.compactKey;
      if (!key) return;
      toggleCompactDetails("jobs", key);
      return;
    }
    const row = target.closest("tr[data-job-id]");
    if (!row) return;
    const id = Number(row.dataset.jobId);
    if (!id) return;
    void openLogStream(id);
  });
}

async function init() {
  setLanguage(loadLanguage());
  setAdvancedVisibility(loadAdvancedVisibility(), { persist: false });
  initTargetsCollapse();
  initAuditCollapse();
  initPullToRefresh();
  setupServiceWorkerMessageHandling();
  initInstallPromptHandling();
  await registerServiceWorker();
  updateInstallUI();
  if (webhookConfig) webhookConfig.textContent = t("webhook.loading");
  if (pushStatus) pushStatus.textContent = t("push.status_loading");
  lastCompactViewport = isCompactViewport();
  window.addEventListener("resize", rerenderResponsiveTablesIfNeeded, { passive: true });

  const ok = await initSession();
  if (!ok) return;

  await reloadAll({ includeDiscover: true, includeAdvanced: showAdvanced });
  connectDashboardStream();
  startFallbackPolling();
}

init();

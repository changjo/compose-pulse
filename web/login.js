const langSelect = document.getElementById("login-lang");
const usernameInput = document.getElementById("username");
const passwordInput = document.getElementById("password");
const rememberInput = document.getElementById("remember-me");
const loginBtn = document.getElementById("login-btn");
const loginError = document.getElementById("login-error");

const I18N = {
  ko: {
    "login.title": "ComposePulse",
    "login.subtitle": "관리자 계정으로 로그인하세요.",
    "label.language": "언어",
    "auth.password": "비밀번호",
    "auth.remember_me": "자동 로그인",
    "auth.login": "Login",
    "auth.enter_credentials": "아이디와 비밀번호를 입력하세요.",
    "auth.rate_limited": "로그인 시도가 너무 많습니다. {seconds}초 후 다시 시도하세요.",
    "auth.login_failed": "로그인 실패: {error}",
  },
  en: {
    "login.title": "ComposePulse",
    "login.subtitle": "Sign in with the administrator account.",
    "label.language": "Language",
    "auth.password": "Password",
    "auth.remember_me": "Remember me",
    "auth.login": "Login",
    "auth.enter_credentials": "Please enter username and password.",
    "auth.rate_limited": "Too many login attempts. Try again in {seconds} seconds.",
    "auth.login_failed": "Login failed: {error}",
  },
};

const DEFAULT_LANG = "ko";
let currentLang = DEFAULT_LANG;
let redirectingToDashboard = false;

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

function applyI18n() {
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const key = el.getAttribute("data-i18n");
    if (!key) return;
    el.textContent = t(key);
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
}

async function fetchJSON(url, init = {}) {
  const response = await fetch(url, {
    credentials: "same-origin",
    ...init,
  });
  if (!response.ok) {
    const raw = await response.text();
    let message = raw || `${response.status} ${response.statusText}`;
    let code = "";
    let retryAfterSeconds = 0;
    try {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed.error === "string") {
        message = parsed.error;
      }
      if (parsed && typeof parsed.code === "string") {
        code = parsed.code;
      }
      if (parsed && Number.isFinite(Number(parsed.retry_after_seconds))) {
        retryAfterSeconds = Number(parsed.retry_after_seconds);
      }
    } catch (_) {
      // keep raw message
    }
    const err = new Error(message);
    err.status = response.status;
    err.code = code;
    err.retryAfterSeconds = retryAfterSeconds;
    throw err;
  }
  return response.json();
}

function showError(message) {
  loginError.textContent = message;
  loginError.classList.remove("hidden");
}

function clearError() {
  loginError.textContent = "";
  loginError.classList.add("hidden");
}

async function login() {
  clearError();
  const username = usernameInput.value.trim();
  const password = passwordInput.value;
  if (!username || !password) {
    showError(t("auth.enter_credentials"));
    return;
  }
  try {
    await fetchJSON("/api/auth/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Requested-With": "XMLHttpRequest",
      },
      body: JSON.stringify({
        username,
        password,
        remember_me: Boolean(rememberInput.checked),
      }),
    });
    if (!redirectingToDashboard) {
      redirectingToDashboard = true;
      window.location.replace("/");
    }
  } catch (err) {
    if (err && err.status === 429 && err.code === "auth_rate_limited" && Number.isFinite(err.retryAfterSeconds)) {
      showError(t("auth.rate_limited", { seconds: Math.max(1, Math.floor(err.retryAfterSeconds)) }));
      return;
    }
    showError(t("auth.login_failed", { error: err.message }));
  }
}

async function redirectIfLoggedIn() {
  try {
    const me = await fetchJSON("/api/auth/me");
    if (me && me.authenticated) {
      if (!redirectingToDashboard) {
        redirectingToDashboard = true;
        window.location.replace("/");
      }
    }
  } catch (_) {
    // ignore: stay on login page
  }
}

async function init() {
  setLanguage(loadLanguage());
  await redirectIfLoggedIn();
}

loginBtn.addEventListener("click", login);
passwordInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    login();
  }
});
usernameInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    login();
  }
});
langSelect.addEventListener("change", (event) => {
  setLanguage(event.target.value);
});

init();

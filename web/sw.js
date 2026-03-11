const STATIC_CACHE = "composepulse-static-v28";
const STATIC_ASSETS = [
  "/login.html",
  "/index.html",
  "/styles.css",
  "/app.js",
  "/login.js",
  "/manifest.webmanifest",
  "/icons/app-icon-32.png",
  "/icons/app-icon-64.png",
  "/icons/app-icon-180.png",
  "/icons/app-icon-192.png",
  "/icons/app-icon-512.png",
  "/icons/app-icon.svg",
  "/apple-touch-icon.png",
  "/apple-touch-icon-precomposed.png",
];

self.addEventListener("install", (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(STATIC_CACHE);
      await cache.addAll(STATIC_ASSETS);
      await self.skipWaiting();
    })()
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    (async () => {
      const keys = await caches.keys();
      await Promise.all(
        keys.map((key) => {
          if (key !== STATIC_CACHE) {
            return caches.delete(key);
          }
          return Promise.resolve(true);
        })
      );
      await self.clients.claim();
    })()
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") {
    return;
  }
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) {
    return;
  }
  if (url.pathname.startsWith("/api/")) {
    return;
  }
  if (url.pathname === "/" || url.pathname === "/login" || url.pathname === "/login/") {
    return;
  }

  event.respondWith(
    (async () => {
      const cache = await caches.open(STATIC_CACHE);
      try {
        const res = await fetch(req);
        if (res && res.status === 200 && url.protocol.startsWith("http")) {
          cache.put(req, res.clone());
        }
        return res;
      } catch (_) {
        const cached = await cache.match(req, { ignoreSearch: true });
        if (cached) {
          return cached;
        }
        const fallback = await cache.match("/index.html");
        if (fallback) {
          return fallback;
        }
        return new Response("offline", { status: 503 });
      }
    })()
  );
});

self.addEventListener("push", (event) => {
  let payload = {};
  try {
    payload = event.data ? event.data.json() : {};
  } catch (_) {
    payload = {};
  }
  const title = String(payload.title || "ComposePulse");
  const body = String(payload.body || "");
  const url = String(payload.url || "/");
  const tag = String(payload.tag || "composepulse");
  const badgeCount = Number(payload.badge_count || 1);
  event.waitUntil(
    (async () => {
      await setAppBadgeSafe(badgeCount > 0 ? badgeCount : 1);
      await self.registration.showNotification(title, {
        body,
        tag,
        icon: "/icons/app-icon-192.png",
        badge: "/icons/app-icon-64.png",
        data: { url },
      });
      const list = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      for (const client of list) {
        client.postMessage({
          type: "push-event",
          event_type: payload.event_type || "",
          url,
          badge_count: badgeCount > 0 ? badgeCount : 1,
        });
      }
    })()
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const rawURL = event.notification && event.notification.data ? event.notification.data.url : "/";
  const targetURL = String(rawURL || "/");
  event.waitUntil(
    (async () => {
      await clearAppBadgeSafe();
      const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
      for (const client of windows) {
        const clientURL = new URL(client.url);
        if (clientURL.origin !== self.location.origin) {
          continue;
        }
        if ("focus" in client) {
          await client.focus();
        }
        client.postMessage({ type: "push-open", url: targetURL });
        return;
      }
      if (self.clients.openWindow) {
        await self.clients.openWindow(targetURL);
      }
    })()
  );
});

self.addEventListener("message", (event) => {
  const payload = event && event.data ? event.data : null;
  if (!payload || typeof payload !== "object") return;
  if (payload.type === "clear-badge") {
    event.waitUntil(clearAppBadgeSafe());
    return;
  }
  if (payload.type === "set-badge") {
    const count = Number(payload.count || 0);
    event.waitUntil(setAppBadgeSafe(count));
  }
});

async function setAppBadgeSafe(count) {
  const normalized = Number.isFinite(Number(count)) ? Math.max(0, Math.floor(Number(count))) : 0;
  try {
    if (self.registration && typeof self.registration.setAppBadge === "function") {
      if (normalized > 0) {
        await self.registration.setAppBadge(normalized);
      } else if (typeof self.registration.clearAppBadge === "function") {
        await self.registration.clearAppBadge();
      } else {
        await self.registration.setAppBadge();
      }
      return;
    }
  } catch (_) {
    // ignore unsupported badging in service worker registration
  }
  try {
    if (typeof navigator !== "undefined" && navigator && typeof navigator.setAppBadge === "function") {
      if (normalized > 0) {
        await navigator.setAppBadge(normalized);
      } else if (typeof navigator.clearAppBadge === "function") {
        await navigator.clearAppBadge();
      }
    }
  } catch (_) {
    // ignore unsupported badging
  }
}

async function clearAppBadgeSafe() {
  try {
    if (self.registration && typeof self.registration.clearAppBadge === "function") {
      await self.registration.clearAppBadge();
      return;
    }
  } catch (_) {
    // ignore unsupported clear in service worker registration
  }
  try {
    if (typeof navigator !== "undefined" && navigator && typeof navigator.clearAppBadge === "function") {
      await navigator.clearAppBadge();
      return;
    }
    if (typeof navigator !== "undefined" && navigator && typeof navigator.setAppBadge === "function") {
      await navigator.setAppBadge(0);
    }
  } catch (_) {
    // ignore unsupported clear
  }
}

self.addEventListener("install", (e) => {
  self.skipWaiting();
});
self.addEventListener("activate", (e) => {
  self.clients.claim();
});
self.addEventListener("push", (e) => {
  let data = {};
  try {
    data = e.data ? e.data.json() : {};
  } catch {}
  const title = data.title || "Taskly";
  const body = data.body || "通知があります";
  const icon = data.icon || "/icons/icon-192.png";
  const badge = data.badge || "/icons/badge-72.png";
  const tag = data.tag || undefined;
  const dataPayload = data.data || {};
  e.waitUntil(
    self.registration.showNotification(title, {
      body,
      icon,
      badge,
      tag,
      data: dataPayload,
    })
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const targetUrl =
    (event.notification.data && event.notification.data.url) || "/home";
  event.waitUntil(
    self.clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then((clientList) => {
        for (const client of clientList) {
          const url = new URL(client.url);
          if (url.origin === self.location.origin) {
            client.focus();
            if (
              targetUrl &&
              url.pathname !== targetUrl &&
              "navigate" in client
            ) {
              return client.navigate(targetUrl);
            }
            return;
          }
        }
        return self.clients.openWindow(targetUrl);
      })
  );
});

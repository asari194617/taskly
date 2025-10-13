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
  const icon = data.icon; // only include when provided
  const badge = data.badge; // only include when provided
  const tag = data.tag || undefined;
  const dataPayload = data.data || {};
  e.waitUntil(
    self.registration.showNotification(
      title,
      Object.assign(
        { body, tag, data: dataPayload },
        icon ? { icon } : {},
        badge ? { badge } : {}
      )
    )
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

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
  e.waitUntil(self.registration.showNotification(title, { body }));
});

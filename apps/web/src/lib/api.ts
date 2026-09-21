export function getApiBase() {
  // Prefer PUBLIC_API_URL when provided; otherwise same-origin
  const base =
    (import.meta as any).env?.PUBLIC_API_URL || "http://localhost:3063";
  return base;
}

export function clearLegacySessionToken() {
  try {
    localStorage.removeItem("api_token");
  } catch {
    // localStorage may be unavailable in private or restricted contexts.
  }
}

export function webFetch(input: RequestInfo | URL, init: RequestInit = {}) {
  clearLegacySessionToken();
  const headers = new Headers(init.headers);
  if (!headers.has("X-Client-Platform")) {
    headers.set("X-Client-Platform", "web");
  }
  const method = (init.method || "GET").toUpperCase();
  if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
    headers.set("X-CSRF-Protection", "1");
  }
  return fetch(input, {
    ...init,
    headers,
    credentials: "include",
  });
}

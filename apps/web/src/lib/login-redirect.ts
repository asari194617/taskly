const localPath = /^\/(?![/\\])/;

export function loginNext(search: string): string {
  const next = new URLSearchParams(search).get("next") || "";
  return localPath.test(next) && !/[\\\r\n]/.test(next) && !/^\/login(?:\/|[?#]|$)/.test(next) ? next : "";
}

export function oauthStartURL(configuredURL: string | undefined, search: string): string | null {
  if (!configuredURL) return null;
  try {
    const url = new URL(configuredURL);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    const next = loginNext(search);
    if (next) url.searchParams.set("next", next);
    return url.toString();
  } catch {
    return null;
  }
}

export function legacyLoginURL(search: string): string {
  const next = loginNext(search);
  return next ? `/login?next=${encodeURIComponent(next)}` : "/login";
}

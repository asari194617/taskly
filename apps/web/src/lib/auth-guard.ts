// Simple client-side auth helpers for Astro pages

export function getToken(): string | null {
  try {
    return localStorage.getItem("api_token");
  } catch {
    return null;
  }
}

export function isAuthenticated(): boolean {
  return !!getToken();
}

export function currentPathWithQueryAndHash(): string {
  try {
    return `${location.pathname}${location.search}${location.hash}`;
  } catch {
    return "/";
  }
}

export function requireAuth(): void {
  try {
    if (!isAuthenticated()) {
      const next = encodeURIComponent(currentPathWithQueryAndHash());
      location.replace(`/login?next=${next}`);
    }
  } catch {
    // no-op
  }
}

export function redirectIfAuthed(defaultTarget: string = "/home"): void {
  try {
    if (isAuthenticated()) {
      const params = new URLSearchParams(location.search);
      const raw = params.get("next") || "";
      const safe = raw.startsWith("/") && !raw.startsWith("//") ? raw : "";
      location.replace(safe || defaultTarget);
    }
  } catch {
    // no-op
  }
}

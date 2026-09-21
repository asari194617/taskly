import { getApiBase, webFetch } from "./api";

export function currentPathWithQueryAndHash(): string {
  try {
    return `${location.pathname}${location.search}${location.hash}`;
  } catch {
    return "/";
  }
}

export function loginRedirect(authenticated: boolean, path: string): string | null {
  return authenticated ? null : `/login?next=${encodeURIComponent(path)}`;
}

export function postLoginRedirect(authenticated: boolean, search: string, defaultTarget = "/home"): string | null {
  if (!authenticated) return null;
  const raw = new URLSearchParams(search).get("next") || "";
  return /^\/(?![/\\])/.test(raw) ? raw : defaultTarget;
}

export function requireAuth(): void {
  try {
    checkSession().then((authenticated) => {
      const target = loginRedirect(authenticated, currentPathWithQueryAndHash());
      if (target) location.replace(target);
    });
  } catch {
    // no-op
  }
}

export function redirectIfAuthed(defaultTarget: string = "/home"): void {
  try {
    checkSession()
      .then((authenticated) => {
        const target = postLoginRedirect(authenticated, location.search, defaultTarget);
        if (target) location.replace(target);
      })
      .catch(() => {});
  } catch {
    // no-op
  }
}

export async function checkSession(): Promise<boolean> {
  try {
    const res = await webFetch(`${getApiBase()}/graphql`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ query: "query{ me{ id } }" }),
    });
    if (!res.ok) return false;
    const json = await res.json().catch(() => ({}));
    return !!json?.data?.me?.id;
  } catch {
    return false;
  }
}

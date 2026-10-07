import { getApiBase, webFetch } from "./api";

export function currentPathWithQueryAndHash(): string {
  try {
    return `${location.pathname}${location.search}${location.hash}`;
  } catch {
    return "/";
  }
}

export function loginRedirect(authenticated: boolean | null, path: string): string | null {
  return authenticated === false ? `/login?next=${encodeURIComponent(path)}` : null;
}

export function postLoginRedirect(authenticated: boolean | null, search: string, defaultTarget = "/home"): string | null {
  if (!authenticated) return null;
  const raw = new URLSearchParams(search).get("next") || "";
  return /^\/(?![/\\])/.test(raw) && !/^\/login(?:\/|[?#]|$)/.test(raw) ? raw : defaultTarget;
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

// null means the API could not confirm the session. Only an explicit 401
// should send the user back through login.
export async function checkSession(): Promise<boolean | null> {
  try {
    const res = await webFetch(`${getApiBase()}/graphql`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ query: "query{ me{ id } }" }),
    });
    if (res.status === 401) return false;
    if (!res.ok) return null;
    const json = await res.json();
    if (json?.errors?.length) return null;
    return json?.data?.me?.id ? true : null;
  } catch {
    return null;
  }
}

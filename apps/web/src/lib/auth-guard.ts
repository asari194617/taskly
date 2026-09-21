import { getApiBase, webFetch } from "./api";

export function currentPathWithQueryAndHash(): string {
  try {
    return `${location.pathname}${location.search}${location.hash}`;
  } catch {
    return "/";
  }
}

export function requireAuth(): void {
  try {
    checkSession().then((authenticated) => {
      if (!authenticated) {
        const next = encodeURIComponent(currentPathWithQueryAndHash());
        location.replace(`/login?next=${next}`);
      }
    });
  } catch {
    // no-op
  }
}

export function redirectIfAuthed(defaultTarget: string = "/home"): void {
  try {
    checkSession()
      .then((authenticated) => {
        if (!authenticated) return;
        const params = new URLSearchParams(location.search);
        const raw = params.get("next") || "";
        const safe = raw.startsWith("/") && !raw.startsWith("//") ? raw : "";
        location.replace(safe || defaultTarget);
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

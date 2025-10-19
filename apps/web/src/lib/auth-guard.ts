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
    if (!isAuthenticated()) return;
    // 既存トークンが本当に有効か最小検証（/graphql me）
    const token = getToken()!;
    const API_BASE =
      (import.meta as any).env?.PUBLIC_API_URL || "http://localhost:3063";
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    headers["Authorization"] = token.startsWith("Bearer ")
      ? token
      : `Bearer ${token}`;
    fetch(`${API_BASE}/graphql`, {
      method: "POST",
      headers,
      body: JSON.stringify({ query: "query{ me{ id } }" }),
    })
      .then(async (res) => {
        if (!res.ok) throw new Error("unauthorized");
        const json = await res.json().catch(() => ({}));
        if (!json?.data?.me?.id) throw new Error("unauthorized");
        const params = new URLSearchParams(location.search);
        const raw = params.get("next") || "";
        const safe = raw.startsWith("/") && !raw.startsWith("//") ? raw : "";
        location.replace(safe || defaultTarget);
      })
      .catch(() => {
        // 無効トークンなら破棄してログイン継続
        try {
          localStorage.removeItem("api_token");
        } catch {}
      });
  } catch {
    // no-op
  }
}

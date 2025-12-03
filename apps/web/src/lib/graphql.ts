export type GQLError = { message: string };

export async function gql<T>(
  query: string,
  variables?: Record<string, any>,
  opts?: { signal?: AbortSignal }
): Promise<T> {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const base =
    (import.meta as any).env?.PUBLIC_API_URL || "http://localhost:3063";
  const res = await fetch(`${base}/graphql`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Client-Platform": "web",
      ...(token
        ? {
            Authorization: token.startsWith("Bearer ")
              ? token
              : `Bearer ${token}`,
          }
        : {}),
    },
    body: JSON.stringify({ query, variables }),
    signal: opts?.signal,
  });
  const json = await res.json();
  if (!res.ok || json.errors?.length) {
    // If unauthorized and we have a token, treat as session expiry: clear token and redirect to login
    if (typeof window !== "undefined" && res.status === 401 && token) {
      try {
        localStorage.removeItem("api_token");
      } catch {}
      // Redirect to login page
      try {
        window.location.href = "/login";
      } catch {}
      throw new Error("token_expired");
    }
    const message =
      json.errors?.map((e: GQLError) => e.message).join("\n") || res.statusText;
    throw new Error(message);
  }
  return json.data as T;
}

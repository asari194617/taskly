import { getApiBase, webFetch } from "./api";

export type GQLError = { message: string };

export async function gql<T>(
  query: string,
  variables?: Record<string, any>,
  opts?: { signal?: AbortSignal; redirectOnUnauthorized?: boolean }
): Promise<T> {
  const res = await webFetch(`${getApiBase()}/graphql`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ query, variables }),
    signal: opts?.signal,
  });
  const json = await res.json();
  if (!res.ok || json.errors?.length) {
    const unauthorized = res.status === 401;
    if (
      typeof window !== "undefined" &&
      unauthorized &&
      opts?.redirectOnUnauthorized !== false
    ) {
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

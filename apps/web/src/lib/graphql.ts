export type GQLError = { message: string };

export async function gql<T>(
  query: string,
  variables?: Record<string, any>
): Promise<T> {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const base =
    (import.meta as any).env?.PUBLIC_API_URL || "http://localhost:8080";
  const res = await fetch(`${base}/graphql`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: token } : {}),
    },
    body: JSON.stringify({ query, variables }),
  });
  const json = await res.json();
  if (!res.ok || json.errors?.length) {
    const message =
      json.errors?.map((e: GQLError) => e.message).join("\n") || res.statusText;
    throw new Error(message);
  }
  return json.data as T;
}

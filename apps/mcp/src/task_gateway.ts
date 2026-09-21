export type TaskGateway = (
  token: string,
  method: "GET" | "POST" | "PATCH" | "DELETE",
  path: string,
  body?: unknown,
) => Promise<{ status: number; body: any }>;

// The Go API owns task authorization, validation and notification side effects.
export function httpTaskGateway(base = process.env.TASKLY_API_BASE_URL || "http://server:8080"): TaskGateway {
  return async (token, method, path, body) => {
    const response = await fetch(new URL(path, base), {
      method,
      headers: { "X-MCP-Token": token, "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: AbortSignal.timeout(10000),
    });
    const text = await response.text();
    let value: any;
    try {
      value = text ? JSON.parse(text) : undefined;
    } catch {
      value = { error: text || response.statusText };
    }
    return { status: response.status, body: value };
  };
}

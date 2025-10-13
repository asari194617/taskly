export function getApiBase() {
  // Prefer PUBLIC_API_URL when provided; otherwise same-origin
  const base =
    (import.meta as any).env?.PUBLIC_API_URL || "http://localhost:8080";
  return base;
}

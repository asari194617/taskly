import { gql } from "../../lib/graphql";

export const apiBase =
  (import.meta as any).env.PUBLIC_API_URL || "http://localhost:3063";

export function getNextParam(): string {
  try {
    const params = new URLSearchParams(location.search);
    const raw = params.get("next") || "";
    return raw.startsWith("/") && !raw.startsWith("//") ? raw : "";
  } catch {
    return "";
  }
}

export function withNext(url: string): string {
  const next = getNextParam();
  if (!next) return url;
  const u = new URL(url, location.origin);
  if (next) u.searchParams.set("next", next);
  return u.pathname + u.search;
}

export async function postLoginRedirect() {
  const target = getNextParam() || "/home";
  try {
    const res = await gql<{ me: { is_agreed_policies: boolean } }>(
      "query{ me{ is_agreed_policies } }"
    );
    if (!res.me?.is_agreed_policies) {
      window.location.replace(`/consent?next=${encodeURIComponent(target)}`);
      return;
    }
  } catch {
    // ignore
  }
  window.location.replace(target);
}

export function getEmailFromQuery(): string {
  try {
    const params = new URLSearchParams(location.search);
    return params.get("email") || "";
  } catch {
    return "";
  }
}

// WebAuthn helpers
export const b64urlToArray = (buf: string) => {
  buf = buf.replace(/-/g, "+").replace(/_/g, "/");
  const pad = buf.length % 4;
  if (pad) buf += "=".repeat(4 - pad);
  const bin = atob(buf);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
};
export const arrayToB64url = (bytes: ArrayBuffer | Uint8Array) => {
  const arr = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  let s = "";
  for (let i = 0; i < arr.length; i++) s += String.fromCharCode(arr[i]);
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
};

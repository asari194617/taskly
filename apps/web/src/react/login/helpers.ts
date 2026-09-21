import { gql } from "../../lib/graphql";

export const apiBase =
  (import.meta as any).env.PUBLIC_API_URL || "http://localhost:3063";

const AUTH_TRANSACTION_KEY = "login_auth_transaction";

export type AuthTransactionState = {
  id: string;
  email: string;
  expiresAt: number;
  methods: string[];
};

export function saveAuthTransaction(
  id: string,
  email: string,
  expiresInSeconds: number,
  methods: string[]
) {
  const state: AuthTransactionState = {
    id,
    email: email.trim().toLowerCase(),
    expiresAt: Date.now() + expiresInSeconds * 1000,
    methods,
  };
  sessionStorage.setItem(AUTH_TRANSACTION_KEY, JSON.stringify(state));
}

export function readAuthTransaction(
  email: string
): AuthTransactionState | null {
  try {
    const raw = sessionStorage.getItem(AUTH_TRANSACTION_KEY);
    if (!raw) return null;
    const state = JSON.parse(raw) as AuthTransactionState;
    if (
      !state?.id ||
      state.email !== email.trim().toLowerCase() ||
      !Number.isFinite(state.expiresAt) ||
      state.expiresAt <= Date.now()
    ) {
      clearAuthTransaction();
      return null;
    }
    return state;
  } catch {
    clearAuthTransaction();
    return null;
  }
}

export function clearAuthTransaction() {
  sessionStorage.removeItem(AUTH_TRANSACTION_KEY);
}

export function isAuthTransactionError(error: unknown): boolean {
  return (
    typeof error === "string" &&
    (error.startsWith("auth transaction") ||
      error === "password required" ||
      error === "submit password before mfa")
  );
}

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

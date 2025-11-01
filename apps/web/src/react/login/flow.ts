// Global-ish helpers for login flow side-effects that should outlive a single component.
// Keep it small and dependency-free so it's safe across Astro page transitions.
import { apiBase } from "./helpers";

const KEY_SENT_AT = (email: string) => `mail_code_requested_at:${email}`;
const KEY_LAST_ERROR = (email: string) => `mail_code_last_error:${email}`;

export type MailCodeRequestResult = {
  ok: boolean;
  error?: string;
  at?: number;
};

export async function requestMailCode(
  email: string
): Promise<MailCodeRequestResult> {
  if (!email) return { ok: false, error: "メールアドレスが未入力です" };
  try {
    const res = await fetch(`${apiBase}/auth/request-code`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Client-Platform": "web",
      },
      // Ensure the request is delivered even if navigation happens quickly
      keepalive: true,
      body: JSON.stringify({ email }),
    });
    if (!res.ok) {
      const err = await res.text();
      sessionStorage.setItem(
        KEY_LAST_ERROR(email),
        err || "送信に失敗しました"
      );
      return { ok: false, error: err || "送信に失敗しました" };
    }
    const at = Date.now();
    sessionStorage.setItem(KEY_SENT_AT(email), String(at));
    sessionStorage.removeItem(KEY_LAST_ERROR(email));
    return { ok: true, at };
  } catch (e: any) {
    const msg = e?.message || "送信に失敗しました";
    sessionStorage.setItem(KEY_LAST_ERROR(email), msg);
    return { ok: false, error: msg };
  }
}

export function readMailCodeStatus(
  email: string
): MailCodeRequestResult | null {
  if (!email) return null;
  const sent = sessionStorage.getItem(KEY_SENT_AT(email));
  const err = sessionStorage.getItem(KEY_LAST_ERROR(email));
  if (err) return { ok: false, error: err };
  if (sent) return { ok: true, at: Number(sent) || Date.now() };
  return null;
}

export const DEFAULT_RESEND_COOLDOWN_SEC = 60;

export function secondsUntilResend(
  email: string,
  now: number = Date.now(),
  cooldownSec: number = DEFAULT_RESEND_COOLDOWN_SEC
): number {
  if (!email) return 0;
  const sent = sessionStorage.getItem(KEY_SENT_AT(email));
  if (!sent) return 0;
  const at = Number(sent) || 0;
  const remainMs = at + cooldownSec * 1000 - now;
  return remainMs > 0 ? Math.ceil(remainMs / 1000) : 0;
}

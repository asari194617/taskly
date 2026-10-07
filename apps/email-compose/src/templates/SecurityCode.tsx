import { CodeEmail } from "./CodeEmail.js";

export function SecurityCodeEmail({ code, email, locale, expiryMinutes = 10 }: {
  code: string;
  email: string;
  locale?: string;
  expiryMinutes?: number;
}) {
  return CodeEmail({ code, email, locale, expiryMinutes, namespace: "security-code" });
}

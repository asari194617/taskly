import * as React from "react";
import { CodeEmail } from "./CodeEmail.js";

export async function LoginCodeEmail({
  code,
  email,
  locale,
  expiryMinutes = 5,
}: {
  code: string;
  email: string;
  locale?: string;
  expiryMinutes?: number;
}) {
  return CodeEmail({ code, email, locale, expiryMinutes, namespace: "login-code" });
}

export default LoginCodeEmail;

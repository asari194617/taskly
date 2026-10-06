/// <reference path="../.astro/types.d.ts" />
/// <reference types="react" />
/// <reference types="react-dom" />

interface ImportMetaEnv {
  readonly PUBLIC_API_URL?: string;
  readonly PUBLIC_AUTH_LOGOUT_URL?: string;
  readonly PUBLIC_AUTH_PASSKEY_URL?: string;
  readonly PUBLIC_AUTH_PASSWORD_URL?: string;
  readonly PUBLIC_AUTH_EMAIL_CODE_URL?: string;
  readonly PUBLIC_AUTH_RECOVERY_URL?: string;
  readonly PUBLIC_AUTH_TOTP_URL?: string;
  readonly PUBLIC_AUTH_PROFILE_URL?: string;
  readonly PUBLIC_AUTH_SECURITY_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}

// Allow importing global CSS in .astro files
declare module "*.css";

import { gql } from "@/lib/graphql";

export type AuthState = {
  email_login_enabled: boolean;
  has_password: boolean;
  totp_enabled: boolean;
  passkey_count?: number;
  recovery_codes_count?: number;
};

export type Passkey = {
  id: number;
  device_name: string;
  created_at: string;
};

export function loadSecuritySettings() {
  return gql<{ authSettings: AuthState; myPasskeys: Passkey[] }>(
    `query{ authSettings{ email_login_enabled has_password totp_enabled passkey_count recovery_codes_count } myPasskeys{ id device_name created_at } }`,
  );
}

export function setEmailLogin(enabled: boolean) {
  return gql<{ setAuthSettings: boolean }>(
    `mutation($on:Boolean!){ setAuthSettings(email_login_enabled:$on) }`,
    { on: enabled },
  );
}

export async function beginTotpSetup() {
  const result = await gql<{ totpSetup: { secret: string; otpauth: string } }>(
    `mutation{ totpSetup{ secret otpauth } }`,
  );
  return result.totpSetup;
}

export function activateTotpSetup(code: string) {
  return gql<{ totpActivate: boolean }>(
    `mutation($code:String!){ totpActivate(code:$code) }`,
    { code },
  );
}

export function disableTotpSetup() {
  return gql<{ totpDisable: boolean }>(`mutation{ totpDisable }`);
}

export async function generateRecoveryCodes() {
  const result = await gql<{ recoveryGenerate: string[] }>(
    `mutation{ recoveryGenerate }`,
  );
  return result.recoveryGenerate;
}

export async function beginPasskeyRegistration() {
  const result = await gql<{ webauthnRegisterBegin: string }>(
    `mutation{ webauthnRegisterBegin }`,
  );
  return JSON.parse(result.webauthnRegisterBegin);
}

export function finishPasskeyRegistration(payload: unknown) {
  return gql<boolean>(
    `mutation($input:String!){ webauthnRegisterFinish(input:$input) }`,
    { input: JSON.stringify(payload) },
  );
}

export function setPassword(current: string | undefined, next: string) {
  return gql<{ setPassword: boolean }>(
    `mutation($current:String,$next:String!){ setPassword(current:$current, next:$next) }`,
    { current, next },
  );
}

export function renamePasskey(id: number, name: string) {
  return gql(`mutation($id:Int!,$name:String!){ renamePasskey(id:$id,name:$name) }`, {
    id,
    name,
  });
}

export function deletePasskey(id: number) {
  return gql(`mutation($id:Int!){ deletePasskey(id:$id) }`, { id });
}

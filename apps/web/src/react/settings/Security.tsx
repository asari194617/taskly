"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type AuthState = {
  email_login_enabled: boolean;
  has_password: boolean;
  totp_enabled: boolean;
};

export default function SettingsSecurity() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [auth, setAuth] = React.useState<AuthState | null>(null);
  const [totpSecret, setTotpSecret] = React.useState<string>("");
  const [totpUri, setTotpUri] = React.useState<string>("");
  const [totpCode, setTotpCode] = React.useState<string>("");

  React.useEffect(() => {
    if (!token) return;
    fetch("/settings/auth", { headers: { Authorization: token } })
      .then((r) => r.json())
      .then(setAuth)
      .catch(() => {});
  }, [token]);

  async function toggleEmailLogin(v: boolean) {
    const prev = auth;
    setAuth((s) => (s ? { ...s, email_login_enabled: v } : s));
    const res = await fetch("/settings/auth", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: token },
      body: JSON.stringify({ email_login_enabled: v }),
    });
    if (!res.ok && prev) setAuth(prev);
  }

  async function beginTotp() {
    const res = await fetch("/2fa/totp/setup", {
      method: "POST",
      headers: { Authorization: token },
    });
    if (res.ok) {
      const data = await res.json();
      setTotpSecret(data.secret);
      setTotpUri(data.otpauth);
    }
  }
  async function activateTotp() {
    const res = await fetch("/2fa/totp/activate", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: token },
      body: JSON.stringify({ code: totpCode }),
    });
    if (res.ok) {
      alert("TOTPを有効化しました");
      setAuth((s) => (s ? { ...s, totp_enabled: true } : s));
      setTotpSecret("");
      setTotpUri("");
      setTotpCode("");
    } else {
      alert("コードが正しくありません");
    }
  }
  async function disableTotp() {
    const res = await fetch("/2fa/totp/disable", {
      method: "POST",
      headers: { Authorization: token },
    });
    if (res.ok) setAuth((s) => (s ? { ...s, totp_enabled: false } : s));
  }

  async function generateRecovery() {
    const res = await fetch("/2fa/recovery/generate", {
      method: "POST",
      headers: { Authorization: token },
    });
    if (res.ok) {
      const data = await res.json();
      const text = (data.codes as string[]).join("\n");
      await navigator.clipboard.writeText(text);
      alert(
        "リカバリーコードをクリップボードにコピーしました。安全な場所に保存してください。"
      );
    }
  }

  async function registerPasskey() {
    // Begin
    const meRes = await fetch("/users/me", {
      headers: { Authorization: token },
    });
    if (!meRes.ok) return;
    const me = await meRes.json();
    const begin = await fetch("/webauthn/register/begin", {
      method: "POST",
      headers: { Authorization: token },
    });
    const options = await begin.json();
    (options.publicKey.challenge as any) = base64urlToBuffer(
      options.publicKey.challenge
    );
    options.publicKey.user.id = new TextEncoder().encode(String(me.id));
    const cred: any = await navigator.credentials.create({
      publicKey: options.publicKey,
    });
    const attResp = cred.response;
    const finishPayload = {
      id: cred.id,
      rawId: bufferToBase64url(cred.rawId),
      type: cred.type,
      response: {
        attestationObject: bufferToBase64url(attResp.attestationObject),
        clientDataJSON: bufferToBase64url(attResp.clientDataJSON),
      },
    };
    const finish = await fetch("/webauthn/register/finish", {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: token },
      body: JSON.stringify(finishPayload),
    });
    if (finish.ok) alert("パスキーを登録しました");
  }

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">セキュリティ設定</h1>
        <Card className="p-4 space-y-3">
          <div className="flex items-center justify-between gap-4">
            <div>
              <div className="font-medium">メールコードでのログイン</div>
              <div className="text-sm text-muted-foreground">
                ONをオフにするには他の認証手段（パスワード/TOTP/パスキー）が必要です。
              </div>
            </div>
            <Switch
              checked={!!auth?.email_login_enabled}
              onCheckedChange={toggleEmailLogin}
            />
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="font-medium">TOTP（二段階認証）</div>
          {auth?.totp_enabled ? (
            <div className="flex items-center gap-3">
              <div className="text-sm text-muted-foreground">
                有効になっています
              </div>
              <Button variant="secondary" onClick={disableTotp}>
                無効化
              </Button>
            </div>
          ) : (
            <div className="space-y-3">
              {totpSecret ? (
                <div className="space-y-2">
                  <div className="text-sm">
                    シークレット:{" "}
                    <code className="font-mono">{totpSecret}</code>
                  </div>
                  <div className="text-xs text-muted-foreground break-all">
                    {totpUri}
                  </div>
                  <div className="flex items-end gap-2">
                    <div className="flex-1">
                      <Label htmlFor="totp">認証コード</Label>
                      <Input
                        id="totp"
                        value={totpCode}
                        onChange={(e) => setTotpCode(e.target.value)}
                        placeholder="123456"
                      />
                    </div>
                    <Button onClick={activateTotp}>有効化</Button>
                  </div>
                </div>
              ) : (
                <Button onClick={beginTotp}>セットアップを開始</Button>
              )}
            </div>
          )}
        </Card>
        <Card className="p-4 space-y-3">
          <div className="font-medium">パスキー（Passkey）</div>
          <div className="text-sm text-muted-foreground">
            この端末のパスキーを登録します。
          </div>
          <Button onClick={registerPasskey}>パスキーを登録</Button>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="font-medium">リカバリーコード</div>
          <div className="text-sm text-muted-foreground">
            新しいコードを発行すると、以前の未使用コードは無効になります。
          </div>
          <Button onClick={generateRecovery}>コードを再発行</Button>
        </Card>
      </div>
    </SettingsShell>
  );
}

function base64urlToBuffer(base64url: string) {
  const padding = "=".repeat((4 - (base64url.length % 4)) % 4);
  const base64 = (base64url + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const buffer = new ArrayBuffer(raw.length);
  const view = new Uint8Array(buffer);
  for (let i = 0; i < raw.length; ++i) view[i] = raw.charCodeAt(i);
  return buffer;
}
function bufferToBase64url(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++)
    binary += String.fromCharCode(bytes[i]);
  return btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

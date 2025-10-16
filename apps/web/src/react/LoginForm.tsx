import * as React from "react";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { Label } from "../components/ui/label";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../components/ui/card";
import { Modal } from "../components/ui/modal";
import { gql } from "../lib/graphql";

const apiBase =
  (import.meta as any).env.PUBLIC_API_URL || "http://localhost:3063";

export default function LoginForm() {
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [code, setCode] = React.useState("");
  const [totp, setTotp] = React.useState("");
  const [recovery, setRecovery] = React.useState("");
  const [hasPassword, setHasPassword] = React.useState<boolean | null>(null);
  const [totpEnabled, setTotpEnabled] = React.useState<boolean>(false);
  const [passkeyEnabled, setPasskeyEnabled] = React.useState<boolean>(false);
  const [availableMethods, setAvailableMethods] = React.useState<string[]>([]);
  const [selectedMethod, setSelectedMethod] = React.useState<string>("");
  const [loading, setLoading] = React.useState(false);
  const [sending, setSending] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);
  const [step, setStep] = React.useState<"email" | "verify">("email");
  const [blocked, setBlocked] = React.useState<string | null>(null);

  // Determine post-login redirect target
  const getRedirectTarget = React.useCallback(() => {
    try {
      const params = new URLSearchParams(location.search);
      const raw = params.get("next") || "";
      const safe = raw.startsWith("/") && !raw.startsWith("//") ? raw : "";
      return safe || "/home";
    } catch {
      return "/home";
    }
  }, []);

  const discover = async () => {
    setMessage(null);
    setHasPassword(null);
    setTotpEnabled(false);
    if (!email) return;
    try {
      const res = await fetch(`${apiBase}/auth/options`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email }),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      setHasPassword(!!data?.has_password);
      setTotpEnabled(!!data?.totp_enabled);
      const methods: string[] = Array.isArray(data?.methods)
        ? data.methods
        : [];
      setAvailableMethods(methods);
      setPasskeyEnabled(!!data?.passkey_enabled || methods.includes("passkey"));
      setStep("verify");
    } catch (_err) {
      setMessage("認証方式の取得に失敗しました");
    }
  };

  const postLoginRedirect = async () => {
    const target = getRedirectTarget();
    try {
      const res = await gql<{ me: { is_agreed_policies: boolean } }>(
        "query{ me{ is_agreed_policies } }"
      );
      if (!res.me?.is_agreed_policies) {
        window.location.replace(`/consent?next=${encodeURIComponent(target)}`);
        return;
      }
    } catch {
      // ignore and fallback to normal redirect
    }
    window.location.replace(target);
  };

  const requestCode = async () => {
    setSending(true);
    setMessage(null);
    try {
      const res = await fetch(`${apiBase}/auth/request-code`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email }),
      });
      if (!res.ok) throw new Error(await res.text());
      setMessage("確認コードを送信しました。メールをご確認ください。");
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
    } finally {
      setSending(false);
    }
  };

  const login = async () => {
    setLoading(true);
    setMessage(null);
    try {
      const res = await fetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({
          email,
          password: password || undefined,
          code: code || undefined,
          totp: totp || undefined,
          recovery_code: recovery || undefined,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        if (data?.error === "blocked") {
          setBlocked(
            "このアカウントは現在ブロックされています。管理者にお問い合わせください。"
          );
          throw new Error("blocked");
        }
        throw new Error(data?.error || JSON.stringify(data));
      }
      if (data?.token) {
        localStorage.setItem("api_token", data.token);
        setMessage("ログインしました");
        // Redirect immediately to consent if not agreed; else to target/home
        await postLoginRedirect();
      } else {
        setMessage("予期せぬレスポンスです");
      }
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
    } finally {
      setLoading(false);
    }
  };

  // WebAuthn helpers
  const b64urlToArray = (buf: string) => {
    buf = buf.replace(/-/g, "+").replace(/_/g, "/");
    const pad = buf.length % 4;
    if (pad) buf += "=".repeat(4 - pad);
    const bin = atob(buf);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  };
  const arrayToB64url = (bytes: ArrayBuffer | Uint8Array) => {
    const arr = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
    let s = "";
    for (let i = 0; i < arr.length; i++) s += String.fromCharCode(arr[i]);
    return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  };
  const passkeyLogin = async () => {
    if (!email) return;
    setLoading(true);
    setMessage(null);
    try {
      const beginRes = await fetch(`${apiBase}/webauthn/login/begin`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email }),
      });
      if (!beginRes.ok) throw new Error(await beginRes.text());
      const options = await beginRes.json();
      // decode challenge, allowCredentials ids
      options.challenge = b64urlToArray(options.challenge);
      if (options.allowCredentials) {
        options.allowCredentials = options.allowCredentials.map((c: any) => ({
          ...c,
          id: b64urlToArray(c.id),
        }));
      }
      const assertion: any = await (navigator as any).credentials.get({
        publicKey: options,
      });
      if (!assertion) throw new Error("キャンセルされました");
      const payload = {
        email,
        id: assertion.id,
        rawId: arrayToB64url(assertion.rawId),
        type: assertion.type,
        response: {
          authenticatorData: arrayToB64url(
            assertion.response.authenticatorData
          ),
          clientDataJSON: arrayToB64url(assertion.response.clientDataJSON),
          signature: arrayToB64url(assertion.response.signature),
          userHandle: assertion.response.userHandle
            ? arrayToB64url(assertion.response.userHandle)
            : null,
        },
      };
      const finishRes = await fetch(`${apiBase}/webauthn/login/finish`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify(payload),
      });
      const data = await finishRes.json();
      if (!finishRes.ok) throw new Error(data?.error || "passkey login failed");
      if (data?.token) {
        localStorage.setItem("api_token", data.token);
        setMessage("ログインしました（パスキー）");
        // redirect after passkey login with consent check
        await postLoginRedirect();
      } else {
        setMessage("予期せぬレスポンスです");
      }
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>ログイン</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {step === "email" && (
            <div className="space-y-2">
              <Label htmlFor="email">メールアドレス</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
              />
              <div className="flex gap-2">
                <Button
                  type="button"
                  onClick={discover}
                  disabled={loading || !email}
                >
                  次へ
                </Button>
              </div>
            </div>
          )}

          {step === "verify" && hasPassword === true ? (
            <div className="space-y-2">
              <Label htmlFor="password">パスワード</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
              />
            </div>
          ) : null}
          {step === "verify" && hasPassword === false && (
            <div className="space-y-4">
              <div className="space-y-1">
                <Label>認証方法を選択</Label>
                <div className="flex flex-wrap gap-2">
                  {availableMethods.includes("email_code") && (
                    <Button
                      type="button"
                      variant={
                        selectedMethod === "email_code"
                          ? "default"
                          : "secondary"
                      }
                      onClick={async () => {
                        setSelectedMethod("email_code");
                        await requestCode();
                      }}
                      disabled={loading || sending || !email}
                    >
                      {sending ? "送信中…" : "メールコード"}
                    </Button>
                  )}
                  {availableMethods.includes("passkey") && (
                    <Button
                      type="button"
                      variant={
                        selectedMethod === "passkey" ? "default" : "secondary"
                      }
                      onClick={() => setSelectedMethod("passkey")}
                    >
                      パスキー
                    </Button>
                  )}
                </div>
              </div>
              {selectedMethod === "email_code" && (
                <div className="space-y-2">
                  <Label htmlFor="code">メールコード</Label>
                  <Input
                    id="code"
                    type="text"
                    inputMode="numeric"
                    value={code}
                    onChange={(e) => setCode(e.target.value)}
                    placeholder="123456"
                  />
                </div>
              )}
            </div>
          )}

          {step === "verify" && totpEnabled && (
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-2">
                <Label htmlFor="totp">TOTP</Label>
                <Input
                  id="totp"
                  type="text"
                  inputMode="numeric"
                  value={totp}
                  onChange={(e) => setTotp(e.target.value)}
                  placeholder="123456"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="recovery">リカバリー</Label>
                <Input
                  id="recovery"
                  type="text"
                  value={recovery}
                  onChange={(e) => setRecovery(e.target.value)}
                  placeholder="abcd-efgh"
                />
              </div>
            </div>
          )}

          {step === "verify" && (
            <div className="flex gap-2">
              <Button
                type="button"
                onClick={login}
                disabled={loading || !email}
              >
                ログイン
              </Button>
              {(passkeyEnabled || selectedMethod === "passkey") && (
                <Button
                  type="button"
                  variant="secondary"
                  onClick={passkeyLogin}
                  disabled={loading || !email}
                >
                  パスキーでログイン
                </Button>
              )}
              <Button
                type="button"
                variant="secondary"
                onClick={() => setStep("email")}
              >
                戻る
              </Button>
            </div>
          )}
          {message && (
            <p className="text-sm text-muted-foreground">{message}</p>
          )}
        </CardContent>
      </Card>
      <Modal
        open={!!blocked}
        onClose={() => setBlocked(null)}
        title="ログインできません"
      >
        <p className="text-sm text-muted-foreground">{blocked}</p>
      </Modal>
    </>
  );
}

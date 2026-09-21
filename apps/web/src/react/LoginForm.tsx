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
import { webFetch } from "../lib/api";

const apiBase =
  (import.meta as any).env.PUBLIC_API_URL || "http://localhost:3063";

type Step =
  | "begin" // DispLoginBegin
  | "password" // DispPasswordInput
  | "select_mfa" // DispSelectMfaMethod
  | "passkey" // DispPasskeyInput
  | "totp" // DispTotpInput
  | "mail_code"; // DispMailCodeInput

export default function LoginForm() {
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [code, setCode] = React.useState("");
  const [totp, setTotp] = React.useState("");
  const [recovery, setRecovery] = React.useState("");
  const [authTransaction, setAuthTransaction] = React.useState("");
  const [hasPassword, setHasPassword] = React.useState<boolean | null>(null);
  const [totpAvailable, setTotpAvailable] = React.useState<boolean>(false);
  const [passkeyAvailable, setPasskeyAvailable] =
    React.useState<boolean>(false);
  const [emailCodeAvailable, setEmailCodeAvailable] =
    React.useState<boolean>(false);
  const [availableMethods, setAvailableMethods] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [sending, setSending] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);
  const [step, setStep] = React.useState<Step>("begin");
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

  // Decide next UI state based on server-advertised methods per PlantUML
  const decideInitialStep = (
    hasPw: boolean,
    methods: string[],
    flags: { totp: boolean; passkey: boolean; emailCode: boolean }
  ) => {
    if (hasPw) {
      // If password is enabled, go to password input first
      setStep("password");
      return;
    }
    const enabled: Array<"passkey" | "mail_code"> = [];
    if (flags.passkey) enabled.push("passkey");
    if (flags.emailCode) enabled.push("mail_code");
    // If only one MFA method is enabled, go directly to that screen; else show selection
    if (enabled.length === 1) {
      setStep(enabled[0]);
    } else {
      setStep("select_mfa");
    }
  };

  const discover = async () => {
    setMessage(null);
    setHasPassword(null);
    setTotpAvailable(false);
    if (!email) return;
    try {
      const res = await webFetch(`${apiBase}/auth/options`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email }),
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      const hasPw = !!data?.has_password;
      setHasPassword(hasPw);
      setTotpAvailable(!!data?.totp_enabled);
      const methods: string[] = Array.isArray(data?.methods)
        ? data.methods
        : [];
      setAvailableMethods(methods);
      const passkey = !!data?.passkey_enabled || methods.includes("passkey");
      const emailCode = methods.includes("email_code");
      setPasskeyAvailable(passkey);
      setEmailCodeAvailable(emailCode);
      decideInitialStep(hasPw, methods, {
        totp: !!data?.totp_enabled,
        passkey,
        emailCode,
      });
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
      const res = await webFetch(`${apiBase}/auth/request-code`, {
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

  const submitPassword = async () => {
    setLoading(true);
    setMessage(null);
    try {
      const res = await webFetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email, password }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data?.error || JSON.stringify(data));
      if (data?.authenticated) {
        await postLoginRedirect();
        return;
      }
      if (!data?.mfa_required || !data?.auth_transaction) {
        throw new Error("予期せぬレスポンスです");
      }
      const methods: string[] = Array.isArray(data.methods) ? data.methods : [];
      setAuthTransaction(data.auth_transaction);
      setPassword("");
      setAvailableMethods(methods);
      setEmailCodeAvailable(methods.includes("email_code"));
      setTotpAvailable(methods.includes("totp"));
      setPasskeyAvailable(false);
      if (methods.length > 1) setStep("select_mfa");
      else if (methods.includes("totp") || methods.includes("recovery"))
        setStep("totp");
      else if (methods.includes("email_code")) setStep("mail_code");
      else throw new Error("利用できる認証方法がありません");
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
    } finally {
      setLoading(false);
    }
  };

  // Perform final login call with the short-lived transaction, never the password.
  const login = async () => {
    setLoading(true);
    setMessage(null);
    try {
      const res = await webFetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({
          email: authTransaction ? undefined : email,
          auth_transaction: authTransaction || undefined,
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
      if (data?.authenticated) {
        setAuthTransaction("");
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
      const beginRes = await webFetch(`${apiBase}/webauthn/login/begin`, {
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
      const finishRes = await webFetch(`${apiBase}/webauthn/login/finish`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify(payload),
      });
      const data = await finishRes.json();
      if (!finishRes.ok) throw new Error(data?.error || "passkey login failed");
      if (data?.authenticated) {
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
          {/* DispLoginBegin */}
          {step === "begin" && (
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

          {/* DispPasswordInput */}
          {step === "password" && hasPassword === true ? (
            <div className="space-y-2">
              <Label htmlFor="password">パスワード</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
              />
              <div className="flex gap-2">
                <Button
                  type="button"
                  onClick={submitPassword}
                  disabled={loading || !email || !password}
                >
                  次へ
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => setStep("begin")}
                >
                  戻る
                </Button>
              </div>
            </div>
          ) : null}

          {/* DispSelectMfaMethod (password disabled with multiple methods) */}
          {step === "select_mfa" && (
            <div className="space-y-4">
              <div className="space-y-1">
                <Label>認証方法を選択</Label>
                <div className="flex flex-wrap gap-2">
                  {/* Email code option */}
                  {emailCodeAvailable && (
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={async () => {
                        await requestCode();
                        setStep("mail_code");
                      }}
                      disabled={loading || sending || !email}
                    >
                      {sending ? "送信中…" : "メールコード"}
                    </Button>
                  )}
                  {passkeyAvailable && (
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setStep("passkey")}
                    >
                      パスキー
                    </Button>
                  )}
                  {totpAvailable && (
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setStep("totp")}
                    >
                      TOTP
                    </Button>
                  )}
                </div>
              </div>
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => setStep(hasPassword ? "password" : "begin")}
                >
                  戻る
                </Button>
              </div>
            </div>
          )}

          {/* DispTotpInput */}
          {step === "totp" && (
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
              <div className="col-span-2 flex gap-2">
                <Button
                  type="button"
                  onClick={() => login()}
                  disabled={
                    loading || !email
                  }
                >
                  続行
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() =>
                    setStep(hasPassword ? "password" : "select_mfa")
                  }
                >
                  戻る
                </Button>
              </div>
            </div>
          )}
          {/* DispMailCodeInput */}
          {step === "mail_code" && (
            <div className="space-y-3">
              <div className="space-y-2">
                <Label htmlFor="code">メールコード</Label>
                <Input
                  id="code"
                  name="code"
                  type="text"
                  inputMode="numeric"
                  value={code}
                  onChange={(e) =>
                    setCode(e.target.value.replace(/\D/g, "").slice(0, 6))
                  }
                  placeholder="123456"
                  autoComplete="one-time-code"
                  minLength={6}
                  maxLength={6}
                  pattern="[0-9]{6}"
                />
              </div>
              <div className="flex gap-2">
                <Button
                  type="button"
                  onClick={() => login()}
                  disabled={
                    loading ||
                    !email ||
                    code.length !== 6
                  }
                >
                  続行
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={requestCode}
                  disabled={sending}
                >
                  {sending ? "再送中…" : "コードを再送"}
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() =>
                    setStep(
                      availableMethods.length > 1 ? "select_mfa" : "begin"
                    )
                  }
                >
                  戻る
                </Button>
              </div>
            </div>
          )}

          {/* DispPasskeyInput */}
          {step === "passkey" && (
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                パスキーでログインを行います。
              </p>
              <div className="flex gap-2">
                <Button
                  type="button"
                  onClick={passkeyLogin}
                  disabled={loading || !email}
                >
                  パスキーで続行
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() =>
                    setStep(hasPassword ? "select_mfa" : "select_mfa")
                  }
                >
                  戻る
                </Button>
              </div>
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

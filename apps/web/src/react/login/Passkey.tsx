import * as React from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../components/ui/card";
import { Button } from "../../components/ui/button";
import {
  apiBase,
  getEmailFromQuery,
  arrayToB64url,
  b64urlToArray,
  postLoginRedirect,
  withNext,
  clearAuthTransaction,
} from "./helpers";

export default function Passkey() {
  const [email] = React.useState(getEmailFromQuery());
  const [loading, setLoading] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);

  const loginPasskey = async () => {
    setMessage(null);
    setLoading(true);
    try {
      const beginRes = await fetch(`${apiBase}/webauthn/login/begin`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email }),
      });
      const beginText = await beginRes.text();
      if (!beginRes.ok) {
        try {
          const err = JSON.parse(beginText);
          if (err?.error === "not found") {
            throw new Error("このメールアドレスのユーザーは存在しません");
          }
        } catch {
          // ignore json parse error
        }
        throw new Error(beginText || "エラーが発生しました");
      }
      const raw = beginText ? JSON.parse(beginText) : {};
      // go-webauthn may respond with { publicKey: {...} }
      const pubkey = raw?.publicKey ?? raw ?? {};

      const toBuf = (val: any) => {
        if (!val) return undefined as any;
        if (typeof val === "string") return b64urlToArray(val);
        if (Array.isArray(val)) return new Uint8Array(val);
        if (val?.type === "Buffer" && Array.isArray(val.data))
          return new Uint8Array(val.data);
        return val; // assume already a buffer-like
      };

      const options: any = {
        ...pubkey,
        challenge: toBuf(pubkey.challenge),
      };
      if (Array.isArray(pubkey.allowCredentials)) {
        options.allowCredentials = pubkey.allowCredentials.map((c: any) => ({
          ...c,
          id: toBuf(c?.id),
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
      if (!finishRes.ok) {
        if (data?.error === "user_not_found") {
          throw new Error("このメールアドレスのユーザーは存在しません");
        }
        throw new Error(data?.error || "passkey login failed");
      }
      if (data?.token) {
        clearAuthTransaction();
        localStorage.setItem("api_token", data.token);
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

  // Auto-invoke on mount for smoother UX
  React.useEffect(() => {
    // Avoid double calls if already loading
    if (!loading) {
      // Fire and forget; errors will be shown via state
      loginPasskey();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <Card>
      <CardHeader>
        <CardTitle>パスキー</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">
          パスキーでログインを行います。
        </p>
        <div className="flex gap-2">
          <Button type="button" onClick={loginPasskey} disabled={loading}>
            パスキーで続行
          </Button>
          <Button
            type="button"
            variant="secondary"
            onClick={() =>
              (window.location.href = withNext(
                `/login/select?email=${encodeURIComponent(email)}`
              ))
            }
          >
            戻る
          </Button>
        </div>
        {message && <p className="text-sm text-muted-foreground">{message}</p>}
      </CardContent>
    </Card>
  );
}

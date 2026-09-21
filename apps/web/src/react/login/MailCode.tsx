import * as React from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../components/ui/card";
import { Label } from "../../components/ui/label";
import { Input } from "../../components/ui/input";
import { Button } from "../../components/ui/button";
import {
  apiBase,
  getEmailFromQuery,
  postLoginRedirect,
  withNext,
} from "./helpers";
import {
  readMailCodeStatus,
  requestMailCode,
  secondsUntilResend,
  DEFAULT_RESEND_COOLDOWN_SEC,
} from "./flow";

export default function MailCode() {
  const [email] = React.useState(getEmailFromQuery());
  const [code, setCode] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [sending, setSending] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);
  const [remain, setRemain] = React.useState(0);

  React.useEffect(() => {
    // Reflect the status if the code was requested on the previous page
    const st = readMailCodeStatus(email);
    if (st?.ok) {
      setMessage("確認コードを送信しました。メールをご確認ください。");
    } else if (st?.error) {
      setMessage(st.error);
    }
    // Setup cooldown countdown
    const update = () => setRemain(secondsUntilResend(email));
    update();
    const t = setInterval(update, 1000);
    // Auto-send code on first render if allowed (no cooldown and not already sent)
    (async () => {
      const canSendNow = secondsUntilResend(email) <= 0 && !st?.ok;
      if (email && canSendNow) {
        try {
          setSending(true);
          setMessage(null);
          const r = await requestMailCode(email);
          if (r.ok) {
            setMessage("確認コードを送信しました。メールをご確認ください。");
            setRemain(DEFAULT_RESEND_COOLDOWN_SEC);
          } else if (r.error) {
            setMessage(r.error);
          }
        } catch (e: any) {
          setMessage(e.message || "エラーが発生しました");
        } finally {
          setSending(false);
        }
      }
    })();
    return () => clearInterval(t);
  }, [email]);

  const sendCode = async () => {
    if (remain > 0) return;
    setSending(true);
    setMessage(null);
    try {
      const r = await requestMailCode(email);
      if (!r.ok) throw new Error(r.error || "送信に失敗しました");
      setMessage("確認コードを送信しました。メールをご確認ください。");
      setRemain(DEFAULT_RESEND_COOLDOWN_SEC);
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
    } finally {
      setSending(false);
    }
  };

  const doLogin = async () => {
    setLoading(true);
    setMessage(null);
    try {
      const password = sessionStorage.getItem("login_pw") || undefined;
      const res = await fetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email, password, code }),
      });
      const data = await res.json();
      if (!res.ok) {
        if (data?.error === "user_not_found") {
          throw new Error("このメールアドレスのユーザーは存在しません");
        }
        throw new Error(data?.error || JSON.stringify(data));
      }
      sessionStorage.removeItem("login_pw");
      if (data?.token) {
        localStorage.setItem("api_token", data.token);
        // Keep loading=true so the button stays disabled until the redirect
        setMessage("認証しました。リダイレクトしています…");
        await postLoginRedirect();
        return;
      }
      setMessage("予期せぬレスポンスです");
      setLoading(false);
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
      setLoading(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>メールコード</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
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
            onClick={doLogin}
            disabled={loading || !email || code.length !== 6}
          >
            {loading ? "認証中…" : "続行"}
          </Button>
          <Button
            type="button"
            variant="secondary"
            onClick={sendCode}
            disabled={sending || remain > 0}
          >
            {sending
              ? "再送中…"
              : remain > 0
              ? `再送まで ${remain}秒`
              : "コードを送信"}
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

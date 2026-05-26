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
  withNext,
  postLoginRedirect,
} from "./helpers";

export default function Password() {
  const [email] = React.useState(getEmailFromQuery());
  const [password, setPassword] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);

  const nextStep = async () => {
    setMessage(null);
    if (!email || !password) return;
    setLoading(true);
    try {
      const res = await fetch(`${apiBase}/auth/options`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email }),
      });
      const text = await res.text();
      if (!res.ok) {
        try {
          const err = JSON.parse(text);
          if (err?.error === "user_not_found") {
            throw new Error("このメールアドレスのユーザーは存在しません");
          }
        } catch {
          // ignore json parse error
        }
        throw new Error(text || "エラーが発生しました");
      }
      const data = text ? JSON.parse(text) : {};
      const methods: string[] = Array.isArray(data?.methods)
        ? data.methods
        : [];
      const passkey = !!data?.passkey_enabled || methods.includes("passkey");
      const emailCode = methods.includes("email_code");
      const totp = !!data?.totp_enabled;
      const recovery = methods.includes("recovery");
      const enabled = [
        passkey ? "passkey" : null,
        emailCode ? "mail" : null,
        totp ? "totp" : null,
        recovery ? "recovery" : null,
      ].filter(Boolean) as string[];
      if (enabled.length === 0) {
        // No MFA configured: password-only login
        await doLogin();
        return;
      }
      // MFA required: persist password temporarily for the next step
      sessionStorage.setItem("login_pw", password);
      const qs = `?email=${encodeURIComponent(email)}`;
      if (enabled.length > 1) {
        window.location.assign(withNext(`/login/select${qs}`));
        return;
      }
      if (passkey) {
        window.location.assign(withNext(`/login/passkey${qs}`));
      } else if (totp) {
        window.location.assign(withNext(`/login/totp${qs}`));
      } else if (emailCode) {
        window.location.assign(withNext(`/login/mail-code${qs}`));
      } else if (recovery) {
        window.location.assign(withNext(`/login/recovery${qs}`));
      }
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
    } finally {
      setLoading(false);
    }
  };

  const doLogin = async () => {
    setMessage(null);
    setLoading(true);
    try {
      const res = await fetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email, password }),
      });
      const data = await res.json();
      if (!res.ok) {
        if (data?.error === "user_not_found") {
          throw new Error("このメールアドレスのユーザーは存在しません");
        }
        throw new Error(data?.error || JSON.stringify(data));
      }
      if (data?.token) {
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

  return (
    <Card>
      <CardHeader>
        <CardTitle>パスワード</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="password">パスワード</Label>
          <Input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="••••••••"
            autoComplete="current-password"
          />
          <div className="flex gap-2">
            <Button
              type="button"
              onClick={nextStep}
              disabled={loading || !email || !password}
            >
              次へ
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={() => (window.location.href = withNext(`/login`))}
            >
              戻る
            </Button>
          </div>
        </div>
        {message && <p className="text-sm text-muted-foreground">{message}</p>}
      </CardContent>
    </Card>
  );
}

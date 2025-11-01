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
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json();
      const passkey =
        !!data?.passkey_enabled ||
        (Array.isArray(data?.methods) && data.methods.includes("passkey"));
      const totp = !!data?.totp_enabled;
      if (!totp) {
        // No MFA required: perform password-only login now
        await doLogin();
        return;
      }
      // MFA required: persist password temporarily and move to TOTP or selection
      sessionStorage.setItem("login_pw", password);
      const qs = `?email=${encodeURIComponent(email)}`;
      if (passkey) {
        window.location.assign(withNext(`/login/select${qs}`));
      } else {
        window.location.assign(withNext(`/login/totp${qs}`));
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
      if (!res.ok) throw new Error(data?.error || JSON.stringify(data));
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

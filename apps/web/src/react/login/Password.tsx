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
  saveAuthTransaction,
  webFetch,
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
      // Verify the password server-side first. The server only answers
      // "mfa required" after the password check has passed.
      const loginRes = await webFetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ email, password }),
      });
      const loginData = await loginRes.json().catch(() => ({}));
      if (loginRes.ok && loginData?.mfa_required) {
        const transactionID = loginData?.auth_transaction;
        const methods: string[] = Array.isArray(loginData?.methods)
          ? loginData.methods
          : [];
        if (!transactionID || methods.length === 0) {
          throw new Error("認証を開始できませんでした");
        }
        saveAuthTransaction(
          transactionID,
          email,
          Number(loginData?.expires_in) || 300,
          methods
        );
        setPassword("");
        const emailCode = methods.includes("email_code");
        const totp = methods.includes("totp");
        const recovery = methods.includes("recovery");
        const qs = `?email=${encodeURIComponent(email)}`;
        if (methods.length > 1) {
          window.location.assign(withNext(`/login/select${qs}`));
        } else if (totp) {
          window.location.assign(withNext(`/login/totp${qs}`));
        } else if (emailCode) {
          window.location.assign(withNext(`/login/mail-code${qs}`));
        } else if (recovery) {
          window.location.assign(withNext(`/login/recovery${qs}`));
        } else {
          setMessage("利用できる認証方法がありません");
        }
        return;
      }
      if (loginRes.ok) {
        // Password-only account: logged in without an MFA step
        if (loginData?.authenticated) {
          await postLoginRedirect();
          return;
        }
        setMessage("予期せぬレスポンスです");
        return;
      }
      if (loginData?.error === "user_not_found") {
        throw new Error("このメールアドレスのユーザーは存在しません");
      }
      if (loginData?.error === "invalid password") {
        throw new Error("パスワードが正しくありません");
      }
      throw new Error(loginData?.error || "エラーが発生しました");
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

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
import { apiBase, clearAuthTransaction, withNext } from "./helpers";

export default function Begin() {
  const [email, setEmail] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);

  const go = async () => {
    setMessage(null);
    clearAuthTransaction();
    if (!email) return;
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
      const hasPw = !!data?.has_password;
      const methods: string[] = Array.isArray(data?.methods)
        ? data.methods
        : [];
      const passkey = !!data?.passkey_enabled || methods.includes("passkey");
      const emailCode = methods.includes("email_code");
      const totp = !!data?.totp_enabled;
      const recovery = methods.includes("recovery");

      const qs = `?email=${encodeURIComponent(email)}`;
      if (hasPw) {
        window.location.assign(withNext(`/login/password${qs}`));
        return;
      }
      const enabled = [
        passkey ? "passkey" : null,
        emailCode ? "mail" : null,
        totp ? "totp" : null,
        recovery ? "recovery" : null,
      ].filter(Boolean) as string[];
      if (enabled.length <= 1) {
        // Direct route when only one method is enabled
        if (passkey) {
          window.location.assign(withNext(`/login/passkey${qs}`));
        } else if (emailCode) {
          window.location.assign(withNext(`/login/mail-code${qs}`));
        } else if (totp) {
          window.location.assign(withNext(`/login/totp${qs}`));
        } else if (recovery) {
          window.location.assign(withNext(`/login/recovery${qs}`));
        } else {
          setMessage("このアカウントに使用可能な認証方法がありません");
        }
        return;
      }
      window.location.assign(withNext(`/login/select${qs}`));
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>ログイン</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="email">メールアドレス</Label>
          <Input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            autoComplete="email"
          />
          <div className="flex gap-2">
            <Button type="button" onClick={go} disabled={loading || !email}>
              次へ
            </Button>
          </div>
        </div>
        {message && <p className="text-sm text-muted-foreground">{message}</p>}
      </CardContent>
    </Card>
  );
}

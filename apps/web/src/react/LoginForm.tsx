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

const apiBase = import.meta.env.PUBLIC_API_URL || "http://localhost:8080";

export default function LoginForm() {
  const [email, setEmail] = React.useState("");
  const [code, setCode] = React.useState("");
  const [totp, setTotp] = React.useState("");
  const [recovery, setRecovery] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);

  const requestCode = async () => {
    setLoading(true);
    setMessage(null);
    try {
      const res = await fetch(`${apiBase}/auth/request-code`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });
      if (!res.ok) throw new Error(await res.text());
      setMessage("確認コードを送信しました。メールをご確認ください。");
    } catch (e: any) {
      setMessage(e.message || "エラーが発生しました");
    } finally {
      setLoading(false);
    }
  };

  const login = async () => {
    setLoading(true);
    setMessage(null);
    try {
      const res = await fetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email,
          code: code || undefined,
          totp: totp || undefined,
          recovery: recovery || undefined,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data?.error || JSON.stringify(data));
      if (data?.token) {
        localStorage.setItem("api_token", data.token);
        setMessage("ログインしました");
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
          />
          <div className="flex gap-2">
            <Button
              type="button"
              onClick={requestCode}
              disabled={loading || !email}
            >
              コード送信
            </Button>
          </div>
        </div>
        <div className="grid grid-cols-3 gap-2">
          <div className="space-y-2 col-span-1">
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
          <div className="space-y-2 col-span-1">
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
          <div className="space-y-2 col-span-1">
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
        <div className="flex gap-2">
          <Button type="button" onClick={login} disabled={loading || !email}>
            ログイン
          </Button>
        </div>
        {message && <p className="text-sm text-muted-foreground">{message}</p>}
      </CardContent>
    </Card>
  );
}

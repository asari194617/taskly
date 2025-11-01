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

export default function Totp() {
  const [email] = React.useState(getEmailFromQuery());
  const [totp, setTotp] = React.useState("");
  const [recovery, setRecovery] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);

  const doLogin = async () => {
    setMessage(null);
    setLoading(true);
    try {
      const password = sessionStorage.getItem("login_pw") || undefined;
      const res = await fetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({
          email,
          password,
          totp: totp || undefined,
          recovery_code: recovery || undefined,
        }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data?.error || JSON.stringify(data));
      sessionStorage.removeItem("login_pw");
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
        <CardTitle>TOTP 認証</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
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
        <div className="flex gap-2">
          <Button type="button" onClick={doLogin} disabled={loading || !email}>
            続行
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

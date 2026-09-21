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
  clearAuthTransaction,
  isAuthTransactionError,
  readAuthTransaction,
  withNext,
} from "./helpers";

export default function Totp() {
  const [email] = React.useState(getEmailFromQuery());
  const [totp, setTotp] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);

  const isValidTotp = React.useMemo(
    () => /^(\d{6})$/.test(totp.trim()),
    [totp]
  );

  const doLogin = async () => {
    setMessage(null);
    setLoading(true);
    try {
      const transaction = readAuthTransaction(email);
      const res = await fetch(`${apiBase}/auth/login`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({
          email: transaction ? undefined : email,
          auth_transaction: transaction?.id,
          totp: totp || undefined,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        if (data?.error === "user_not_found") {
          throw new Error("このメールアドレスのユーザーは存在しません");
        }
        if (isAuthTransactionError(data?.error)) {
          clearAuthTransaction();
          throw new Error(
            "認証の有効期限が切れました。パスワードからやり直してください"
          );
        }
        throw new Error(data?.error || JSON.stringify(data));
      }
      clearAuthTransaction();
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
        <div className="space-y-2">
          <Label htmlFor="totp">TOTP</Label>
          <Input
            id="totp"
            type="text"
            inputMode="numeric"
            value={totp}
            onChange={(e) => {
              // accept only digits, max 6
              const next = e.target.value.replace(/\D/g, "").slice(0, 6);
              setTotp(next);
            }}
            onKeyDown={async (e) => {
              if (e.key === "Enter" && !loading && email && isValidTotp) {
                e.preventDefault();
                await doLogin();
              }
            }}
            placeholder="123456"
            autoComplete="one-time-code"
          />
        </div>
        <div className="flex gap-2">
          <Button
            type="button"
            onClick={doLogin}
            disabled={loading || !email || !isValidTotp}
          >
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

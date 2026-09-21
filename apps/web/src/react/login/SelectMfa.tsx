import * as React from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../components/ui/card";
import { Label } from "../../components/ui/label";
import { Button } from "../../components/ui/button";
import {
  apiBase,
  getEmailFromQuery,
  readAuthTransaction,
  withNext,
  webFetch,
} from "./helpers";
import { requestMailCode } from "./flow";

export default function SelectMfa() {
  const [email] = React.useState(getEmailFromQuery());
  const [loading, setLoading] = React.useState(false);
  const [sendingMail, setSendingMail] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);
  const [options, setOptions] = React.useState<{
    passkey: boolean;
    emailCode: boolean;
    totp: boolean;
    recovery: boolean;
  }>({ passkey: false, emailCode: false, totp: false, recovery: false });

  React.useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const transaction = readAuthTransaction(email);
        if (transaction) {
          setOptions({
            passkey: false,
            emailCode: transaction.methods.includes("email_code"),
            totp: transaction.methods.includes("totp"),
            recovery: transaction.methods.includes("recovery"),
          });
          return;
        }
        const res = await webFetch(`${apiBase}/auth/options`, {
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
        if (data?.has_password) {
          setMessage(
            "認証の有効期限が切れました。パスワードからやり直してください",
          );
          window.location.replace(
            withNext(`/login/password?email=${encodeURIComponent(email)}`),
          );
          return;
        }
        const passkey =
          !!data?.passkey_enabled ||
          (Array.isArray(data?.methods) && data.methods.includes("passkey"));
        const emailCode =
          Array.isArray(data?.methods) && data.methods.includes("email_code");
        const totp = !!data?.totp_enabled;
        const recovery =
          Array.isArray(data?.methods) && data.methods.includes("recovery");
        setOptions({
          passkey,
          emailCode: emailCode,
          totp,
          recovery,
        });
      } catch (e: unknown) {
        setMessage(e instanceof Error ? e.message : "エラーが発生しました");
      } finally {
        setLoading(false);
      }
    })();
  }, [email]);

  const qs = `?email=${encodeURIComponent(email)}`;

  return (
    <Card>
      <CardHeader>
        <CardTitle>認証方法を選択</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          <Label>利用する認証方法を選んでください</Label>
          <div className="flex flex-col gap-2">
            {options.emailCode && (
              <Button
                type="button"
                className="w-full justify-start h-12"
                onClick={async () => {
                  setSendingMail(true);
                  setMessage(
                    "メールを送信しています。しばらくお待ちください。",
                  );
                  try {
                    const result = await requestMailCode(email);
                    if (!result.ok) {
                      throw new Error(
                        result.error || "メールの送信に失敗しました",
                      );
                    }
                    window.location.href = withNext(`/login/mail-code${qs}`);
                  } catch (e: unknown) {
                    setMessage(
                      e instanceof Error
                        ? e.message
                        : "メールの送信に失敗しました",
                    );
                  } finally {
                    setSendingMail(false);
                  }
                }}
                disabled={loading || sendingMail}
                variant="secondary"
              >
                {sendingMail ? "メールを送信中…" : "メールコードで続行"}
              </Button>
            )}
            {options.passkey && (
              <Button
                type="button"
                className="w-full justify-start h-12"
                onClick={() =>
                  (window.location.href = withNext(`/login/passkey${qs}`))
                }
                disabled={loading || sendingMail}
                variant="secondary"
              >
                パスキーで続行
              </Button>
            )}
            {options.totp && (
              <Button
                type="button"
                className="w-full justify-start h-12"
                onClick={() =>
                  (window.location.href = withNext(`/login/totp${qs}`))
                }
                disabled={loading || sendingMail}
                variant="secondary"
              >
                認証コード（TOTP）で続行
              </Button>
            )}
            {options.recovery && (
              <Button
                type="button"
                className="w-full justify-start h-12"
                onClick={() =>
                  (window.location.href = withNext(`/login/recovery${qs}`))
                }
                disabled={loading || sendingMail}
                variant="secondary"
              >
                リカバリーコードで続行
              </Button>
            )}
          </div>
        </div>
        <div className="flex gap-2">
          <Button
            type="button"
            variant="secondary"
            onClick={() => (window.location.href = withNext(`/login`))}
          >
            戻る
          </Button>
        </div>
        {message && <p className="text-sm text-muted-foreground">{message}</p>}
      </CardContent>
    </Card>
  );
}

import * as React from "react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../components/ui/card";
import { Label } from "../../components/ui/label";
import { Button } from "../../components/ui/button";
import { apiBase, getEmailFromQuery, withNext } from "./helpers";
import { requestMailCode } from "./flow";

export default function SelectMfa() {
  const [email] = React.useState(getEmailFromQuery());
  const [loading, setLoading] = React.useState(false);
  const [message, setMessage] = React.useState<string | null>(null);
  const [options, setOptions] = React.useState<{
    passkey: boolean;
    emailCode: boolean;
    totp: boolean;
  }>({ passkey: false, emailCode: false, totp: false });

  React.useEffect(() => {
    (async () => {
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
        const emailCode =
          Array.isArray(data?.methods) && data.methods.includes("email_code");
        const totp = !!data?.totp_enabled;
        // If password was entered earlier, email code should not be offered
        const pw = sessionStorage.getItem("login_pw") || "";
        setOptions({ passkey, emailCode: pw ? false : emailCode, totp });
      } catch (e: any) {
        setMessage(e.message || "エラーが発生しました");
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
                  setLoading(true);
                  try {
                    await requestMailCode(email);
                  } finally {
                    window.location.href = withNext(`/login/mail-code${qs}`);
                  }
                }}
                disabled={loading}
                variant="secondary"
              >
                メールコードで続行
              </Button>
            )}
            {options.passkey && (
              <Button
                type="button"
                className="w-full justify-start h-12"
                onClick={() =>
                  (window.location.href = withNext(`/login/passkey${qs}`))
                }
                disabled={loading}
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
                disabled={loading}
                variant="secondary"
              >
                認証コード（TOTP）で続行
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

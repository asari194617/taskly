"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { useLang } from "@/react/LanguageProvider";

const apiBase =
  (import.meta as any).env.PUBLIC_API_URL || "http://localhost:8080";

function urlB64ToUint8Array(base64String: string) {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const rawData = atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; ++i) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

export default function SettingsNotifications() {
  const { dict } = useLang();
  const [emailOn, setEmailOn] = React.useState(true);
  const [pushOn, setPushOn] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [perm, setPerm] = React.useState<NotificationPermission>(
    typeof Notification !== "undefined" ? Notification.permission : "default"
  );
  const [message, setMessage] = React.useState<string | null>(null);
  const [dailyEnabled, setDailyEnabled] = React.useState<boolean>(true);

  React.useEffect(() => {
    // Load push settings
    (async () => {
      try {
        const token = localStorage.getItem("api_token") || "";
        const res = await fetch(`${apiBase}/settings/push`, {
          headers: {
            Authorization: token ? `Bearer ${token}` : "",
            "X-Client-Platform": "web",
          },
        });
        if (res.ok) {
          const data = await res.json();
          setDailyEnabled(!!data?.daily_enabled);
        }
      } catch {}
    })();
  }, []);
  React.useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        if (!("serviceWorker" in navigator)) return;
        const reg = await navigator.serviceWorker.ready;
        const sub = await reg.pushManager.getSubscription();
        if (!mounted) return;
        setPushOn(!!sub);
        setPerm(
          typeof Notification !== "undefined"
            ? Notification.permission
            : "default"
        );
      } catch {}
    })();
    return () => {
      mounted = false;
    };
  }, []);

  const fetchVapidKey = async (): Promise<string> => {
    const res = await fetch(`${apiBase}/push/vapidPublicKey`);
    if (!res.ok) throw new Error("VAPIDキーの取得に失敗しました");
    const data = await res.json();
    if (!data?.publicKey) throw new Error("VAPIDキーが空です");
    return data.publicKey as string;
  };

  const subscribe = async () => {
    setMessage(null);
    try {
      if (!("serviceWorker" in navigator))
        throw new Error(dict.settingsNotifications.errorSwUnsupported);
      if (!("PushManager" in window))
        throw new Error(dict.settingsNotifications.errorPushUnsupported);
      // Ensure SW registered
      const swReg = await navigator.serviceWorker.ready;
      let permission: NotificationPermission =
        typeof Notification !== "undefined"
          ? Notification.permission
          : "default";
      if (permission === "default") {
        permission = await Notification.requestPermission();
        setPerm(permission);
      }
      if (permission !== "granted")
        throw new Error(dict.settingsNotifications.errorNotAllowed);
      // Ensure previous subscription is cleared (especially if server key changed)
      const existing = await swReg.pushManager.getSubscription();
      if (existing) {
        try {
          await existing.unsubscribe();
        } catch {}
      }
      const vapid = await fetchVapidKey();
      const appServerKey = urlB64ToUint8Array(vapid);
      let sub: PushSubscription | null = null;
      try {
        sub = await swReg.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: appServerKey,
        });
      } catch (err: any) {
        // Retry once after forcing unsubscribe, handles InvalidAccessError with stale key
        if (String(err?.name || err).includes("InvalidAccessError")) {
          const s2 = await swReg.pushManager.getSubscription();
          if (s2) {
            try {
              await s2.unsubscribe();
            } catch {}
          }
          sub = await swReg.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: appServerKey,
          });
        } else {
          throw err;
        }
      }
      if (!sub) throw new Error("subscription failed");
      const key = sub.toJSON() as any;
      const token = localStorage.getItem("api_token") || "";
      const res = await fetch(`${apiBase}/push/subscribe`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: token ? `Bearer ${token}` : "",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({
          endpoint: sub.endpoint,
          P256dh: key.keys?.p256dh,
          Auth: key.keys?.auth,
        }),
      });
      if (!res.ok)
        throw new Error(dict.settingsNotifications.errorServerRegister);
      setPushOn(true);
      setMessage(dict.settingsNotifications.enabledMsg);
    } catch (e: any) {
      setMessage(e.message || dict.settingsNotifications.errorGeneric);
      throw e;
    }
  };

  const unsubscribe = async () => {
    setMessage(null);
    try {
      if (!("serviceWorker" in navigator)) return;
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.getSubscription();
      if (sub) {
        const endpoint = sub.endpoint;
        await sub.unsubscribe();
        const token = localStorage.getItem("api_token") || "";
        await fetch(`${apiBase}/push/unsubscribe`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: token ? `Bearer ${token}` : "",
            "X-Client-Platform": "web",
          },
          body: JSON.stringify({ endpoint }),
        });
      }
      setPushOn(false);
      setMessage(dict.settingsNotifications.disabledMsg);
    } catch (e: any) {
      setMessage(e.message || dict.settingsNotifications.errorGeneric);
    }
  };

  const togglePush = async (next: boolean) => {
    setLoading(true);
    try {
      if (next) await subscribe();
      else await unsubscribe();
    } finally {
      setLoading(false);
    }
  };

  const testPush = async () => {
    setMessage(null);
    try {
      const token = localStorage.getItem("api_token") || "";
      const res = await fetch(`${apiBase}/push/test`, {
        method: "POST",
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
          "X-Client-Platform": "web",
        },
      });
      if (!res.ok) throw new Error(dict.settingsNotifications.errorTestSend);
      const data = await res.json().catch(() => ({} as any));
      const sent = typeof data?.sent === "number" ? data.sent : undefined;
      if (typeof sent === "number") {
        const templ = dict.settingsNotifications.testSent;
        setMessage(templ.replace("{n}", String(sent)));
        if (sent === 0) {
          setMessage(dict.settingsNotifications.testSentZeroHint);
        }
      } else {
        setMessage(dict.settingsNotifications.sendTest);
      }
    } catch (e: any) {
      setMessage(e.message || dict.settingsNotifications.errorGeneric);
    }
  };

  const checkSubscriptions = async () => {
    setMessage(null);
    try {
      const token = localStorage.getItem("api_token") || "";
      const res = await fetch(`${apiBase}/push/subscriptions`, {
        headers: {
          Authorization: token ? `Bearer ${token}` : "",
          "X-Client-Platform": "web",
        },
      });
      if (!res.ok) throw new Error(dict.settingsNotifications.errorFetchSubs);
      const data = await res.json();
      const count = data?.count ?? 0;
      const templ = dict.settingsNotifications.subsCount;
      setMessage(templ.replace("{n}", String(count)));
    } catch (e: any) {
      setMessage(e.message || dict.settingsNotifications.errorGeneric);
    }
  };

  const saveDaily = async (next: boolean) => {
    setDailyEnabled(next);
    try {
      const token = localStorage.getItem("api_token") || "";
      await fetch(`${apiBase}/settings/push`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: token ? `Bearer ${token}` : "",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ daily_enabled: next }),
      });
    } catch {}
  };

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">
          {dict.settingsNotifications.heading}
        </h1>
        <Card className="p-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>{dict.settingsNotifications.emailDesc}</div>
            <Switch checked={emailOn} onCheckedChange={setEmailOn} />
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <div className="font-medium">
                {dict.settingsNotifications.webPush}
              </div>
              <div className="text-sm text-muted-foreground">
                <div className="flex items-center justify-between pt-2 border-t border-border/50">
                  <div>
                    <div className="text-sm">
                      {dict.settingsNotifications.daily}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {dict.settingsNotifications.serverDefault}:{" "}
                      {import.meta.env.PUBLIC_PUSH_DIGEST_TIME || "07:45"}
                    </div>
                  </div>
                  <Switch checked={dailyEnabled} onCheckedChange={saveDaily} />
                </div>
                {dict.settingsNotifications.emailDesc}
              </div>
              {perm === "denied" && (
                <div className="text-xs text-red-500 mt-1">
                  {dict.settingsNotifications.blocked}
                </div>
              )}
              {message && (
                <div className="text-xs text-muted-foreground mt-1">
                  {message}
                </div>
              )}
            </div>
            <Switch
              checked={pushOn}
              onCheckedChange={togglePush}
              disabled={loading || perm === "denied"}
            />
          </div>
          <div className="flex gap-2 justify-end">
            <Button
              variant="secondary"
              onClick={testPush}
              disabled={!pushOn || loading}
            >
              {dict.settingsNotifications.sendTest}
            </Button>
            <Button
              variant="secondary"
              onClick={checkSubscriptions}
              disabled={loading}
            >
              {dict.settingsNotifications.checkSubs}
            </Button>
            {!pushOn && perm !== "granted" && (
              <Button
                variant="secondary"
                onClick={() => subscribe()}
                disabled={loading}
              >
                {dict.settingsNotifications.enableAndAllow}
              </Button>
            )}
          </div>
        </Card>
      </div>
    </SettingsShell>
  );
}

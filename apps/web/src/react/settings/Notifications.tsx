"use client";
import { AlertCircleIcon } from "lucide-react";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { useLang } from "@/react/LanguageProvider";
import { toast } from "@/hooks/use-toast";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { webFetch } from "@/lib/api";
import { applyNotificationSetting, saveNotificationSetting } from "@/features/notifications/settings";

const apiBase =
  (import.meta as any).env.PUBLIC_API_URL || "http://localhost:3063";

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

function applicationServerKeysMatch(
  subscription: PushSubscription,
  expected: Uint8Array
) {
  const current = subscription.options.applicationServerKey;
  if (!current) return false;
  const bytes = new Uint8Array(current);
  return (
    bytes.length === expected.length &&
    bytes.every((value, index) => value === expected[index])
  );
}

export default function SettingsNotifications() {
  const { dict } = useLang();
  const [isOpenWebPushModal, setIsOpenWebPushModal] = React.useState(false);
  const [emailOn, setEmailOn] = React.useState(true);
  const [pushOn, setPushOn] = React.useState(false);
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [perm, setPerm] = React.useState<NotificationPermission>(
    typeof Notification !== "undefined" ? Notification.permission : "default"
  );
  const [typeTask, setTypeTask] = React.useState<boolean>(true);
  const [typeMorning, setTypeMorning] = React.useState<boolean>(true);
  const [typeEvening, setTypeEvening] = React.useState<boolean>(true);
  const [typeDayShift, setTypeDayShift] = React.useState<boolean>(true);
  const [devices, setDevices] = React.useState<
    Array<{ id: number; name: string; is_enabled: boolean }>
  >([]);
  const vapidKeyRef = React.useRef<Promise<string> | null>(null);

  const loadDevices = React.useCallback(async () => {
    try {
      const res = await webFetch(`${apiBase}/push/devices`, {
        headers: {
          "X-Client-Platform": "web",
        },
      });
      if (!res.ok) return;
      const items = await res.json();
      if (Array.isArray(items)) setDevices(items);
    } catch {}
  }, []);

  React.useEffect(() => {
    (async () => {
      try {
        const h = {
          "X-Client-Platform": "web",
        } as any;
        // methods
        const m = await webFetch(`${apiBase}/settings/notification-methods`, {
          headers: h,
        });
        if (m.ok) {
          const j = await m.json();
          if (typeof j.email === "boolean") setEmailOn(j.email);
          if (typeof j.webpush === "boolean") setPushOn(j.webpush);
        }
        // types
        const t = await webFetch(`${apiBase}/settings/notification-types`, {
          headers: h,
        });
        if (t.ok) {
          const jj = await t.json();
          setTypeTask(jj?.TASK !== false);
          setTypeMorning(jj?.TASK_MORNING !== false);
          setTypeEvening(jj?.TASK_EVENING !== false);
          setTypeDayShift(jj?.DAYSHIFT !== false);
        }
        await loadDevices();
      } catch {}
    })();
  }, [loadDevices]);
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
    const res = await webFetch(`${apiBase}/push/vapidPublicKey`);
    if (!res.ok) throw new Error("VAPIDキーの取得に失敗しました");
    const data = await res.json();
    if (!data?.publicKey) throw new Error("VAPIDキーが空です");
    return data.publicKey as string;
  };

  React.useEffect(() => {
    // Warm the public key while the user reads the page. This keeps the click
    // handler focused on the browser permission/subscription operations.
    vapidKeyRef.current = fetchVapidKey().catch((error) => {
      vapidKeyRef.current = null;
      throw error;
    });
    void vapidKeyRef.current.catch(() => {});
  }, []);

  const subscribe = async () => {
    try {
      if (!("serviceWorker" in navigator))
        throw new Error(dict.settingsNotifications.errorSwUnsupported);
      if (!("PushManager" in window))
        throw new Error(dict.settingsNotifications.errorPushUnsupported);
      let permission: NotificationPermission =
        typeof Notification !== "undefined"
          ? Notification.permission
          : "default";
      // Permission must be requested before any network await so browsers keep
      // this operation associated with the user's switch click.
      if (permission === "default") {
        permission = await Notification.requestPermission();
        setPerm(permission);
      }
      if (permission !== "granted")
        throw new Error(dict.settingsNotifications.errorNotAllowed);

      const [swReg, vapid] = await Promise.all([
        navigator.serviceWorker.ready,
        vapidKeyRef.current || fetchVapidKey(),
      ]);
      const appServerKey = urlB64ToUint8Array(vapid);
      const existing = await swReg.pushManager.getSubscription();
      let sub: PushSubscription | null = existing;

      // Reuse a valid browser subscription and re-sync it to the API. Replacing
      // every subscription made the setting fragile and could discard a
      // working subscription before the replacement succeeded.
      if (existing && !applicationServerKeysMatch(existing, appServerKey)) {
        try {
          await existing.unsubscribe();
        } catch {}
        sub = null;
      }

      if (!sub) {
        try {
          sub = await swReg.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: appServerKey,
          });
        } catch (err: any) {
          // Retry once after forcing unsubscribe, handles a stale browser key.
          if (String(err?.name || err).includes("InvalidAccessError")) {
            const stale = await swReg.pushManager.getSubscription();
            if (stale) {
              try {
                await stale.unsubscribe();
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
      }
      if (!sub) throw new Error("subscription failed");
      const key = sub.toJSON() as any;
      const res = await webFetch(`${apiBase}/push/subscribe`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
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
      await loadDevices();
      toast({ title: dict.settingsNotifications.enabledMsg });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: e.message || dict.settingsNotifications.errorGeneric,
      });
      throw e;
    }
  };

  const unsubscribe = async () => {
    if (!("serviceWorker" in navigator)) return;
    const reg = await navigator.serviceWorker.ready;
    const sub = await reg.pushManager.getSubscription();
    if (sub) {
      const res = await webFetch(`${apiBase}/push/unsubscribe`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ endpoint: sub.endpoint }),
      });
      if (!res.ok)
        throw new Error(dict.settingsNotifications.errorServerRegister);
      await sub.unsubscribe();
    }
    setPushOn(false);
    await loadDevices();
    toast({ title: dict.settingsNotifications.disabledMsg });
  };

  const togglePush = async (next: boolean) => {
    setLoading(true);
    try {
      if (next) {
        // Subscribe first so permission and PushManager calls remain tied to
        // the user's click. The subscription endpoint enables this device.
        await subscribe();
      } else {
        await unsubscribe();
      }
    } catch (e: any) {
      if (!next) {
        toast({
          variant: "destructive",
          title: e.message || dict.settingsNotifications.errorGeneric,
        });
      }
    } finally {
      setLoading(false);
    }
  };

  const toggleEmail = async (next: boolean) => {
    setSaving(true);
    try {
      await applyNotificationSetting(emailOn, next, setEmailOn, () =>
        saveNotificationSetting(apiBase, "notification-methods", { email: next }));
    } catch (e: any) {
      toast({ variant: "destructive", title: e?.message || dict.settingsNotifications.errorGeneric });
    } finally {
      setSaving(false);
    }
  };

  const toggleType = async (
    key: "TASK" | "TASK_MORNING" | "TASK_EVENING" | "DAYSHIFT",
    previous: boolean,
    next: boolean,
    publish: (value: boolean) => void,
  ) => {
    setSaving(true);
    try {
      await applyNotificationSetting(previous, next, publish, () =>
        saveNotificationSetting(apiBase, "notification-types", { [key]: next }));
    } catch (e: any) {
      toast({ variant: "destructive", title: e?.message || dict.settingsNotifications.errorGeneric });
    } finally {
      setSaving(false);
    }
  };

  const toggleDevice = async (id: number, next: boolean) => {
    try {
      const res = await webFetch(`${apiBase}/push/devices/${id}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ is_enabled: next }),
      });
      if (res.ok) {
        setDevices((prev) =>
          prev.map((d) => (d.id === id ? { ...d, is_enabled: next } : d))
        );
      }
    } catch {}
  };

  const testPush = async () => {
    try {
      const res = await webFetch(`${apiBase}/push/test`, {
        method: "POST",
        headers: {
          "X-Client-Platform": "web",
        },
      });
      if (!res.ok) throw new Error(dict.settingsNotifications.errorTestSend);
      const data = await res.json().catch(() => ({} as any));
      const sent = typeof data?.sent === "number" ? data.sent : undefined;
      if (typeof sent === "number") {
        const templ = dict.settingsNotifications.testSent;
        toast({ title: templ.replace("{n}", String(sent)) });
        if (sent === 0) {
          toast({ title: dict.settingsNotifications.testSentZeroHint });
        }
      } else {
        toast({ title: dict.settingsNotifications.sendTest });
      }
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: e.message || dict.settingsNotifications.errorGeneric,
      });
    }
  };

  // Removed "check subscriptions" from UI for simplicity.

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="scroll-m-20 text-2xl font-bold tracking-tight text-balance">
          {dict.settingsNotifications.heading}
        </h1>
        <Accordion type="multiple" className="w-full">
          <AccordionItem value="item-1">
            <AccordionTrigger>通知方法</AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <Card className="p-4 space-y-3">
                <div className="font-bold flex items-center justify-between">
                  <div>メール</div>
                  <Switch checked={emailOn} onCheckedChange={toggleEmail} disabled={saving} />
                </div>
              </Card>
              <Card className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <p className="font-bold leading-7 [&:not(:first-child)]:mt-6">
                    Webプッシュ通知
                  </p>
                  <Switch
                    checked={pushOn}
                    onCheckedChange={togglePush}
                    disabled={loading || (perm === "denied" && !pushOn)}
                  />
                </div>
                <div className="flex items-center justify-end">
                  <Dialog
                    open={isOpenWebPushModal}
                    onOpenChange={setIsOpenWebPushModal}
                  >
                    <DialogTrigger asChild>
                      <Button
                        variant="outline"
                        disabled={!pushOn || perm === "denied"}
                      >
                        通知端末の設定
                      </Button>
                    </DialogTrigger>
                    <DialogContent className="sm:max-w-md">
                      <DialogHeader>
                        <DialogTitle>通知端末の設定</DialogTitle>
                        <DialogDescription>
                          Web
                          Push通知設定をしている各端末で、通知を受け取るかどうかを設定できます。
                        </DialogDescription>
                      </DialogHeader>
                      <div className="flex flex-col gap-4">
                        {devices.map((d) => (
                          <Card key={d.id} className="p-4 space-y-3">
                            <div className="flex items-center justify-between">
                              <small className="text-sm leading-none font-medium">
                                {d.name || "Web Push"}
                              </small>
                              <Switch
                                checked={!!d.is_enabled}
                                onCheckedChange={(v) => toggleDevice(d.id, v)}
                              />
                            </div>
                          </Card>
                        ))}
                        {devices.length === 0 && (
                          <div className="text-xs text-muted-foreground">
                            この端末で一度Webプッシュ通知を有効化すると表示されます。
                          </div>
                        )}
                      </div>
                      <DialogFooter className="sm:justify-start">
                        <DialogClose asChild>
                          <Button type="button" variant="secondary">
                            閉じる
                          </Button>
                        </DialogClose>
                      </DialogFooter>
                    </DialogContent>
                  </Dialog>
                </div>
              </Card>
            </AccordionContent>
          </AccordionItem>
          <AccordionItem value="item-2">
            <AccordionTrigger>通知の種類</AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <Card className="p-4 space-y-3">
                {/* 期限が近い課題 */}
                <div className="flex items-center justify-between">
                  <p className="font-bold leading-7 [&:not(:first-child)]:mt-6">
                    期限が近い課題
                  </p>
                  <Switch
                    checked={typeTask}
                    disabled={saving}
                    onCheckedChange={(v) => toggleType("TASK", typeTask, v, setTypeTask)}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <p className="ml-1 leading-7 [&:not(:first-child)]:mt-6">
                    朝の通知
                  </p>
                  <Switch
                    checked={typeMorning}
                    disabled={saving || !typeTask}
                    onCheckedChange={(v) => toggleType("TASK_MORNING", typeMorning, v, setTypeMorning)}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <p className="ml-1 leading-7 [&:not(:first-child)]:mt-6">
                    夕方の通知
                  </p>
                  <Switch
                    checked={typeEvening}
                    disabled={saving || !typeTask}
                    onCheckedChange={(v) => toggleType("TASK_EVENING", typeEvening, v, setTypeEvening)}
                  />
                </div>
              </Card>
              <Card className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <p className="font-bold leading-7 [&:not(:first-child)]:mt-6">
                    日直のお知らせ
                  </p>
                  <Switch
                    checked={typeDayShift}
                    disabled={saving}
                    onCheckedChange={(v) => toggleType("DAYSHIFT", typeDayShift, v, setTypeDayShift)}
                  />
                </div>
              </Card>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
        <Alert>
          <AlertCircleIcon />
          <AlertTitle>
            通知を受け取るには、通知方法と通知の種類の両方が有効である必要があります。
          </AlertTitle>
          <AlertDescription>
            いずれかが無効の場合、配信されません。また、Webプッシュ通知を有効にするには、ブラウザの通知許可設定で「許可」にする必要があります。
          </AlertDescription>
        </Alert>
        <Accordion type="multiple" className="w-full">
          <AccordionItem value="item-3">
            <AccordionTrigger>その他の設定</AccordionTrigger>
            <AccordionContent className="flex flex-col gap-4 text-balance">
              <Card className="p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <p className="font-bold leading-7 [&:not(:first-child)]:mt-6">
                    テスト通知
                  </p>
                  <Button
                    variant="outline"
                    onClick={() => testPush()}
                    disabled={!pushOn || loading}
                  >
                    {dict.settingsNotifications.sendTest}
                  </Button>
                </div>
              </Card>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
        {perm === "denied" && (
          <div className="text-xs text-red-500 mt-1">
            {dict.settingsNotifications.blocked}
          </div>
        )}
      </div>
    </SettingsShell>
  );
}

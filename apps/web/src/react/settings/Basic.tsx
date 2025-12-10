"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { useToast } from "@/hooks/use-toast";
import { getApiBase } from "@/lib/api";
import { gql } from "@/lib/graphql";
import { useLang } from "@/react/LanguageProvider";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export default function SettingsBasic() {
  const [displayName, setDisplayName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [notificationLang, setNotificationLang] = React.useState<"ja" | "en">(
    "ja"
  );
  const [loading, setLoading] = React.useState(false);
  const { toast } = useToast();
  const { dict } = useLang();
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";

  React.useEffect(() => {
    async function fetchMe() {
      try {
        const data = await gql<{
          me: { display_name?: string; email: string };
        }>(`query{ me{ display_name email } }`);
        setDisplayName(data.me.display_name || "");
        setEmail(data.me.email || "");
      } catch {}
    }
    async function fetchLang() {
      try {
        if (!token) return;
        const base = getApiBase();
        const res = await fetch(`${base}/settings/lang`, {
          headers: {
            Authorization: `Bearer ${token}`,
            "X-Client-Platform": "web",
          },
        });
        if (res.ok) {
          const json = await res.json();
          if (
            json?.notification_lang === "ja" ||
            json?.notification_lang === "en"
          ) {
            setNotificationLang(json.notification_lang);
          }
        }
      } catch {}
    }
    if (token) fetchMe();
    if (token) fetchLang();
  }, [token]);

  async function save() {
    setLoading(true);
    try {
      await gql<{ updateMe: boolean }>(
        `mutation($name:String!){ updateMe(display_name:$name) }`,
        { name: displayName }
      );
      // Save notification language via REST
      try {
        const base = getApiBase();
        await fetch(`${base}/settings/lang`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
            "X-Client-Platform": "web",
          },
          body: JSON.stringify({ notification_lang: notificationLang }),
        });
      } catch {}
      toast({ title: "保存しました" });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "保存に失敗しました",
        description: e?.message,
      });
    } finally {
      setLoading(false);
    }
  }

  async function deleteAccount() {
    try {
      await gql<{ deleteMe: boolean }>(`mutation{ deleteMe }`);
      localStorage.removeItem("api_token");
      location.href = "/";
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message || undefined,
      });
    }
  }

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="scroll-m-20 text-2xl font-bold tracking-tight text-balance">
          {dict.settingsBasic.heading}
        </h1>
        <Card className="p-4 space-y-3">
          <div className="space-y-1">
            <Label htmlFor="email">{dict.settingsBasic.email}</Label>
            <Input id="email" value={email} disabled readOnly />
          </div>
          <div className="space-y-1">
            <Label htmlFor="display_name">
              {dict.settingsBasic.displayName}
            </Label>
            <Input
              id="display_name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="notification_lang">
              {dict.settingsBasic.notificationLang}
            </Label>
            <Select
              value={notificationLang}
              onValueChange={(v: "ja" | "en") => setNotificationLang(v)}
            >
              <SelectTrigger id="notification_lang">
                <SelectValue placeholder={dict.tasks.selectPlaceholder} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="ja">{dict.common.japanese}</SelectItem>
                <SelectItem value="en">{dict.common.english}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <Button onClick={save} disabled={loading}>
            {loading ? dict.common.saving : dict.common.save}
          </Button>
        </Card>
        <Card className="p-4 space-y-3 border-destructive/30">
          <div>
            <div className="font-medium text-destructive">
              {dict.settingsBasic.deleteAccount}
            </div>
            <div className="text-sm text-muted-foreground">
              {dict.settingsBasic.irreversible}
            </div>
          </div>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button variant="destructive">
                {dict.settingsBasic.deleteAccount}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {dict.settingsBasic.deleteConfirmTitle}
                </AlertDialogTitle>
                <AlertDialogDescription>
                  {dict.settingsBasic.irreversible}
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>{dict.common.cancel}</AlertDialogCancel>
                <AlertDialogAction
                  className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  onClick={deleteAccount}
                >
                  {dict.common.delete}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </Card>
      </div>
    </SettingsShell>
  );
}

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
import { getApiBase, webFetch } from "@/lib/api";
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
  const authProfileURL = import.meta.env.PUBLIC_AUTH_PROFILE_URL?.trim();
  const [displayName, setDisplayName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [notificationLang, setNotificationLang] = React.useState<"ja" | "en">(
    "ja"
  );
  const [loading, setLoading] = React.useState(false);
  const [closureOpen, setClosureOpen] = React.useState(false);
  const [closingAccount, setClosingAccount] = React.useState(false);
  const { toast } = useToast();
  const { dict } = useLang();

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
        const base = getApiBase();
        const res = await webFetch(`${base}/settings/lang`, {
          headers: {
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
    fetchMe();
    fetchLang();
  }, []);

  async function save() {
    setLoading(true);
    try {
      if (!authProfileURL) {
        await gql<{ updateMe: boolean }>(
          `mutation($name:String!){ updateMe(display_name:$name) }`,
          { name: displayName }
        );
      }
      const base = getApiBase();
      const res = await webFetch(`${base}/settings/lang`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Client-Platform": "web",
        },
        body: JSON.stringify({ notification_lang: notificationLang }),
      });
      if (!res.ok) throw new Error(dict.settingsBasic.emailLanguageSaveFailed);
      toast({ title: dict.settingsBasic.saved });
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.settingsBasic.saveFailed,
        description: e?.message,
      });
    } finally {
      setLoading(false);
    }
  }

  async function deleteAccount() {
    if (closingAccount) return;
    setClosingAccount(true);
    try {
      await gql<{ deleteMe: boolean }>(`mutation{ deleteMe }`);
      location.href = "/";
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.settingsBasic.deleteFailed,
        description: e?.message || undefined,
      });
    } finally {
      setClosingAccount(false);
    }
  }

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="scroll-m-20 text-2xl font-bold tracking-tight text-balance">
          {dict.settingsBasic.heading}
        </h1>
        <Card className="p-4 space-y-3">
          {authProfileURL ? <p><a className="underline" href={authProfileURL}>{dict.settingsBasic.authProfileManage}</a></p> : <>
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
          </>}
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
        <Card className="space-y-4 border-border p-4">
          <div className="space-y-2">
            <h2 className="font-medium">
              {dict.settingsBasic.deleteAccount}
            </h2>
            <p className="text-sm text-muted-foreground">
              {dict.settingsBasic.closureSummary}
            </p>
            <p className="text-sm">
              <a className="text-muted-foreground underline underline-offset-4 hover:text-foreground" href="/account-data">
                {dict.settingsBasic.erasureLink}
              </a>
            </p>
          </div>
          <AlertDialog open={closureOpen} onOpenChange={(open) => {
            if (!closingAccount) setClosureOpen(open);
          }}>
            <AlertDialogTrigger asChild>
              <Button variant="destructive" className="w-fit">
                {dict.settingsBasic.startClosure}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>
                  {dict.settingsBasic.deleteConfirmTitle}
                </AlertDialogTitle>
                <AlertDialogDescription className="text-left">
                  <span className="block">{dict.settingsBasic.closureAccess}</span>
                  <span className="mt-2 block">{dict.settingsBasic.closureData}</span>
                  <a className="mt-2 inline-block" href="/account-data">{dict.settingsBasic.erasureLink}</a>
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel disabled={closingAccount}>{dict.common.cancel}</AlertDialogCancel>
                <AlertDialogAction
                  className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                  disabled={closingAccount}
                  onClick={(event) => {
                    event.preventDefault();
                    void deleteAccount();
                  }}
                >
                  {closingAccount ? dict.settingsBasic.closingAccount : dict.settingsBasic.deleteAccount}
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </Card>
      </div>
    </SettingsShell>
  );
}

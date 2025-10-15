"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer";
import { useMediaQuery } from "@/react/hooks/useMediaQuery";
import { getApiBase } from "@/lib/api";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { useLang } from "@/react/LanguageProvider";

type AuthState = {
  email_login_enabled: boolean;
  has_password: boolean;
  totp_enabled: boolean;
};

export default function SettingsSecurity() {
  const { dict } = useLang();
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [auth, setAuth] = React.useState<AuthState | null>(null);
  const [totpSecret, setTotpSecret] = React.useState<string>("");
  const [totpUri, setTotpUri] = React.useState<string>("");
  const [totpCode, setTotpCode] = React.useState<string>("");
  const [totpOpen, setTotpOpen] = React.useState(false);
  const [totpStep, setTotpStep] = React.useState<1 | 2>(1);
  const [recoveryOpen, setRecoveryOpen] = React.useState(false);
  const [recoveryCodes, setRecoveryCodes] = React.useState<string[]>([]);
  const isDesktop = useMediaQuery("(min-width: 768px)");
  const [pwOpen, setPwOpen] = React.useState(false);
  const [pwCurrent, setPwCurrent] = React.useState("");
  const [pwNext, setPwNext] = React.useState("");

  React.useEffect(() => {
    if (!token) return;
    gql<{ authSettings: AuthState }>(
      `query{ authSettings{ email_login_enabled has_password totp_enabled } }`
    )
      .then((d) => setAuth(d.authSettings))
      .catch(() =>
        toast({ variant: "destructive", title: dict.toast.userInfoFetchError })
      );
  }, [token]);

  async function toggleEmailLogin(v: boolean) {
    const prev = auth;
    setAuth((s) => (s ? { ...s, email_login_enabled: v } : s));
    try {
      await gql<{ setAuthSettings: boolean }>(
        `mutation($on:Boolean!){ setAuthSettings(email_login_enabled:$on) }`,
        { on: v }
      );
    } catch {
      if (prev) setAuth(prev);
      toast({ variant: "destructive", title: dict.toast.updateFailed });
    }
  }

  async function beginTotp() {
    const d = await gql<{ totpSetup: { secret: string; otpauth: string } }>(
      `mutation{ totpSetup{ secret otpauth } }`
    );
    setTotpSecret(d.totpSetup.secret);
    setTotpUri(d.totpSetup.otpauth);
    setTotpStep(1);
    setTotpOpen(true);
  }
  async function activateTotp() {
    try {
      await gql<{ totpActivate: boolean }>(
        `mutation($code:String!){ totpActivate(code:$code) }`,
        { code: totpCode }
      );
      toast({ title: dict.settingsSecurity.totp.activate });
      setAuth((s) => (s ? { ...s, totp_enabled: true } : s));
      setTotpSecret("");
      setTotpUri("");
      setTotpCode("");
      setTotpOpen(false);
    } catch {
      toast({
        variant: "destructive",
        title: dict.settingsSecurity.totp.codeInvalid,
      });
    }
  }
  async function disableTotp() {
    try {
      await gql<{ totpDisable: boolean }>(`mutation{ totpDisable }`);
      setAuth((s) => (s ? { ...s, totp_enabled: false } : s));
    } catch {}
  }

  async function generateRecovery() {
    const data = await gql<{ recoveryGenerate: string[] }>(
      `mutation{ recoveryGenerate }`
    );
    setRecoveryCodes(data.recoveryGenerate);
    setRecoveryOpen(true);
  }

  async function registerPasskey() {
    try {
      // Begin via GraphQL and convert fields
      const resp = await gql<{ webauthnRegisterBegin: string }>(
        `mutation{ webauthnRegisterBegin }`
      );
      const options = JSON.parse(resp.webauthnRegisterBegin);
      if (
        options?.publicKey?.challenge &&
        typeof options.publicKey.challenge === "string"
      ) {
        (options.publicKey.challenge as any) = base64urlToBuffer(
          options.publicKey.challenge
        );
      }
      if (
        options?.publicKey?.user?.id &&
        typeof options.publicKey.user.id === "string"
      ) {
        options.publicKey.user.id = base64urlToBuffer(
          options.publicKey.user.id as string
        ) as any;
      }
      if (Array.isArray(options?.publicKey?.excludeCredentials)) {
        options.publicKey.excludeCredentials =
          options.publicKey.excludeCredentials.map((ex: any) => {
            if (ex?.id && typeof ex.id === "string")
              ex.id = base64urlToBuffer(ex.id);
            return ex;
          });
      }
      const cred: any = await navigator.credentials.create({
        publicKey: options.publicKey,
      });
      const attResp: any = cred.response;
      const finishPayload = {
        id: cred.id,
        rawId: bufferToBase64url(cred.rawId),
        type: cred.type,
        response: {
          attestationObject: bufferToBase64url(attResp.attestationObject),
          clientDataJSON: bufferToBase64url(attResp.clientDataJSON),
        },
      };
      const finished = await gql<boolean>(
        `mutation($input:String!){ webauthnRegisterFinish(input:$input) }`,
        { input: JSON.stringify(finishPayload) }
      );
      if (finished) {
        toast({ title: dict.settingsSecurity.passkey.registered });
      } else {
        throw new Error("finish returned false");
      }
    } catch (err: any) {
      toast({
        title: "Failed to register passkey",
        description: err.message,
        variant: "destructive",
      });
    }
  }

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">
          {dict.settingsSecurity.heading}
        </h1>
        <Card className="p-4 space-y-3">
          <div className="flex items-center justify-between gap-4">
            <div>
              <div className="font-medium">
                {dict.settingsSecurity.password.title}
              </div>
              <div className="text-sm text-muted-foreground">
                {dict.settingsSecurity.password.desc}
              </div>
            </div>
            <Dialog
              open={isDesktop && pwOpen}
              onOpenChange={(v: boolean) => setPwOpen(isDesktop ? v : pwOpen)}
            >
              <DialogTrigger asChild>
                <Button variant="secondary" onClick={() => setPwOpen(true)}>
                  {dict.settingsSecurity.password.open}
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>
                    {dict.settingsSecurity.password.dialogTitle}
                  </DialogTitle>
                  <DialogDescription>
                    {dict.settingsSecurity.password.dialogDesc}
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-3">
                  <div>
                    <Label>{dict.settingsSecurity.password.current}</Label>
                    <Input
                      type="password"
                      value={pwCurrent}
                      onChange={(e) => setPwCurrent(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label>{dict.settingsSecurity.password.next}</Label>
                    <Input
                      type="password"
                      value={pwNext}
                      onChange={(e) => setPwNext(e.target.value)}
                    />
                  </div>
                </div>
                <DialogFooter>
                  <Button variant="secondary" onClick={() => setPwOpen(false)}>
                    {dict.common.close}
                  </Button>
                  <Button
                    onClick={async () => {
                      try {
                        await gql<{ setPassword: boolean }>(
                          `mutation($current:String,$next:String!){ setPassword(current:$current, next:$next) }`,
                          { current: pwCurrent || undefined, next: pwNext }
                        );
                        toast({ title: dict.toast.updated });
                        setPwCurrent("");
                        setPwNext("");
                        setPwOpen(false);
                      } catch (e: any) {
                        toast({
                          variant: "destructive",
                          title: dict.toast.updateFailed,
                          description: e?.message,
                        });
                      }
                    }}
                  >
                    {dict.settingsSecurity.password.update}
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
            <Drawer
              open={!isDesktop && pwOpen}
              onOpenChange={(v: boolean) => setPwOpen(!isDesktop ? v : pwOpen)}
            >
              <DrawerContent>
                <DrawerHeader className="text-left">
                  <DrawerTitle>
                    {dict.settingsSecurity.password.dialogTitle}
                  </DrawerTitle>
                  <DrawerDescription>
                    {dict.settingsSecurity.password.dialogDesc}
                  </DrawerDescription>
                </DrawerHeader>
                <div className="px-4 pb-4 space-y-3">
                  <div>
                    <Label>{dict.settingsSecurity.password.current}</Label>
                    <Input
                      type="password"
                      value={pwCurrent}
                      onChange={(e) => setPwCurrent(e.target.value)}
                    />
                  </div>
                  <div>
                    <Label>{dict.settingsSecurity.password.next}</Label>
                    <Input
                      type="password"
                      value={pwNext}
                      onChange={(e) => setPwNext(e.target.value)}
                    />
                  </div>
                  <div className="flex gap-2">
                    <Button
                      variant="secondary"
                      onClick={() => setPwOpen(false)}
                      className="flex-1"
                    >
                      {dict.common.close}
                    </Button>
                    <Button
                      className="flex-1"
                      onClick={async () => {
                        try {
                          await gql<{ setPassword: boolean }>(
                            `mutation($current:String,$next:String!){ setPassword(current:$current, next:$next) }`,
                            { current: pwCurrent || undefined, next: pwNext }
                          );
                          toast({ title: dict.toast.updated });
                          setPwCurrent("");
                          setPwNext("");
                          setPwOpen(false);
                        } catch (e: any) {
                          toast({
                            variant: "destructive",
                            title: dict.toast.updateFailed,
                            description: e?.message,
                          });
                        }
                      }}
                    >
                      {dict.settingsSecurity.password.update}
                    </Button>
                  </div>
                </div>
              </DrawerContent>
            </Drawer>
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="flex items-center justify-between gap-4">
            <div>
              <div className="font-medium">
                {dict.settingsSecurity.emailLogin.title}
              </div>
              <div className="text-sm text-muted-foreground">
                {dict.settingsSecurity.emailLogin.note}
              </div>
            </div>
            <Switch
              checked={!!auth?.email_login_enabled}
              onCheckedChange={toggleEmailLogin}
            />
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="font-medium">{dict.settingsSecurity.totp.title}</div>
          {auth?.totp_enabled ? (
            <div className="flex items-center gap-3">
              <div className="text-sm text-muted-foreground">
                {dict.settingsSecurity.totp.enabled}
              </div>
              <Button variant="secondary" onClick={disableTotp}>
                {dict.settingsSecurity.totp.disable}
              </Button>
            </div>
          ) : (
            <div className="space-y-3">
              <Button onClick={beginTotp}>
                {dict.settingsSecurity.totp.begin}
              </Button>
              <Dialog open={totpOpen} onOpenChange={setTotpOpen}>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>
                      {dict.settingsSecurity.totp.setupTitle}
                    </DialogTitle>
                    <DialogDescription>
                      {dict.settingsSecurity.totp.setupDesc}
                    </DialogDescription>
                  </DialogHeader>
                  {totpStep === 1 ? (
                    <div className="space-y-3">
                      <div className="flex justify-center">
                        {totpUri ? (
                          <img
                            alt="TOTP QR"
                            className="rounded border"
                            width={200}
                            height={200}
                            src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(
                              totpUri
                            )}`}
                          />
                        ) : null}
                      </div>
                      <div>
                        <Label>{dict.settingsSecurity.totp.secret}</Label>
                        <div className="flex items-center gap-2">
                          <Input
                            readOnly
                            value={totpSecret}
                            className="font-mono"
                          />
                          <Button
                            type="button"
                            variant="secondary"
                            onClick={async () => {
                              await navigator.clipboard.writeText(totpSecret);
                              toast({ title: dict.common.copy });
                            }}
                          >
                            {dict.common.copy}
                          </Button>
                        </div>
                      </div>
                      <div className="flex justify-end">
                        <Button onClick={() => setTotpStep(2)}>
                          {dict.settingsSecurity.totp.next}
                        </Button>
                      </div>
                    </div>
                  ) : (
                    <div className="space-y-3">
                      <div>
                        <Label htmlFor="totp">
                          {dict.settingsSecurity.totp.code}
                        </Label>
                        <Input
                          id="totp"
                          value={totpCode}
                          onChange={(e) => setTotpCode(e.target.value)}
                          placeholder="123456"
                        />
                      </div>
                      <DialogFooter>
                        <Button
                          variant="secondary"
                          onClick={() => setTotpStep(1)}
                        >
                          {dict.settingsSecurity.totp.back}
                        </Button>
                        <Button onClick={activateTotp}>
                          {dict.settingsSecurity.totp.activate}
                        </Button>
                      </DialogFooter>
                    </div>
                  )}
                </DialogContent>
              </Dialog>
            </div>
          )}
        </Card>
        <Card className="p-4 space-y-3">
          <div className="font-medium">
            {dict.settingsSecurity.passkey.title}
          </div>
          <div className="text-sm text-muted-foreground">
            {dict.settingsSecurity.passkey.desc}
          </div>
          <Button onClick={registerPasskey}>
            {dict.settingsSecurity.passkey.register}
          </Button>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="font-medium">
            {dict.settingsSecurity.recovery.title}
          </div>
          <div className="text-sm text-muted-foreground">
            {dict.settingsSecurity.recovery.desc}
          </div>
          <Button onClick={generateRecovery}>
            {dict.settingsSecurity.recovery.generate}
          </Button>
          <Dialog open={recoveryOpen} onOpenChange={setRecoveryOpen}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>
                  {dict.settingsSecurity.recovery.dialogTitle}
                </DialogTitle>
                <DialogDescription>
                  {dict.settingsSecurity.recovery.dialogDesc}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-2 max-h-64 overflow-auto">
                {recoveryCodes.map((code, i) => (
                  <div
                    key={i}
                    className="rounded border px-3 py-2 font-mono text-sm flex items-center justify-between"
                  >
                    <span>{code}</span>
                  </div>
                ))}
              </div>
              <DialogFooter>
                <Button
                  variant="secondary"
                  onClick={async () => {
                    await navigator.clipboard.writeText(
                      recoveryCodes.join("\n")
                    );
                    toast({ title: dict.common.copy });
                  }}
                >
                  {dict.common.copy}
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => {
                    const w = window.open("");
                    if (!w) return;
                    w.document.write(`<pre>${recoveryCodes.join("\n")}</pre>`);
                    w.document.close();
                    w.focus();
                    w.print();
                  }}
                >
                  {dict.common.print}
                </Button>
                <Button onClick={() => setRecoveryOpen(false)}>
                  {dict.common.close}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </Card>
      </div>
    </SettingsShell>
  );
}

function base64urlToBuffer(base64url: string) {
  const padding = "=".repeat((4 - (base64url.length % 4)) % 4);
  const base64 = (base64url + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const buffer = new ArrayBuffer(raw.length);
  const view = new Uint8Array(buffer);
  for (let i = 0; i < raw.length; ++i) view[i] = raw.charCodeAt(i);
  return buffer;
}
function bufferToBase64url(buffer: ArrayBuffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++)
    binary += String.fromCharCode(bytes[i]);
  return btoa(binary)
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

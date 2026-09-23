"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
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
import { toast } from "@/hooks/use-toast";
import { useLang } from "@/react/LanguageProvider";
import {
  activateTotpSetup,
  beginPasskeyRegistration,
  beginTotpSetup,
  deletePasskey,
  disableTotpSetup,
  finishPasskeyRegistration,
  generateRecoveryCodes,
  loadSecuritySettings,
  renamePasskey,
  setEmailLogin,
  setPassword,
  type AuthState,
  type Passkey,
} from "@/features/security/api";
import {
  base64urlToBuffer,
  bufferToBase64url,
} from "@/features/security/webauthn";

export default function SettingsSecurity() {
  const { dict } = useLang();
  const authPasskeyURL = import.meta.env.PUBLIC_AUTH_PASSKEY_URL?.trim();
  const authPasswordURL = import.meta.env.PUBLIC_AUTH_PASSWORD_URL?.trim();
  const authEmailCodeURL = import.meta.env.PUBLIC_AUTH_EMAIL_CODE_URL?.trim();
  const authRecoveryURL = import.meta.env.PUBLIC_AUTH_RECOVERY_URL?.trim();
  const authTOTPURL = import.meta.env.PUBLIC_AUTH_TOTP_URL?.trim();
  const [auth, setAuth] = React.useState<AuthState | null>(null);
  const [totpSecret, setTotpSecret] = React.useState<string>("");
  const [totpUri, setTotpUri] = React.useState<string>("");
  const [totpCode, setTotpCode] = React.useState<string>("");
  const [totpOpen, setTotpOpen] = React.useState(false);
  const [totpStep, setTotpStep] = React.useState<1 | 2>(1);
  const [recoveryOpen, setRecoveryOpen] = React.useState(false);
  const [recoveryCodes, setRecoveryCodes] = React.useState<string[]>([]);
  const [disablingTotp, setDisablingTotp] = React.useState(false);
  const [generatingRecovery, setGeneratingRecovery] = React.useState(false);
  const isDesktop = useMediaQuery("(min-width: 768px)");
  const [pwOpen, setPwOpen] = React.useState(false);
  const [pwCurrent, setPwCurrent] = React.useState("");
  const [pwNext, setPwNext] = React.useState("");
  const [pkOpen, setPkOpen] = React.useState(false);
  const [passkeys, setPasskeys] = React.useState<Passkey[]>([]);
  const [editingPk, setEditingPk] = React.useState<{
    id: number;
    name: string;
  } | null>(null);

  React.useEffect(() => {
    if (new URLSearchParams(window.location.search).has("change-password")) {
      if (authPasswordURL) {
        window.location.assign(authPasswordURL);
        return;
      }
      setPwOpen(true);
    }
  }, [authPasswordURL]);

  React.useEffect(() => {
    loadSecuritySettings()
      .then((d) => {
        setAuth(d.authSettings);
        setPasskeys(d.myPasskeys || []);
      })
      .catch(() =>
        toast({ variant: "destructive", title: dict.toast.userInfoFetchError }),
      );
  }, []);

  async function toggleEmailLogin(v: boolean) {
    const prev = auth;
    setAuth((s) => (s ? { ...s, email_login_enabled: v } : s));
    try {
      await setEmailLogin(v);
    } catch {
      if (prev) setAuth(prev);
      toast({ variant: "destructive", title: dict.toast.updateFailed });
    }
  }

  async function beginTotp() {
    const setup = await beginTotpSetup();
    setTotpSecret(setup.secret);
    setTotpUri(setup.otpauth);
    setTotpStep(1);
    setTotpOpen(true);
  }
  async function activateTotp() {
    try {
      await activateTotpSetup(totpCode);
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
    setDisablingTotp(true);
    try {
      await disableTotpSetup();
      setAuth((s) => (s ? { ...s, totp_enabled: false } : s));
      toast({ title: dict.settingsSecurity.totp.disabled });
    } catch {
      toast({ variant: "destructive", title: dict.toast.updateFailed });
    } finally {
      setDisablingTotp(false);
    }
  }

  async function generateRecovery() {
    setGeneratingRecovery(true);
    try {
      setRecoveryCodes(await generateRecoveryCodes());
      setRecoveryOpen(true);
    } catch {
      toast({ variant: "destructive", title: dict.toast.updateFailed });
    } finally {
      setGeneratingRecovery(false);
    }
  }

  async function registerPasskey() {
    try {
      // Begin via GraphQL and convert fields
      const options = await beginPasskeyRegistration();
      if (
        options?.publicKey?.challenge &&
        typeof options.publicKey.challenge === "string"
      ) {
        (options.publicKey.challenge as any) = base64urlToBuffer(
          options.publicKey.challenge,
        );
      }
      if (
        options?.publicKey?.user?.id &&
        typeof options.publicKey.user.id === "string"
      ) {
        options.publicKey.user.id = base64urlToBuffer(
          options.publicKey.user.id as string,
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
      const finished = await finishPasskeyRegistration(finishPayload);
      if (finished) {
        toast({ title: dict.settingsSecurity.passkey.registered });
        // reload list/counts
        try {
          const d = await loadSecuritySettings();
          setPasskeys(d.myPasskeys || []);
          setAuth((s) => ({ ...(s || {}), ...d.authSettings }) as AuthState);
        } catch {}
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
        <h1 className="scroll-m-20 text-2xl font-bold tracking-tight text-balance">
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
            {authPasswordURL && (
              <Button asChild variant="secondary">
                <a href={authPasswordURL}>
                  {dict.settingsSecurity.password.open}
                </a>
              </Button>
            )}
            {!authPasswordURL && (
              <>
                <Dialog
                  open={isDesktop && pwOpen}
                  onOpenChange={(v: boolean) =>
                    setPwOpen(isDesktop ? v : pwOpen)
                  }
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
                      <Button
                        variant="secondary"
                        onClick={() => setPwOpen(false)}
                      >
                        {dict.common.close}
                      </Button>
                      <Button
                        onClick={async () => {
                          try {
                            await setPassword(pwCurrent || undefined, pwNext);
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
                  onOpenChange={(v: boolean) =>
                    setPwOpen(!isDesktop ? v : pwOpen)
                  }
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
                              await setPassword(pwCurrent || undefined, pwNext);
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
              </>
            )}
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="flex items-center justify-between gap-4">
            <div>
              {authEmailCodeURL ? (
                <div className="font-medium">
                  {dict.settingsSecurity.emailLogin.title}
                </div>
              ) : (
                <label htmlFor="security-email-login" className="font-medium">
                  {dict.settingsSecurity.emailLogin.title}
                </label>
              )}
              <div className="text-sm text-muted-foreground">
                {authEmailCodeURL
                  ? dict.settingsSecurity.emailLogin.externalNote
                  : dict.settingsSecurity.emailLogin.note}
              </div>
            </div>
            {authEmailCodeURL ? (
              <Button asChild variant="secondary">
                <a href={authEmailCodeURL}>
                  {dict.settingsSecurity.emailLogin.manage}
                </a>
              </Button>
            ) : (
              <Switch
                id="security-email-login"
                checked={!!auth?.email_login_enabled}
                onCheckedChange={toggleEmailLogin}
              />
            )}
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="font-medium">{dict.settingsSecurity.totp.title}</div>
          {authTOTPURL ? (
            <Button asChild variant="secondary">
              <a href={authTOTPURL}>{dict.settingsSecurity.totp.manage}</a>
            </Button>
          ) : auth?.totp_enabled ? (
            <div className="flex items-center gap-3">
              <div className="text-sm text-muted-foreground">
                {dict.settingsSecurity.totp.enabled}
              </div>
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="secondary">
                    {dict.settingsSecurity.totp.disable}
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>
                      {dict.settingsSecurity.totp.disableConfirmTitle}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                      {dict.settingsSecurity.totp.disableConfirmDesc}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>{dict.common.cancel}</AlertDialogCancel>
                    <AlertDialogAction
                      variant="destructive"
                      disabled={disablingTotp}
                      onClick={disableTotp}
                    >
                      {dict.settingsSecurity.totp.disable}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
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
                              totpUri,
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
          {authPasskeyURL && (
            <Button asChild>
              <a href={authPasskeyURL}>
                {dict.settingsSecurity.passkey.manage}
              </a>
            </Button>
          )}
          <div className={authPasskeyURL ? "hidden" : "flex flex-wrap gap-2"}>
            <Button onClick={registerPasskey}>
              {dict.settingsSecurity.passkey.register}
            </Button>
            <Dialog
              open={isDesktop && pkOpen}
              onOpenChange={(v: boolean) => setPkOpen(isDesktop ? v : pkOpen)}
            >
              <DialogTrigger asChild>
                <Button variant="secondary" onClick={() => setPkOpen(true)}>
                  {dict.settingsSecurity.passkey.manage}
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>
                    {dict.settingsSecurity.passkey.manage}
                  </DialogTitle>
                </DialogHeader>
                <div className="space-y-3">
                  <Button onClick={registerPasskey}>
                    {dict.settingsSecurity.passkey.addThisDevice}
                  </Button>
                  <div className="space-y-2 max-h-64 overflow-auto">
                    {passkeys.length === 0 ? (
                      <div className="text-sm text-muted-foreground">
                        {dict.settingsSecurity.passkey.none}
                      </div>
                    ) : (
                      passkeys.map((pk) => (
                        <div
                          key={pk.id}
                          className="flex items-center gap-2 border rounded px-3 py-2"
                        >
                          {editingPk?.id === pk.id ? (
                            <>
                              <Input
                                value={editingPk.name}
                                onChange={(e) =>
                                  setEditingPk({
                                    id: pk.id,
                                    name: e.target.value,
                                  })
                                }
                                className="flex-1"
                              />
                              <Button
                                size="sm"
                                onClick={async () => {
                                  try {
                                    await renamePasskey(pk.id, editingPk.name);
                                    setPasskeys((list) =>
                                      list.map((it) =>
                                        it.id === pk.id
                                          ? {
                                              ...it,
                                              device_name: editingPk.name,
                                            }
                                          : it,
                                      ),
                                    );
                                    setEditingPk(null);
                                  } catch {
                                    toast({
                                      variant: "destructive",
                                      title: dict.toast.updateFailed,
                                    });
                                  }
                                }}
                              >
                                {dict.common.save}
                              </Button>
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() => setEditingPk(null)}
                              >
                                {dict.common.cancel}
                              </Button>
                            </>
                          ) : (
                            <>
                              <div className="flex-1">
                                <div className="font-medium">
                                  {pk.device_name || "Passkey"}
                                </div>
                                <div className="text-xs text-muted-foreground">
                                  {new Date(pk.created_at).toLocaleString()}
                                </div>
                              </div>
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() =>
                                  setEditingPk({
                                    id: pk.id,
                                    name: pk.device_name,
                                  })
                                }
                              >
                                {dict.settingsSecurity.passkey.rename}
                              </Button>
                              <Button
                                size="sm"
                                variant="destructive"
                                onClick={async () => {
                                  try {
                                    await deletePasskey(pk.id);
                                    setPasskeys((list) =>
                                      list.filter((it) => it.id !== pk.id),
                                    );
                                  } catch {
                                    toast({
                                      variant: "destructive",
                                      title: dict.toast.deleteFailed,
                                    });
                                  }
                                }}
                              >
                                {dict.settingsSecurity.passkey.delete}
                              </Button>
                            </>
                          )}
                        </div>
                      ))
                    )}
                  </div>
                  <DialogFooter>
                    <Button
                      variant="secondary"
                      onClick={() => setPkOpen(false)}
                    >
                      {dict.common.close}
                    </Button>
                  </DialogFooter>
                </div>
              </DialogContent>
            </Dialog>
            <Drawer
              open={!isDesktop && pkOpen}
              onOpenChange={(v: boolean) => setPkOpen(!isDesktop ? v : pkOpen)}
            >
              <DrawerContent>
                <DrawerHeader className="text-left">
                  <DrawerTitle>
                    {dict.settingsSecurity.passkey.manage}
                  </DrawerTitle>
                </DrawerHeader>
                <div className="px-4 pb-4 space-y-3">
                  <Button className="w-full" onClick={registerPasskey}>
                    {dict.settingsSecurity.passkey.addThisDevice}
                  </Button>
                  <div className="space-y-2 max-h-64 overflow-auto">
                    {passkeys.length === 0 ? (
                      <div className="text-sm text-muted-foreground">
                        {dict.settingsSecurity.passkey.none}
                      </div>
                    ) : (
                      passkeys.map((pk) => (
                        <div
                          key={pk.id}
                          className="flex items-center gap-2 border rounded px-3 py-2"
                        >
                          {editingPk?.id === pk.id ? (
                            <>
                              <Input
                                value={editingPk.name}
                                onChange={(e) =>
                                  setEditingPk({
                                    id: pk.id,
                                    name: e.target.value,
                                  })
                                }
                                className="flex-1"
                              />
                              <Button
                                size="sm"
                                onClick={async () => {
                                  try {
                                    await renamePasskey(pk.id, editingPk.name);
                                    setPasskeys((list) =>
                                      list.map((it) =>
                                        it.id === pk.id
                                          ? {
                                              ...it,
                                              device_name: editingPk.name,
                                            }
                                          : it,
                                      ),
                                    );
                                    setEditingPk(null);
                                  } catch {
                                    toast({
                                      variant: "destructive",
                                      title: dict.toast.updateFailed,
                                    });
                                  }
                                }}
                              >
                                {dict.common.save}
                              </Button>
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() => setEditingPk(null)}
                              >
                                {dict.common.cancel}
                              </Button>
                            </>
                          ) : (
                            <>
                              <div className="flex-1">
                                <div className="font-medium">
                                  {pk.device_name || "Passkey"}
                                </div>
                                <div className="text-xs text-muted-foreground">
                                  {new Date(pk.created_at).toLocaleString()}
                                </div>
                              </div>
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() =>
                                  setEditingPk({
                                    id: pk.id,
                                    name: pk.device_name,
                                  })
                                }
                              >
                                {dict.settingsSecurity.passkey.rename}
                              </Button>
                              <Button
                                size="sm"
                                variant="destructive"
                                onClick={async () => {
                                  try {
                                    await deletePasskey(pk.id);
                                    setPasskeys((list) =>
                                      list.filter((it) => it.id !== pk.id),
                                    );
                                  } catch {
                                    toast({
                                      variant: "destructive",
                                      title: dict.toast.deleteFailed,
                                    });
                                  }
                                }}
                              >
                                {dict.settingsSecurity.passkey.delete}
                              </Button>
                            </>
                          )}
                        </div>
                      ))
                    )}
                  </div>
                  <div className="flex gap-2">
                    <Button
                      className="flex-1"
                      variant="secondary"
                      onClick={() => setPkOpen(false)}
                    >
                      {dict.common.close}
                    </Button>
                  </div>
                </div>
              </DrawerContent>
            </Drawer>
          </div>
        </Card>
        <Card className="p-4 space-y-3">
          <div className="font-medium">
            {dict.settingsSecurity.recovery.title}
          </div>
          <div className="text-sm text-muted-foreground">
            {dict.settingsSecurity.recovery.desc}
          </div>
          {authRecoveryURL ? (
            <Button asChild>
              <a href={authRecoveryURL}>
                {dict.settingsSecurity.recovery.reissue}
              </a>
            </Button>
          ) : auth?.recovery_codes_count && auth.recovery_codes_count > 0 ? (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button disabled={generatingRecovery}>
                  {dict.settingsSecurity.recovery.reissue}
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>
                    {dict.settingsSecurity.recovery.reissueConfirmTitle}
                  </AlertDialogTitle>
                  <AlertDialogDescription>
                    {dict.settingsSecurity.recovery.reissueConfirmDesc}
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>{dict.common.cancel}</AlertDialogCancel>
                  <AlertDialogAction
                    disabled={generatingRecovery}
                    onClick={generateRecovery}
                  >
                    {dict.settingsSecurity.recovery.reissue}
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          ) : (
            <Button disabled={generatingRecovery} onClick={generateRecovery}>
              {dict.settingsSecurity.recovery.generate}
            </Button>
          )}
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
                      recoveryCodes.join("\n"),
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

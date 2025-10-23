"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { useLang } from "@/react/LanguageProvider";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

type McpInfo = {
  enabled: boolean;
  has_token: boolean;
  expires_at?: string | null;
};
type LogItem = { action: string; created_at: string };

export default function SettingsMcp() {
  const { dict } = useLang();
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [info, setInfo] = React.useState<McpInfo | null>(null);
  const [createdToken, setCreatedToken] = React.useState<string | null>(null);
  const [logs, setLogs] = React.useState<LogItem[]>([]);
  const [creating, setCreating] = React.useState(false);
  const [copied, setCopied] = React.useState(false);
  const copiedTimerRef = React.useRef<number | null>(null);
  const [expires, setExpires] = React.useState<string>("");
  const [editExpires, setEditExpires] = React.useState<string>("");

  async function loadInfo() {
    try {
      const d = await gql<{ mcpInfo: McpInfo }>(
        `query{ mcpInfo{ enabled has_token expires_at } }`
      );
      setInfo(d.mcpInfo);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.toast.userInfoFetchError,
        description: e?.message,
      });
    }
  }
  async function loadLogs() {
    try {
      const d = await gql<{ myLogs: LogItem[] }>(
        `query($pf:String,$limit:Int){ myLogs(prefix:$pf, limit:$limit){ action created_at } }`,
        { pf: "mcp.", limit: 20 }
      );
      setLogs(d.myLogs || []);
    } catch (e: any) {
      // non-fatal
    }
  }
  React.useEffect(() => {
    if (token) {
      loadInfo();
      loadLogs();
    }
  }, [token]);

  async function createToken() {
    try {
      if (creating) return;
      setCreating(true);
      const d = await gql<{ mcpCreate: { ok: boolean; token: string } }>(
        `mutation($e:String){ mcpCreate(expires_at:$e){ ok token } }`,
        { e: expires || undefined }
      );
      setCreatedToken(d.mcpCreate.token);
      setExpires("");
      // Do NOT refresh info here; it would flip has_token to true and unmount the dialog
      // Keep logs optional to update background activity if desired
      // await loadLogs();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.toast.createFailed,
        description: e?.message,
      });
    } finally {
      setCreating(false);
    }
  }
  async function revokeToken() {
    try {
      await gql<{ mcpRevoke: boolean }>(`mutation{ mcpRevoke }`);
      toast({ title: dict.settingsNotifications.disabledMsg });
      setCreatedToken(null);
      await loadInfo();
      await loadLogs();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.toast.updateFailed,
        description: e?.message,
      });
    }
  }

  async function updateExpiry() {
    try {
      await gql<{ mcpUpdate: boolean }>(
        `mutation($e:String){ mcpUpdate(expires_at:$e) }`,
        { e: editExpires || undefined }
      );
      toast({ title: dict.toast.updated });
      setEditExpires("");
      await loadInfo();
      await loadLogs();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.toast.updateFailed,
        description: e?.message,
      });
    }
  }

  const disabled = info && !info.enabled;

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">{dict.settingsMcp.heading}</h1>
        <Alert>
          <AlertTitle>{dict.common.warning}</AlertTitle>
          <AlertDescription>{dict.settingsMcp.shareWarning}</AlertDescription>
        </Alert>
        <Card className="p-4 space-y-3">
          {disabled ? (
            <div className="text-sm text-muted-foreground">
              {dict.settingsMcp.disabled}
            </div>
          ) : (
            <div className="space-y-3">
              <div className="text-sm text-muted-foreground">
                {dict.settingsMcp.description}
              </div>
              <div className="space-y-2">
                {info?.has_token && (
                  <div className="text-sm text-muted-foreground">
                    期限:{" "}
                    {info.expires_at
                      ? new Date(info.expires_at).toLocaleString()
                      : "-"}
                    {info.expires_at &&
                      new Date(info.expires_at) <= new Date() && (
                        <span className="ml-2">
                          <Badge variant="destructive">期限切れ</Badge>
                        </span>
                      )}
                  </div>
                )}
                <div className="flex flex-wrap gap-2">
                  {info?.has_token ? (
                    <>
                      <Button variant="destructive" onClick={revokeToken}>
                        {dict.settingsMcp.revokeToken}
                      </Button>
                      <Dialog
                        onOpenChange={(open) => {
                          if (open) {
                            setEditExpires(info?.expires_at || "");
                          } else {
                            setEditExpires("");
                          }
                        }}
                      >
                        <DialogTrigger asChild>
                          <Button variant="outline">期限を更新</Button>
                        </DialogTrigger>
                        <DialogContent>
                          <DialogHeader>
                            <DialogTitle>有効期限を設定/更新</DialogTitle>
                          </DialogHeader>
                          <div className="space-y-3">
                            <div>
                              <div className="text-sm mb-1">
                                有効期限(任意, RFC3339)
                              </div>
                              <input
                                className="w-full h-9 rounded border bg-transparent px-3 text-sm"
                                value={editExpires}
                                onChange={(e) => setEditExpires(e.target.value)}
                                placeholder="例: 2025-12-31T23:59:59Z"
                              />
                            </div>
                            <DialogFooter>
                              <Button onClick={updateExpiry}>
                                {dict.common.save || "保存"}
                              </Button>
                            </DialogFooter>
                          </div>
                        </DialogContent>
                      </Dialog>
                    </>
                  ) : (
                    <Dialog
                      onOpenChange={async (open) => {
                        if (open) {
                          setCreatedToken(null);
                          setCreating(false);
                          setCopied(false);
                          setExpires("");
                          if (copiedTimerRef.current) {
                            window.clearTimeout(copiedTimerRef.current);
                            copiedTimerRef.current = null;
                          }
                        } else {
                          // After closing, refresh info and logs to reflect new state
                          await loadInfo();
                          await loadLogs();
                          setCopied(false);
                          if (copiedTimerRef.current) {
                            window.clearTimeout(copiedTimerRef.current);
                            copiedTimerRef.current = null;
                          }
                        }
                      }}
                    >
                      <DialogTrigger asChild>
                        <Button>{dict.settingsMcp.issueToken}</Button>
                      </DialogTrigger>
                      <DialogContent>
                        <DialogHeader>
                          <DialogTitle>
                            {dict.settingsMcp.dialogTitle}
                          </DialogTitle>
                        </DialogHeader>
                        {createdToken ? (
                          <div className="space-y-3">
                            <div className="text-sm">
                              {dict.settingsMcp.tokenWarn}
                            </div>
                            <code className="block break-all rounded border bg-muted/30 p-2 text-sm">
                              {createdToken}
                            </code>
                            <div className="flex gap-2">
                              <TooltipProvider delayDuration={100}>
                                <Tooltip open={copied}>
                                  <TooltipTrigger asChild>
                                    <Button
                                      onClick={async () => {
                                        try {
                                          await navigator.clipboard.writeText(
                                            createdToken!
                                          );
                                          setCopied(true);
                                          if (copiedTimerRef.current) {
                                            window.clearTimeout(
                                              copiedTimerRef.current
                                            );
                                          }
                                          copiedTimerRef.current =
                                            window.setTimeout(
                                              () => setCopied(false),
                                              1200
                                            ) as unknown as number;
                                        } catch {}
                                      }}
                                    >
                                      {dict.common.copy}
                                    </Button>
                                  </TooltipTrigger>
                                  <TooltipContent side="top">
                                    {dict.common.copied}
                                  </TooltipContent>
                                </Tooltip>
                              </TooltipProvider>
                              <Button
                                variant="secondary"
                                onClick={() => setCreatedToken(null)}
                              >
                                {dict.common.close}
                              </Button>
                            </div>
                          </div>
                        ) : (
                          <div className="space-y-3">
                            <div>
                              <div className="text-sm mb-1">
                                有効期限(任意, RFC3339)
                              </div>
                              <input
                                className="w-full h-9 rounded border bg-transparent px-3 text-sm"
                                value={expires}
                                onChange={(e) => setExpires(e.target.value)}
                                placeholder="例: 2025-12-31T23:59:59Z"
                              />
                            </div>
                            <DialogFooter>
                              <Button onClick={createToken} disabled={creating}>
                                {creating
                                  ? dict.common.loading
                                  : dict.settingsMcp.issueToken}
                              </Button>
                            </DialogFooter>
                          </div>
                        )}
                      </DialogContent>
                    </Dialog>
                  )}
                </div>
              </div>
            </div>
          )}
        </Card>
        <Card className="p-0 overflow-x-auto">
          <div className="p-4 flex items-center justify-between">
            <div className="font-medium">{dict.settingsMcp.logs.title}</div>
            <Button variant="secondary" size="sm" onClick={loadLogs}>
              {dict.settingsMcp.logs.reload}
            </Button>
          </div>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{dict.settingsMcp.logs.action}</TableHead>
                <TableHead>{dict.settingsMcp.logs.time}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.map((l, i) => (
                <TableRow key={i}>
                  <TableCell className="font-mono text-xs">
                    {l.action}
                  </TableCell>
                  <TableCell>
                    {new Date(l.created_at).toLocaleString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      </div>
    </SettingsShell>
  );
}

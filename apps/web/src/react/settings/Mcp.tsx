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
import { DateTimePicker } from "@/components/ui/date-time-picker";
import { CircleHelp as CircleQuestionMark } from "lucide-react";
import { formatYmdHm } from "@/lib/utils";

type McpInfo = {
  enabled: boolean;
  has_token: boolean;
  expires_at?: string | null;
};
type LogItem = { action: string; created_at: string };

export default function SettingsMcp() {
  const { dict } = useLang();
  const [info, setInfo] = React.useState<McpInfo | null>(null);
  const [createdToken, setCreatedToken] = React.useState<string | null>(null);
  const [logs, setLogs] = React.useState<LogItem[]>([]);
  const [creating, setCreating] = React.useState(false);
  const [copied, setCopied] = React.useState(false);
  const copiedTimerRef = React.useRef<number | null>(null);
  const [expires, setExpires] = React.useState<string>(""); // local input
  const [editExpires, setEditExpires] = React.useState<string>("");
  const [openIssue, setOpenIssue] = React.useState(false);
  const HELP_URL = "https://ritzy-lift-e38.notion.site/MCP-290a3874f8c880a08c31f53116cb2ead?source=copy_link";

  function toRFC3339(input: string): string | undefined {
    const v = (input || "").trim();
    if (!v) return undefined;
    const d = new Date(v);
    if (isNaN(d.getTime())) return undefined;
    return d.toISOString();
  }
  function fromRFC3339(s?: string | null): string {
    if (!s) return "";
    const d = new Date(s);
    if (isNaN(d.getTime())) return "";
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(
      d.getDate()
    )}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

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
    loadInfo();
    loadLogs();
  }, []);

  // Keyboard shortcut: Cmd/Ctrl+N to open "issue token" dialog when not having one
  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const key = (e.key || "").toLowerCase();
      if ((e.metaKey || e.ctrlKey) && key === "n") {
        if (info && !info.has_token) {
          e.preventDefault();
          setOpenIssue(true);
        }
      }
    };
    window.addEventListener("keydown", onKey as any);
    return () => window.removeEventListener("keydown", onKey as any);
  }, [info]);

  async function createToken() {
    try {
      if (creating) return;
      setCreating(true);
      const d = await gql<{ mcpCreate: { ok: boolean; token: string } }>(
        `mutation($e:String){ mcpCreate(expires_at:$e){ ok token } }`,
        { e: toRFC3339(expires) }
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
      toast({ title: dict.settingsMcp.revokedMsg });
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
        { e: toRFC3339(editExpires) }
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
        <div className="flex items-center justify-between">
          <h1 className="scroll-m-20 text-2xl font-bold tracking-tight text-balance">{dict.settingsMcp.heading}</h1>
          {HELP_URL && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <a href={HELP_URL} target="_blank" rel="noreferrer">
                    <Button variant="ghost" size="icon" aria-label={dict.settingsDetail.help}>
                      <CircleQuestionMark />
                    </Button>
                  </a>
                </TooltipTrigger>
                <TooltipContent>{dict.settingsDetail.help}</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
        </div>
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
                    {dict.settingsDetail.expiry}: {info.expires_at ? formatYmdHm(info.expires_at) : "-"}
                    {info.expires_at &&
                      new Date(info.expires_at) <= new Date() && (
                        <span className="ml-2">
                          <Badge variant="destructive">{dict.settingsDetail.expired}</Badge>
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
                            setEditExpires(fromRFC3339(info?.expires_at));
                          } else {
                            setEditExpires("");
                          }
                        }}
                      >
                        <DialogTrigger asChild>
                          <Button variant="outline">{dict.settingsDetail.updateExpiry}</Button>
                        </DialogTrigger>
                        <DialogContent>
                          <DialogHeader>
                            <DialogTitle>{dict.settingsDetail.expiryDialog}</DialogTitle>
                          </DialogHeader>
                          <div className="space-y-3">
                            <div>
                              <div className="text-sm mb-1">{dict.settingsDetail.expiryOptional}</div>
                              <DateTimePicker
                                value={editExpires}
                                onChange={setEditExpires}
                                placeholder={dict.settingsDetail.selectDateTime}
                              />
                            </div>
                            <DialogFooter>
                              <Button onClick={updateExpiry}>
                                {dict.common.save}
                              </Button>
                            </DialogFooter>
                          </div>
                        </DialogContent>
                      </Dialog>
                    </>
                  ) : (
                    <Dialog
                      open={openIssue}
                      onOpenChange={async (open) => {
                        setOpenIssue(open);
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
                        <Button onClick={() => setOpenIssue(true)}>
                          {dict.settingsMcp.issueToken}
                        </Button>
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
                              <div className="text-sm mb-1">{dict.settingsDetail.expiryOptional}</div>
                              <DateTimePicker
                                value={expires}
                                onChange={setExpires}
                                placeholder={dict.settingsDetail.selectDateTime}
                                autoFocus
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
                  <TableCell>{formatYmdHm(l.created_at)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      </div>
    </SettingsShell>
  );
}

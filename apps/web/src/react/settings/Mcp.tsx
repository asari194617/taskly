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

type McpInfo = { enabled: boolean; has_token: boolean };
type LogItem = { action: string; created_at: string };

export default function SettingsMcp() {
  const { dict } = useLang();
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [info, setInfo] = React.useState<McpInfo | null>(null);
  const [createdToken, setCreatedToken] = React.useState<string | null>(null);
  const [logs, setLogs] = React.useState<LogItem[]>([]);

  async function loadInfo() {
    try {
      const d = await gql<{ mcpInfo: McpInfo }>(
        `query{ mcpInfo{ enabled has_token } }`
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
      const d = await gql<{ mcpCreate: { ok: boolean; token: string } }>(
        `mutation{ mcpCreate{ ok token } }`
      );
      setCreatedToken(d.mcpCreate.token);
      await loadInfo();
      await loadLogs();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: dict.toast.createFailed,
        description: e?.message,
      });
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

  const disabled = info && !info.enabled;

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">{dict.settingsMcp.heading}</h1>
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
              <div className="flex flex-wrap gap-2">
                {info?.has_token ? (
                  <Button variant="destructive" onClick={revokeToken}>
                    {dict.settingsMcp.revokeToken}
                  </Button>
                ) : (
                  <Dialog>
                    <DialogTrigger asChild>
                      <Button
                        onClick={(e) => {
                          e.preventDefault();
                          setCreatedToken(null);
                        }}
                      >
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
                            <Button
                              onClick={() =>
                                navigator.clipboard.writeText(createdToken!)
                              }
                            >
                              {dict.common.copy}
                            </Button>
                            <Button
                              variant="secondary"
                              onClick={() => setCreatedToken(null)}
                            >
                              {dict.common.close}
                            </Button>
                          </div>
                        </div>
                      ) : (
                        <DialogFooter>
                          <Button onClick={createToken}>
                            {dict.settingsMcp.issueToken}
                          </Button>
                        </DialogFooter>
                      )}
                    </DialogContent>
                  </Dialog>
                )}
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

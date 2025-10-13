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

type McpInfo = { enabled: boolean; has_token: boolean };
type LogItem = { action: string; created_at: string };

export default function SettingsMcp() {
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
        title: "MCP情報の取得に失敗しました",
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
        title: "作成に失敗しました",
        description: e?.message,
      });
    }
  }
  async function revokeToken() {
    try {
      await gql<{ mcpRevoke: boolean }>(`mutation{ mcpRevoke }`);
      toast({ title: "トークンを無効化しました" });
      setCreatedToken(null);
      await loadInfo();
      await loadLogs();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "無効化に失敗しました",
        description: e?.message,
      });
    }
  }

  const disabled = info && !info.enabled;

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">MCP 設定</h1>
        <Card className="p-4 space-y-3">
          {disabled ? (
            <div className="text-sm text-muted-foreground">
              この環境ではMCPが無効化されています。
            </div>
          ) : (
            <div className="space-y-3">
              <div className="text-sm text-muted-foreground">
                アプリ連携用のMCPトークンを発行・無効化できます。トークンは一度しか表示されません。
              </div>
              <div className="flex flex-wrap gap-2">
                {info?.has_token ? (
                  <Button variant="destructive" onClick={revokeToken}>
                    トークンを無効化
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
                        トークンを発行
                      </Button>
                    </DialogTrigger>
                    <DialogContent>
                      <DialogHeader>
                        <DialogTitle>MCPトークン</DialogTitle>
                      </DialogHeader>
                      {createdToken ? (
                        <div className="space-y-3">
                          <div className="text-sm">
                            このトークンは一度しか表示されません。必ず安全な場所に保存してください。
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
                              コピー
                            </Button>
                            <Button
                              variant="secondary"
                              onClick={() => setCreatedToken(null)}
                            >
                              閉じる
                            </Button>
                          </div>
                        </div>
                      ) : (
                        <DialogFooter>
                          <Button onClick={createToken}>発行する</Button>
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
            <div className="font-medium">操作履歴</div>
            <Button variant="secondary" size="sm" onClick={loadLogs}>
              再読み込み
            </Button>
          </div>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>アクション</TableHead>
                <TableHead>日時</TableHead>
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

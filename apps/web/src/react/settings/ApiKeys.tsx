"use client";
import * as React from "react";
import SettingsShell from "@/react/SettingsShell";
import { Card } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogClose,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { useLang } from "@/react/LanguageProvider";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { DialogDescription } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { DateTimePicker } from "@/components/ui/date-time-picker";
import {
  Tooltip,
  TooltipProvider,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { CircleHelp as CircleQuestionMark } from "lucide-react";

type ApiKey = {
  id: number;
  description?: string | null;
  is_valid: boolean;
  created_at: string;
  expires_at?: string | null;
};

export default function SettingsApiKeys() {
  const { dict } = useLang();
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [items, setItems] = React.useState<ApiKey[]>([]);
  const [desc, setDesc] = React.useState("");
  const [expires, setExpires] = React.useState<string>(""); // local input
  const [createdToken, setCreatedToken] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);
  const HELP_URL =
    (import.meta as any).env?.PUBLIC_HELP_API_KEYS || "/docs/api-keys";

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

  async function load() {
    try {
      const data = await gql<{ apiKeys: ApiKey[] }>(
        `query{ apiKeys{ id description is_valid created_at expires_at } }`
      );
      setItems(data.apiKeys || []);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "読み込みに失敗しました",
        description: e?.message,
      });
    }
  }
  React.useEffect(() => {
    if (token) load();
  }, [token]);

  async function create() {
    try {
      if (creating) return;
      setCreating(true);
      const data = await gql<{ apiKeyCreate: { id: number; token: string } }>(
        `mutation($d:String,$e:String){ apiKeyCreate(description:$d, expires_at:$e){ id token } }`,
        { d: desc || undefined, e: toRFC3339(expires) }
      );
      setCreatedToken(data.apiKeyCreate.token);
      setDesc("");
      setExpires("");
      await load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "発行に失敗しました",
        description: e?.message,
      });
    } finally {
      setCreating(false);
    }
  }
  async function toggleValid(id: number, isValid: boolean) {
    try {
      await gql<{ apiKeyUpdate: { id: number; is_valid: boolean } }>(
        `mutation($id:Int!,$v:Boolean){ apiKeyUpdate(id:$id, is_valid:$v){ id is_valid } }`,
        { id, v: !isValid }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    }
  }

  async function updateExpiry(id: number, value: string) {
    try {
      await gql<{ apiKeyUpdate: { id: number; is_valid: boolean } }>(
        `mutation($id:Int!,$e:String){ apiKeyUpdate(id:$id, expires_at:$e){ id is_valid } }`,
        { id, e: toRFC3339(value) }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    }
  }

  const [editId, setEditId] = React.useState<number | null>(null);
  const [editExpires, setEditExpires] = React.useState<string>("");

  return (
    <SettingsShell>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-xl font-semibold">
            {dict.settingsApiKeys.heading}
          </h1>
          {HELP_URL && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <a href={HELP_URL} target="_blank" rel="noreferrer">
                    <Button variant="ghost" size="icon" aria-label="ヘルプ">
                      <CircleQuestionMark />
                    </Button>
                  </a>
                </TooltipTrigger>
                <TooltipContent>ヘルプ</TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
        </div>
        <Alert>
          <AlertTitle>{dict.common.warning}</AlertTitle>
          <AlertDescription>
            {dict.settingsApiKeys.shareWarning}
          </AlertDescription>
        </Alert>
        <Card className="p-4 space-y-3">
          <div className="flex flex-wrap items-end gap-2">
            <div className="min-w-60 flex-1">
              <div className="text-sm mb-1">
                {dict.settingsApiKeys.descOptional}
              </div>
              <Input
                value={desc}
                onChange={(e) => setDesc(e.target.value)}
                placeholder="例: CI/CD"
              />
            </div>
            <div className="min-w-60">
              <div className="text-sm mb-1">有効期限(任意)</div>
              <DateTimePicker
                value={expires}
                onChange={setExpires}
                placeholder="日時を選択"
              />
            </div>
            <Dialog
              onOpenChange={(open) => {
                if (open) {
                  setCreatedToken(null);
                  setCreating(false);
                } else {
                  // reset states when closing
                  setCreating(false);
                }
              }}
            >
              <DialogTrigger asChild>
                <Button
                  type="button"
                  onClick={() => {
                    // One-click: open dialog and immediately create
                    create();
                  }}
                >
                  {dict.settingsApiKeys.issueNew}
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>{dict.settingsApiKeys.dialogTitle}</DialogTitle>
                </DialogHeader>
                {createdToken ? (
                  <div className="space-y-2">
                    <div className="text-sm">
                      {dict.settingsApiKeys.tokenWarn}
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
                      <DialogClose asChild>
                        <Button
                          variant="secondary"
                          onClick={() => setCreatedToken(null)}
                        >
                          {dict.common.close}
                        </Button>
                      </DialogClose>
                    </div>
                  </div>
                ) : (
                  <DialogFooter>
                    <Button disabled>
                      {creating
                        ? dict.common.loading
                        : dict.settingsApiKeys.issueNew}
                    </Button>
                  </DialogFooter>
                )}
              </DialogContent>
            </Dialog>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{dict.settingsApiKeys.table.id}</TableHead>
                <TableHead>{dict.settingsApiKeys.table.description}</TableHead>
                <TableHead>{dict.settingsApiKeys.table.status}</TableHead>
                <TableHead>{dict.settingsApiKeys.table.createdAt}</TableHead>
                <TableHead>期限</TableHead>
                <TableHead className="text-right">
                  {dict.settingsApiKeys.table.actions}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((it) => (
                <TableRow key={it.id}>
                  <TableCell>{it.id}</TableCell>
                  <TableCell className="max-w-[300px] truncate">
                    {it.description || "-"}
                  </TableCell>
                  <TableCell>
                    {it.expires_at && new Date(it.expires_at) <= new Date() ? (
                      <Badge variant="destructive">期限切れ</Badge>
                    ) : it.is_valid ? (
                      dict.common.enabled
                    ) : (
                      dict.common.disabled
                    )}
                  </TableCell>
                  <TableCell>
                    {new Date(it.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    {it.expires_at
                      ? new Date(it.expires_at).toLocaleString()
                      : "-"}
                  </TableCell>
                  <TableCell className="text-right">
                    <div className="flex gap-2 justify-end">
                      <Button
                        variant="secondary"
                        onClick={() => toggleValid(it.id, it.is_valid)}
                      >
                        {it.is_valid ? dict.common.disable : dict.common.enable}
                      </Button>
                      <Dialog
                        onOpenChange={(open) => {
                          if (open) {
                            setEditId(it.id);
                            setEditExpires(fromRFC3339(it.expires_at));
                          } else {
                            setEditId(null);
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
                            <DialogDescription>
                              空欄で保存すると期限を解除します。
                            </DialogDescription>
                          </DialogHeader>
                          <DateTimePicker
                            value={editExpires}
                            onChange={setEditExpires}
                            placeholder="日時を選択"
                          />
                          <DialogFooter>
                            <Button
                              onClick={() => updateExpiry(editId!, editExpires)}
                            >
                              保存
                            </Button>
                          </DialogFooter>
                        </DialogContent>
                      </Dialog>
                    </div>
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

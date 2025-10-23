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
import { Input } from "@/components/ui/input";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import { useLang } from "@/react/LanguageProvider";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogDescription,
} from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";

type IItem = {
  id: number;
  description?: string | null;
  url: string;
  created_at: string;
  expires_at?: string | null;
};

export default function SettingsIcal() {
  const { dict } = useLang();
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [items, setItems] = React.useState<IItem[]>([]);
  const [desc, setDesc] = React.useState("");
  async function load() {
    try {
      const data = await gql<{ icalTokens: IItem[] }>(
        `query{ icalTokens{ id description url created_at expires_at } }`
      );
      setItems(data.icalTokens || []);
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
  const [expires, setExpires] = React.useState<string>("");
  async function create() {
    try {
      await gql<{ icalCreate: { id: number; url: string } }>(
        `mutation($d:String,$e:String){ icalCreate(description:$d,expires_at:$e){ id url } }`,
        { d: desc || undefined, e: expires || undefined }
      );
      setDesc("");
      setExpires("");
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "作成に失敗しました",
        description: e?.message,
      });
    }
  }
  async function revoke(id: number) {
    try {
      await gql<{ icalDelete: boolean }>(
        `mutation($id:Int!){ icalDelete(id:$id) }`,
        { id }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "削除に失敗しました",
        description: e?.message,
      });
    }
  }
  const [editId, setEditId] = React.useState<number | null>(null);
  const [editExpires, setEditExpires] = React.useState<string>("");
  async function updateExpiry(id: number, value: string) {
    try {
      await gql<{ icalUpdate: boolean }>(
        `mutation($id:Int!,$e:String){ icalUpdate(id:$id, expires_at:$e) }`,
        { id, e: value || undefined }
      );
      setEditId(null);
      setEditExpires("");
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "更新に失敗しました",
        description: e?.message,
      });
    }
  }
  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">{dict.settingsIcal.heading}</h1>
        <Alert>
          <AlertTitle>{dict.common.warning}</AlertTitle>
          <AlertDescription>{dict.settingsIcal.shareWarning}</AlertDescription>
        </Alert>
        <Card className="p-4 space-y-3">
          <div className="flex items-end gap-2">
            <div className="flex-1 min-w-60">
              <div className="text-sm mb-1">
                {dict.settingsIcal.descOptional}
              </div>
              <Input
                value={desc}
                onChange={(e) => setDesc(e.target.value)}
                placeholder="例: 個人用"
              />
            </div>
            <div className="min-w-60">
              <div className="text-sm mb-1">有効期限(任意, RFC3339)</div>
              <Input
                value={expires}
                onChange={(e) => setExpires(e.target.value)}
                placeholder="例: 2025-12-31T23:59:59Z"
              />
            </div>
            <Button onClick={create}>{dict.settingsIcal.issueUrl}</Button>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{dict.settingsIcal.table.id}</TableHead>
                <TableHead>{dict.settingsIcal.table.description}</TableHead>
                <TableHead>{dict.settingsIcal.table.url}</TableHead>
                <TableHead>{dict.settingsIcal.table.createdAt}</TableHead>
                <TableHead>期限</TableHead>
                <TableHead className="text-right">
                  {dict.settingsIcal.table.actions}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((it) => (
                <TableRow key={it.id}>
                  <TableCell>{it.id}</TableCell>
                  <TableCell className="max-w-[240px] truncate">
                    {it.description || "-"}
                  </TableCell>
                  <TableCell className="max-w-[360px] truncate font-mono text-xs">
                    {it.url}
                  </TableCell>
                  <TableCell>
                    {new Date(it.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell className="space-x-2">
                    {it.expires_at ? (
                      <>
                        <span>{new Date(it.expires_at).toLocaleString()}</span>
                        {new Date(it.expires_at) <= new Date() && (
                          <Badge variant="destructive">期限切れ</Badge>
                        )}
                      </>
                    ) : (
                      "-"
                    )}
                  </TableCell>
                  <TableCell className="text-right flex gap-2 justify-end">
                    <Button
                      variant="secondary"
                      onClick={() => navigator.clipboard.writeText(it.url)}
                    >
                      {dict.common.copy}
                    </Button>
                    <Dialog
                      onOpenChange={(open) => {
                        if (open) {
                          setEditId(it.id);
                          setEditExpires(it.expires_at || "");
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
                        <Input
                          value={editExpires}
                          onChange={(e) => setEditExpires(e.target.value)}
                          placeholder="例: 2025-12-31T23:59:59Z"
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
                    <Button variant="destructive" onClick={() => revoke(it.id)}>
                      {dict.common.delete}
                    </Button>
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

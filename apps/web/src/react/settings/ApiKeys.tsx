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
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";

type ApiKey = {
  id: number;
  description?: string | null;
  is_valid: boolean;
  created_at: string;
};

export default function SettingsApiKeys() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [items, setItems] = React.useState<ApiKey[]>([]);
  const [desc, setDesc] = React.useState("");
  const [createdToken, setCreatedToken] = React.useState<string | null>(null);

  async function load() {
    try {
      const data = await gql<{ apiKeys: ApiKey[] }>(
        `query{ apiKeys{ id description is_valid created_at } }`
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
      const data = await gql<{ apiKeyCreate: { id: number; token: string } }>(
        `mutation($d:String){ apiKeyCreate(description:$d){ id token } }`,
        { d: desc || undefined }
      );
      setCreatedToken(data.apiKeyCreate.token);
      setDesc("");
      await load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "発行に失敗しました",
        description: e?.message,
      });
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

  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">APIキー</h1>
        <Card className="p-4 space-y-3">
          <div className="flex flex-wrap items-end gap-2">
            <div className="min-w-60 flex-1">
              <div className="text-sm mb-1">説明（任意）</div>
              <Input
                value={desc}
                onChange={(e) => setDesc(e.target.value)}
                placeholder="例: CI/CD"
              />
            </div>
            <Dialog>
              <DialogTrigger asChild>
                <Button
                  onClick={(e) => {
                    e.preventDefault();
                  }}
                >
                  新しいAPIキーを発行
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>APIキーを発行</DialogTitle>
                </DialogHeader>
                {createdToken ? (
                  <div className="space-y-2">
                    <div className="text-sm">
                      このトークンは一度しか表示されません。必ず保存してください。
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
                    <Button onClick={create}>発行する</Button>
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
                <TableHead>ID</TableHead>
                <TableHead>説明</TableHead>
                <TableHead>状態</TableHead>
                <TableHead>作成日</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((it) => (
                <TableRow key={it.id}>
                  <TableCell>{it.id}</TableCell>
                  <TableCell className="max-w-[300px] truncate">
                    {it.description || "-"}
                  </TableCell>
                  <TableCell>{it.is_valid ? "有効" : "無効"}</TableCell>
                  <TableCell>
                    {new Date(it.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="secondary"
                      onClick={() => toggleValid(it.id, it.is_valid)}
                    >
                      {it.is_valid ? "無効化" : "有効化"}
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

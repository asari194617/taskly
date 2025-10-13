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

type IItem = {
  id: number;
  description?: string | null;
  url: string;
  created_at: string;
};

export default function SettingsIcal() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [items, setItems] = React.useState<IItem[]>([]);
  const [desc, setDesc] = React.useState("");
  async function load() {
    try {
      const data = await gql<{ icalTokens: IItem[] }>(
        `query{ icalTokens{ id description url created_at } }`
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
  async function create() {
    try {
      await gql<{ icalCreate: { id: number; url: string } }>(
        `mutation($d:String){ icalCreate(description:$d){ id url } }`,
        { d: desc || undefined }
      );
      setDesc("");
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
  return (
    <SettingsShell>
      <div className="space-y-6">
        <h1 className="text-xl font-semibold">iCal</h1>
        <Card className="p-4 space-y-3">
          <div className="flex items-end gap-2">
            <div className="flex-1 min-w-60">
              <div className="text-sm mb-1">説明（任意）</div>
              <Input
                value={desc}
                onChange={(e) => setDesc(e.target.value)}
                placeholder="例: 個人用"
              />
            </div>
            <Button onClick={create}>URLを発行</Button>
          </div>
        </Card>
        <Card className="p-0 overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>ID</TableHead>
                <TableHead>説明</TableHead>
                <TableHead>URL</TableHead>
                <TableHead>作成日</TableHead>
                <TableHead className="text-right">操作</TableHead>
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
                  <TableCell className="text-right flex gap-2 justify-end">
                    <Button
                      variant="secondary"
                      onClick={() => navigator.clipboard.writeText(it.url)}
                    >
                      コピー
                    </Button>
                    <Button variant="destructive" onClick={() => revoke(it.id)}>
                      削除
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

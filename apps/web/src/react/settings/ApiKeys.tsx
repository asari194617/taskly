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
import { useLang } from "@/react/LanguageProvider";

type ApiKey = {
  id: number;
  description?: string | null;
  is_valid: boolean;
  created_at: string;
};

export default function SettingsApiKeys() {
  const { dict } = useLang();
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
        <h1 className="text-xl font-semibold">
          {dict.settingsApiKeys.heading}
        </h1>
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
            <Dialog>
              <DialogTrigger asChild>
                <Button
                  onClick={(e) => {
                    e.preventDefault();
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
                    <Button onClick={create}>{dict.common.create}</Button>
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
                    {it.is_valid ? dict.common.enabled : dict.common.disabled}
                  </TableCell>
                  <TableCell>
                    {new Date(it.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="secondary"
                      onClick={() => toggleValid(it.id, it.is_valid)}
                    >
                      {it.is_valid ? dict.common.disable : dict.common.enable}
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

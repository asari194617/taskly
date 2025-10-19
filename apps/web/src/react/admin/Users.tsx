"use client";
import * as React from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { gql } from "@/lib/graphql";
import { toast } from "@/hooks/use-toast";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { MoreVertical } from "lucide-react";

type AUser = {
  id: number;
  email: string;
  display_name?: string | null;
  roles: string[];
  is_blocked: boolean;
};

export default function AdminUsers() {
  const token =
    (typeof window !== "undefined" && localStorage.getItem("api_token")) || "";
  const [q, setQ] = React.useState("");
  const [items, setItems] = React.useState<AUser[]>([]);
  const [loading, setLoading] = React.useState(false);

  async function load() {
    if (!token) return;
    setLoading(true);
    try {
      const d = await gql<{ adminUsers: AUser[] }>(
        `query($q:String){ adminUsers(q:$q){ id email display_name roles is_blocked } }`,
        { q: q || undefined }
      );
      setItems(d.adminUsers || []);
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "読み込みに失敗しました",
        description: e?.message,
      });
    } finally {
      setLoading(false);
    }
  }
  React.useEffect(() => {
    load();
  }, [token]);

  async function addRole(id: number, role: string) {
    try {
      await gql<{ adminUserAddRole: boolean }>(
        `mutation($id:Int!,$r:String!){ adminUserAddRole(user_id:$id, role:$r) }`,
        { id, r: role }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "付与に失敗しました",
        description: e?.message,
      });
    }
  }
  async function removeRole(id: number, role: string) {
    try {
      await gql<{ adminUserRemoveRole: boolean }>(
        `mutation($id:Int!,$r:String!){ adminUserRemoveRole(user_id:$id, role:$r) }`,
        { id, r: role }
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
  async function blockUser(id: number) {
    const reason = window.prompt("理由(任意)") || undefined;
    try {
      await gql<{ adminUserBlock: boolean }>(
        `mutation($id:Int!,$reason:String){ adminUserBlock(user_id:$id, reason:$reason) }`,
        { id, reason }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "ブロックに失敗しました",
        description: e?.message,
      });
    }
  }
  async function unblockUser(id: number) {
    try {
      await gql<{ adminUserUnblock: boolean }>(
        `mutation($id:Int!){ adminUserUnblock(user_id:$id) }`,
        { id }
      );
      load();
    } catch (e: any) {
      toast({
        variant: "destructive",
        title: "解除に失敗しました",
        description: e?.message,
      });
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">ユーザー管理</h1>
      </div>
      <Card className="p-4 space-y-3">
        <div className="flex gap-2 items-end">
          <div className="flex-1 min-w-60">
            <div className="text-sm mb-1">検索（メール/表示名）</div>
            <Input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="example@example.com"
            />
          </div>
          <Button onClick={load} disabled={loading}>
            検索
          </Button>
        </div>
      </Card>
      <Card className="p-0 overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>メール</TableHead>
              <TableHead>表示名</TableHead>
              <TableHead>ロール</TableHead>
              <TableHead>ブロック</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className="text-center text-sm text-muted-foreground"
                >
                  読み込み中…
                </TableCell>
              </TableRow>
            ) : items.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={6}
                  className="text-center text-sm text-muted-foreground"
                >
                  該当するユーザーがいません
                </TableCell>
              </TableRow>
            ) : (
              items.map((u) => (
                <TableRow key={u.id}>
                  <TableCell>{u.id}</TableCell>
                  <TableCell>{u.email}</TableCell>
                  <TableCell>{u.display_name || "-"}</TableCell>
                  <TableCell className="max-w-[240px] truncate">
                    {(u.roles || []).join(", ")}
                  </TableCell>
                  <TableCell>{u.is_blocked ? "Yes" : "No"}</TableCell>
                  <TableCell className="text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button size="icon" variant="ghost" aria-label="操作">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        {(() => {
                          const roles = u.roles || [];
                          const hasAdmin = roles.includes("admin");
                          const hasManager = roles.includes("manager");
                          return (
                            <>
                              {/* Admin role toggle */}
                              {hasAdmin ? (
                                <DropdownMenuItem
                                  onClick={() => removeRole(u.id, "admin")}
                                >
                                  Admin削除
                                </DropdownMenuItem>
                              ) : (
                                <DropdownMenuItem
                                  onClick={() => addRole(u.id, "admin")}
                                >
                                  Admin付与
                                </DropdownMenuItem>
                              )}
                              {/* Manager role toggle */}
                              {hasManager ? (
                                <DropdownMenuItem
                                  onClick={() => removeRole(u.id, "manager")}
                                >
                                  Manager削除
                                </DropdownMenuItem>
                              ) : (
                                <DropdownMenuItem
                                  onClick={() => addRole(u.id, "manager")}
                                >
                                  Manager付与
                                </DropdownMenuItem>
                              )}
                              {/* Block/unblock */}
                              {u.is_blocked ? (
                                <DropdownMenuItem
                                  onClick={() => unblockUser(u.id)}
                                >
                                  ブロック解除
                                </DropdownMenuItem>
                              ) : (
                                <DropdownMenuItem
                                  className="text-destructive focus:text-destructive"
                                  onClick={() => blockUser(u.id)}
                                >
                                  ブロック
                                </DropdownMenuItem>
                              )}
                            </>
                          );
                        })()}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
}
